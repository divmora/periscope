package collector

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/divmora/periscope/internal/storage"
	"github.com/divmora/periscope/internal/store"
)

// CollectorConfig configures a sync/drain run for a project.
type CollectorConfig struct {
	ProjectID        int
	ProjectIID       string
	SourceDSN        string
	SourceDB         *sql.DB        // Optional override (e.g. for testing)
	Dialect          Dialect        // Optional override
	StorageConfig    storage.Config // Used if StorageDriver is nil
	StorageDriver    storage.Driver // Optional override (e.g. for testing)
	EntryTypes       []string       // Filter entry types to sync (empty means all)
	BatchSize        int            // Max rows per poll and buffer flush trigger (default 10,000)
	FlushInterval    time.Duration  // Max duration before flushing active buffers (default 1 hour)
	MaxBufferBytes   int64          // Max uncompressed bytes before flush (default 100MB)
	PruneEnabled     bool
	DryRun           bool
	WatermarkGetter  func(ctx context.Context, projectID int, entryType string) (int64, error)
	WatermarkUpdater WatermarkUpdaterFunc
}

// SyncResult provides execution telemetry and stats for a completed sync run.
type SyncResult struct {
	ProjectIID           string           `json:"project_iid"`
	EntriesPolled        int              `json:"entries_polled"`
	TotalRecordsWritten  int              `json:"total_records_written"`
	TotalBytesCompressed int64            `json:"total_bytes_compressed"`
	TotalPruned          int64            `json:"total_pruned"`
	FlushedBatches       []FlushResult    `json:"flushed_batches"`
	Watermarks           map[string]int64 `json:"watermarks"`
	Duration             time.Duration    `json:"duration"`
}

// Collector coordinates source database draining, tag aggregation, dimension extraction,
// Parquet encoding, storage persistence, watermark checkpointing, and source pruning.
type Collector struct {
	cfg        CollectorConfig
	db         *sql.DB
	dialect    Dialect
	ownsDB     bool
	driver     storage.Driver
	flusher    *Flusher
	watermarks map[string]int64
}

// NewCollector instantiates and initializes a Collector.
func NewCollector(ctx context.Context, cfg CollectorConfig) (*Collector, error) {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = DefaultBatchSize
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = DefaultFlushInterval
	}
	if cfg.MaxBufferBytes <= 0 {
		cfg.MaxBufferBytes = DefaultMaxBufferBytes
	}
	if cfg.WatermarkGetter == nil {
		cfg.WatermarkGetter = store.GetWatermark
	}
	if cfg.WatermarkUpdater == nil {
		cfg.WatermarkUpdater = store.UpdateWatermark
	}

	var db *sql.DB
	var dialect Dialect
	var ownsDB bool

	if cfg.SourceDB != nil {
		db = cfg.SourceDB
		dialect = cfg.Dialect
		if dialect == "" {
			dialect = DialectMySQL
		}
	} else if cfg.SourceDSN != "" {
		var err error
		db, dialect, err = OpenSourceDB(cfg.SourceDSN)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to source database: %w", err)
		}
		ownsDB = true
	} else {
		return nil, fmt.Errorf("either SourceDB or SourceDSN must be provided")
	}

	var driver storage.Driver
	if cfg.StorageDriver != nil {
		driver = cfg.StorageDriver
	} else {
		var err error
		driver, err = storage.NewDriverFromConfig(ctx, cfg.StorageConfig)
		if err != nil {
			if ownsDB {
				_ = db.Close()
			}
			return nil, fmt.Errorf("failed to initialize storage driver: %w", err)
		}
	}

	c := &Collector{
		cfg:        cfg,
		db:         db,
		dialect:    dialect,
		ownsDB:     ownsDB,
		driver:     driver,
		watermarks: make(map[string]int64),
	}

	// Wire pruner function
	prunerFunc := func(ctx context.Context, entryType string, sequence int64) (int64, error) {
		return PruneSource(ctx, c.db, c.dialect, entryType, sequence)
	}

	// Flusher watermark wrapper that also updates local state
	watermarkFunc := func(ctx context.Context, projectID int, entryType string, sequence int64) error {
		if err := cfg.WatermarkUpdater(ctx, projectID, entryType, sequence); err != nil {
			return err
		}
		c.watermarks[entryType] = sequence
		return nil
	}

	c.flusher = NewFlusher(FlusherConfig{
		ProjectID:        cfg.ProjectID,
		ProjectIID:       cfg.ProjectIID,
		BatchSize:        cfg.BatchSize,
		FlushInterval:    cfg.FlushInterval,
		MaxBufferBytes:   cfg.MaxBufferBytes,
		Driver:           driver,
		WatermarkUpdater: watermarkFunc,
		Pruner:           prunerFunc,
		PruneEnabled:     cfg.PruneEnabled,
		DryRun:           cfg.DryRun,
	})

	return c, nil
}

// Close releases any allocated database connections.
func (c *Collector) Close() error {
	if c.ownsDB && c.db != nil {
		return c.db.Close()
	}
	return nil
}

// Sync performs a full incremental drain and sync pass.
func (c *Collector) Sync(ctx context.Context) (*SyncResult, error) {
	startTime := time.Now()
	result := &SyncResult{
		ProjectIID:     c.cfg.ProjectIID,
		FlushedBatches: make([]FlushResult, 0),
		Watermarks:     make(map[string]int64),
	}

	// 1. Determine starting watermark sequence
	var startSequence int64
	if len(c.cfg.EntryTypes) == 1 {
		// Specific single entry type
		wm, err := c.cfg.WatermarkGetter(ctx, c.cfg.ProjectID, c.cfg.EntryTypes[0])
		if err != nil {
			return nil, fmt.Errorf("failed to fetch watermark: %w", err)
		}
		startSequence = wm
		c.watermarks[c.cfg.EntryTypes[0]] = wm
	} else if len(c.cfg.EntryTypes) > 1 {
		// Multiple types: find minimum watermark across specified types to avoid skipping
		minSeq := int64(-1)
		for _, t := range c.cfg.EntryTypes {
			wm, err := c.cfg.WatermarkGetter(ctx, c.cfg.ProjectID, t)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch watermark for type %s: %w", t, err)
			}
			c.watermarks[t] = wm
			if minSeq == -1 || wm < minSeq {
				minSeq = wm
			}
		}
		if minSeq > 0 {
			startSequence = minSeq
		}
	} else {
		// All types: fetch watermarks if available
		// Default start from 0 if no watermarks
		startSequence = 0
		if allWatermarks, err := store.GetAllWatermarks(ctx, c.cfg.ProjectID); err == nil && len(allWatermarks) > 0 {
			minSeq := int64(-1)
			for t, wm := range allWatermarks {
				c.watermarks[t] = wm
				if minSeq == -1 || wm < minSeq {
					minSeq = wm
				}
			}
			if minSeq > 0 {
				startSequence = minSeq
			}
		}
	}

	currentSeq := startSequence

	// 2. Continuous polling loop until no new records
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		entries, err := PollEntries(ctx, c.db, c.dialect, currentSeq, c.cfg.BatchSize, c.cfg.EntryTypes)
		if err != nil {
			return nil, fmt.Errorf("poll entries failed at sequence > %d: %w", currentSeq, err)
		}

		if len(entries) == 0 {
			break
		}

		result.EntriesPolled += len(entries)

		// Fetch tags
		if err := FetchTagsForEntries(ctx, c.db, c.dialect, entries); err != nil {
			return nil, fmt.Errorf("fetch tags failed: %w", err)
		}

		// Extract typed records and push into flusher
		for _, entry := range entries {
			// Skip if entry's sequence is <= its specific type's watermark (when querying across multiple types)
			if knownWatermark, ok := c.watermarks[entry.Type]; ok && entry.Sequence <= knownWatermark {
				if entry.Sequence > currentSeq {
					currentSeq = entry.Sequence
				}
				continue
			}

			rec := ExtractRecord(entry)
			flushRes, err := c.flusher.AddRecord(ctx, rec)
			if err != nil {
				return nil, fmt.Errorf("flusher add record error: %w", err)
			}
			if flushRes != nil {
				result.FlushedBatches = append(result.FlushedBatches, *flushRes)
				result.TotalRecordsWritten += flushRes.RecordCount
				result.TotalBytesCompressed += flushRes.BytesCompressed
				result.TotalPruned += flushRes.PrunedCount
			}

			if entry.Sequence > currentSeq {
				currentSeq = entry.Sequence
			}
		}

		// If we fetched fewer rows than the requested batch size, we reached current end of stream
		if len(entries) < c.cfg.BatchSize {
			break
		}
	}

	// 3. Flush all remaining buffered records
	finalFlushes, err := c.flusher.FlushAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("final buffer flush failed: %w", err)
	}

	for _, f := range finalFlushes {
		result.FlushedBatches = append(result.FlushedBatches, f)
		result.TotalRecordsWritten += f.RecordCount
		result.TotalBytesCompressed += f.BytesCompressed
		result.TotalPruned += f.PrunedCount
	}

	for k, v := range c.watermarks {
		result.Watermarks[k] = v
	}

	result.Duration = time.Since(startTime)
	return result, nil
}
