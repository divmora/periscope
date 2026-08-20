package collector

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/divmora/periscope/internal/storage"
	"github.com/google/uuid"
)

// Default hybrid flush trigger thresholds
const (
	DefaultBatchSize      = 10000
	DefaultFlushInterval  = 1 * time.Hour
	DefaultMaxBufferBytes = int64(100 * 1024 * 1024) // 100MB
)

// WatermarkUpdaterFunc is a callback invoked after successful Parquet upload to persist watermark.
type WatermarkUpdaterFunc func(ctx context.Context, projectID int, entryType string, sequence int64) error

// PrunerFunc is a callback invoked after watermark persistence to prune drained records from the source DB.
type PrunerFunc func(ctx context.Context, entryType string, sequence int64) (int64, error)

// FlusherConfig holds configuration settings for the hybrid buffer flusher.
type FlusherConfig struct {
	ProjectID        int
	ProjectIID       string
	BatchSize        int
	FlushInterval    time.Duration
	MaxBufferBytes   int64
	Driver           storage.Driver
	WatermarkUpdater WatermarkUpdaterFunc
	Pruner           PrunerFunc
	PruneEnabled     bool
	DryRun           bool
}

// PartitionBuffer holds buffered records and tracking metadata for a single Hive partition slice.
type PartitionBuffer struct {
	key            PartitionKey
	records        []TelemetryRecord
	estimatedBytes int64
	firstRecordAt  time.Time
	minSequence    int64
	maxSequence    int64
}

// FlushResult contains telemetry statistics for a flushed partition batch.
type FlushResult struct {
	PartitionKey    PartitionKey  `json:"partition_key"`
	StorageKey      string        `json:"storage_key"`
	RecordCount     int           `json:"record_count"`
	MinSequence     int64         `json:"min_sequence"`
	MaxSequence     int64         `json:"max_sequence"`
	BytesCompressed int64         `json:"bytes_compressed"`
	Duration        time.Duration `json:"duration"`
	PrunedCount     int64         `json:"pruned_count"`
}

// Flusher manages thread-safe partition buffers and handles hybrid flush triggers.
type Flusher struct {
	mu      sync.Mutex
	cfg     FlusherConfig
	buffers map[PartitionKey]*PartitionBuffer
}

// NewFlusher creates an initialized Flusher.
func NewFlusher(cfg FlusherConfig) *Flusher {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = DefaultBatchSize
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = DefaultFlushInterval
	}
	if cfg.MaxBufferBytes <= 0 {
		cfg.MaxBufferBytes = DefaultMaxBufferBytes
	}

	return &Flusher{
		cfg:     cfg,
		buffers: make(map[PartitionKey]*PartitionBuffer),
	}
}

// AddRecord adds a record into the appropriate partition buffer.
// If any hybrid trigger is met (row count, time window, buffer size), it flushes that partition buffer.
func (f *Flusher) AddRecord(ctx context.Context, rec TelemetryRecord) (*FlushResult, error) {
	f.mu.Lock()

	key := PartitionKeyFromRecord(f.cfg.ProjectIID, rec)
	buf, exists := f.buffers[key]
	if !exists {
		buf = &PartitionBuffer{
			key:           key,
			records:       make([]TelemetryRecord, 0, f.cfg.BatchSize),
			firstRecordAt: time.Now().UTC(),
			minSequence:   rec.Sequence,
			maxSequence:   rec.Sequence,
		}
		f.buffers[key] = buf
	}

	buf.records = append(buf.records, rec)
	if rec.Sequence < buf.minSequence {
		buf.minSequence = rec.Sequence
	}
	if rec.Sequence > buf.maxSequence {
		buf.maxSequence = rec.Sequence
	}

	// Approximate uncompressed payload size
	recSize := int64(len(rec.Content) + len(rec.UUID) + len(rec.BatchID) + len(rec.Type) + 256)
	buf.estimatedBytes += recSize

	// Check hybrid triggers
	shouldFlush := len(buf.records) >= f.cfg.BatchSize ||
		time.Since(buf.firstRecordAt) >= f.cfg.FlushInterval ||
		buf.estimatedBytes >= f.cfg.MaxBufferBytes

	if !shouldFlush {
		f.mu.Unlock()
		return nil, nil
	}

	// Detach buffer for flushing while holding lock, then release lock during I/O
	delete(f.buffers, key)
	f.mu.Unlock()

	return f.flushBuffer(ctx, buf)
}

// FlushAll flushes all active non-empty partition buffers.
func (f *Flusher) FlushAll(ctx context.Context) ([]FlushResult, error) {
	f.mu.Lock()
	bufsToFlush := make([]*PartitionBuffer, 0, len(f.buffers))
	for k, buf := range f.buffers {
		if len(buf.records) > 0 {
			bufsToFlush = append(bufsToFlush, buf)
		}
		delete(f.buffers, k)
	}
	f.mu.Unlock()

	var results []FlushResult
	for _, buf := range bufsToFlush {
		res, err := f.flushBuffer(ctx, buf)
		if err != nil {
			return results, fmt.Errorf("failed to flush partition buffer %v: %w", buf.key, err)
		}
		if res != nil {
			results = append(results, *res)
		}
	}

	return results, nil
}

// flushBuffer performs Parquet encoding, storage upload, watermark update, and optional pruning.
func (f *Flusher) flushBuffer(ctx context.Context, buf *PartitionBuffer) (*FlushResult, error) {
	if buf == nil || len(buf.records) == 0 {
		return nil, nil
	}

	startTime := time.Now()
	batchUUID := uuid.NewString()
	storageKey := PartitionPath(buf.key, buf.minSequence, buf.maxSequence, batchUUID)

	result := &FlushResult{
		PartitionKey: buf.key,
		StorageKey:   storageKey,
		RecordCount:  len(buf.records),
		MinSequence:  buf.minSequence,
		MaxSequence:  buf.maxSequence,
	}

	// Dry run mode skips upload, watermark update, and pruning
	if f.cfg.DryRun {
		result.Duration = time.Since(startTime)
		return result, nil
	}

	// Step 1: Parquet Encoding with ZSTD
	parquetData, err := EncodeParquet(buf.records)
	if err != nil {
		return nil, fmt.Errorf("failed to encode parquet for key %s: %w", storageKey, err)
	}
	result.BytesCompressed = int64(len(parquetData))

	// Step 2: Storage Upload (Strict verification)
	if f.cfg.Driver == nil {
		return nil, fmt.Errorf("storage driver is not configured on flusher")
	}

	err = f.cfg.Driver.Upload(
		ctx,
		storageKey,
		bytes.NewReader(parquetData),
		int64(len(parquetData)),
		"application/vnd.apache.parquet",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to upload parquet to storage key %s: %w", storageKey, err)
	}

	// Step 3: Checkpoint Watermark in PostgreSQL (Two-Phase Commit Phase 2)
	if f.cfg.WatermarkUpdater != nil {
		err = f.cfg.WatermarkUpdater(ctx, f.cfg.ProjectID, buf.key.Type, buf.maxSequence)
		if err != nil {
			return nil, fmt.Errorf("parquet uploaded to %s but failed to checkpoint watermark: %w", storageKey, err)
		}
	}

	// Step 4: Optional Source Pruning (Two-Phase Commit Phase 3)
	if f.cfg.PruneEnabled && f.cfg.Pruner != nil {
		prunedCount, pruneErr := f.cfg.Pruner(ctx, buf.key.Type, buf.maxSequence)
		if pruneErr != nil {
			// Pruning failure is non-fatal to data integrity since Parquet is safely persisted and watermarked
			// Return result with error note or log
		} else {
			result.PrunedCount = prunedCount
		}
	}

	result.Duration = time.Since(startTime)
	return result, nil
}

// ActiveBufferCount returns the number of active partition buffers currently in memory.
func (f *Flusher) ActiveBufferCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.buffers)
}

// TotalBufferedRecords returns the total count of buffered records across all active partition buffers.
func (f *Flusher) TotalBufferedRecords() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	count := 0
	for _, b := range f.buffers {
		count += len(b.records)
	}
	return count
}
