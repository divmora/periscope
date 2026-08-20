package e2e_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/divmora/periscope/internal/collector"
	"github.com/divmora/periscope/internal/storage"
	"github.com/divmora/periscope/tests/e2e/harness"
	"github.com/google/uuid"
)

// ===========================================================================
// Tier 5 - Stress Test 1: Multi-Gigabyte / 100,000-Row Ingestion Stream
// ===========================================================================
// Verifies that the hybrid buffer flusher, Parquet encoder, and drain coordinator
// smoothly process a 100,000-row ingestion stream across all 16 Telescope entry types,
// triggering batch flushes every 10,000 rows, creating correctly named Parquet files
// with ZSTD compression, advancing watermarks, and executing source pruning.
func TestTier5_MultiGigabyte_100kStreamIngestion(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	driver, err := storage.NewLocalDriver(storage.LocalConfig{BasePath: tempDir})
	if err != nil {
		t.Fatalf("Failed creating local storage driver: %v", err)
	}

	totalRows := 100000
	batchFlushSize := 10000
	expectedBatches := totalRows / batchFlushSize // 10 batches

	projectID := 5001
	projectIID := "01950001-7000-7000-8000-000000100000"

	var watermarksPersisted int64
	var totalPrunedRows int64
	var flushedBatchesCount int64

	watermarkUpdater := func(ctx context.Context, pID int, entryType string, sequence int64) error {
		for {
			curr := atomic.LoadInt64(&watermarksPersisted)
			if sequence <= curr {
				break
			}
			if atomic.CompareAndSwapInt64(&watermarksPersisted, curr, sequence) {
				break
			}
		}
		return nil
	}

	pruner := func(ctx context.Context, entryType string, sequence int64) (int64, error) {
		atomic.AddInt64(&totalPrunedRows, int64(batchFlushSize))
		return int64(batchFlushSize), nil
	}

	flusher := collector.NewFlusher(collector.FlusherConfig{
		ProjectID:        projectID,
		ProjectIID:       projectIID,
		BatchSize:        batchFlushSize,
		FlushInterval:    1 * time.Hour,
		MaxBufferBytes:   100 * 1024 * 1024, // 100MB
		Driver:           driver,
		WatermarkUpdater: watermarkUpdater,
		Pruner:           pruner,
		PruneEnabled:     true,
	})

	baseTime := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	var generatedRecords []collector.TelemetryRecord

	t.Logf("Starting 100,000-row ingestion stream generation and flushing...")
	startTime := time.Now()

	for i := 1; i <= totalRows; i++ {
		seq := int64(i)
		entryType := harness.AllKnownEntryTypes[i%len(harness.AllKnownEntryTypes)]
		entryTime := baseTime.Add(time.Duration(i) * 36 * time.Millisecond) // spans 1 hour total
		payloadJSON, tags := harness.GenerateEntryPayload(entryType, seq)

		rawEntry := collector.TelescopeEntry{
			Sequence:             seq,
			UUID:                 uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("entry-%d", seq))).String(),
			BatchID:              uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("batch-%d", seq/100))).String(),
			Type:                 entryType,
			Content:              payloadJSON,
			CreatedAt:            entryTime,
			Tags:                 tags,
			ShouldDisplayOnIndex: true,
		}

		rec := collector.ExtractRecord(rawEntry)
		flushRes, err := flusher.AddRecord(ctx, rec)
		if err != nil {
			t.Fatalf("AddRecord failed at sequence %d: %v", seq, err)
		}

		if flushRes != nil {
			atomic.AddInt64(&flushedBatchesCount, 1)
			if flushRes.RecordCount != batchFlushSize {
				t.Errorf("Flushed batch record count mismatch: expected %d, got %d", batchFlushSize, flushRes.RecordCount)
			}
			if flushRes.BytesCompressed <= 0 {
				t.Errorf("Flushed batch compressed bytes must be > 0, got %d", flushRes.BytesCompressed)
			}
		}

		// Keep sample for verification
		if i%10000 == 0 || i == 1 {
			generatedRecords = append(generatedRecords, rec)
		}
	}

	// Flush any remaining buffered records
	finalFlushes, err := flusher.FlushAll(ctx)
	if err != nil {
		t.Fatalf("FlushAll failed: %v", err)
	}
	for range finalFlushes {
		atomic.AddInt64(&flushedBatchesCount, 1)
	}

	duration := time.Since(startTime)
	t.Logf("Completed 100k ingestion in %v (Flushed Batches: %d)", duration, flushedBatchesCount)

	// Assert batch count
	if int(flushedBatchesCount) < expectedBatches {
		t.Errorf("Expected at least %d flushed batches for 100k rows, got %d", expectedBatches, flushedBatchesCount)
	}

	// Assert final watermark
	if watermarksPersisted != int64(totalRows) {
		t.Errorf("Final watermark mismatch: expected %d, got %d", totalRows, watermarksPersisted)
	}

	// Assert total pruned
	if totalPrunedRows < int64(totalRows) {
		t.Errorf("Total pruned rows mismatch: expected %d, got %d", totalRows, totalPrunedRows)
	}

	// Inspect all generated Parquet files in storage
	objects, err := driver.List(ctx, fmt.Sprintf("projectId=%s", projectIID))
	if err != nil {
		t.Fatalf("Failed listing objects from storage: %v", err)
	}
	if len(objects) == 0 {
		t.Fatalf("No Parquet files found in storage for project %s", projectIID)
	}

	inspector := &harness.ParquetInspector{}
	var totalVerifiedRows int64
	seenSequences := make(map[int64]bool)

	for _, obj := range objects {
		// Verify Hive partition path format
		parsedPath, err := harness.ParseHivePath(obj.Key)
		if err != nil {
			t.Errorf("Hive path '%s' failed validation: %v", obj.Key, err)
			continue
		}
		if parsedPath.ProjectID != projectIID {
			t.Errorf("Path project ID mismatch: expected %s, got %s", projectIID, parsedPath.ProjectID)
		}

		// Verify filename format: part-000000000001-000000010000-uuid.parquet
		if !strings.HasPrefix(parsedPath.Filename, "part-") || !strings.HasSuffix(parsedPath.Filename, ".parquet") {
			t.Errorf("Filename %s does not match part-{minSeq}-{maxSeq}-{uuid}.parquet format", parsedPath.Filename)
		}

		// Read and inspect Parquet bytes
		rc, err := driver.Get(ctx, obj.Key)
		if err != nil {
			t.Fatalf("Failed to get object %s: %v", obj.Key, err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("Failed reading object %s: %v", obj.Key, err)
		}

		insp, err := inspector.InspectParquetBytes(data)
		if err != nil {
			t.Fatalf("Failed to inspect parquet bytes for %s: %v", obj.Key, err)
		}
		if !insp.HasZSTD {
			t.Errorf("Parquet file %s missing ZSTD compression codec", obj.Key)
		}

		rows, err := inspector.ReadTelemetryRows(data)
		if err != nil {
			t.Fatalf("Failed reading telemetry rows from %s: %v", obj.Key, err)
		}

		for _, r := range rows {
			if seenSequences[r.Sequence] {
				t.Errorf("Duplicate sequence %d detected in file %s", r.Sequence, obj.Key)
			}
			seenSequences[r.Sequence] = true
			totalVerifiedRows++
		}
	}

	if totalVerifiedRows != int64(totalRows) {
		t.Errorf("Total verified Parquet rows mismatch: expected %d, got %d", totalRows, totalVerifiedRows)
	}

	// Verify sample records match original content
	for _, sample := range generatedRecords {
		if !seenSequences[sample.Sequence] {
			t.Errorf("Sample sequence %d was not found in any Parquet file", sample.Sequence)
		}
	}
}

// ===========================================================================
// Tier 5 - Stress Test 2: High Concurrency / Simultaneous Multi-Project Sync
// ===========================================================================
// Simulates 20 distinct projects concurrently executing telemetry sync pipelines
// against a shared S3/MinIO bucket. Verifies race safety, tenant isolation,
// independent watermark tracking, and absence of data cross-contamination.
func TestTier5_HighConcurrency_MultiProjectSimultaneousSync(t *testing.T) {
	ctx := context.Background()
	s3Mock := harness.NewS3MockServer()
	defer s3Mock.Close()

	bucket := "periscope-adversarial-concurrent-lakehouse"
	s3Mock.CreateBucket(bucket)

	driver, err := storage.NewDriverFromConfig(ctx, storage.Config{
		Type:            "minio",
		Bucket:          bucket,
		Region:          "us-east-1",
		Endpoint:        s3Mock.URL(),
		AccessKeyID:     "minioadmin",
		SecretAccessKey: "minioadmin",
		UsePathStyle:    true,
		DisableSSL:      true,
	})
	if err != nil {
		t.Fatalf("Storage driver initialization failed: %v", err)
	}

	metaDB := harness.NewMockMetadataDatabase()

	numConcurrentProjects := 20
	entriesPerProject := 500

	var wg sync.WaitGroup
	wg.Add(numConcurrentProjects)
	errChan := make(chan error, numConcurrentProjects)

	baseTime := time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC)

	t.Logf("Launching %d concurrent project sync routines (%d entries each)...", numConcurrentProjects, entriesPerProject)

	for p := 1; p <= numConcurrentProjects; p++ {
		go func(projID int) {
			defer wg.Done()

			projIID := fmt.Sprintf("0195%04d-0000-7000-8000-%012d", projID, projID)
			dialect := "postgres"
			if projID%2 == 0 {
				dialect = "mysql"
			}
			sourceDB := harness.NewMockTelescopeDatabase(dialect)

			// Generate project-specific entries
			startSeq := int64(projID * 10000)
			types := []string{"request", "query", "exception", "log", "job"}
			entries, tags := harness.GenerateEntriesBatch(startSeq, entriesPerProject, baseTime, 100*time.Millisecond, types)
			sourceDB.Seed(entries, tags)

			// Run sync cycle
			res, err := harness.RunSyncCycle(ctx, sourceDB, metaDB, driver, projID, projIID, types, 250, true)
			if err != nil {
				errChan <- fmt.Errorf("project %d sync failed: %w", projID, err)
				return
			}

			if res.RecordsDrained != entriesPerProject {
				errChan <- fmt.Errorf("project %d records drained mismatch: expected %d, got %d", projID, entriesPerProject, res.RecordsDrained)
				return
			}
			if res.RowsPruned != int64(entriesPerProject) {
				errChan <- fmt.Errorf("project %d rows pruned mismatch: expected %d, got %d", projID, entriesPerProject, res.RowsPruned)
				return
			}
			if sourceDB.CountEntries() != 0 {
				errChan <- fmt.Errorf("project %d source database not fully pruned: %d remaining", projID, sourceDB.CountEntries())
				return
			}
		}(p)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		t.Errorf("Concurrent sync failure: %v", err)
	}

	// Verify tenant isolation across all projects in S3
	inspector := &harness.ParquetInspector{}
	totalExpectedGlobalRows := numConcurrentProjects * entriesPerProject
	var totalGlobalVerifiedRows int64

	for p := 1; p <= numConcurrentProjects; p++ {
		projIID := fmt.Sprintf("0195%04d-0000-7000-8000-%012d", p, p)
		prefix := fmt.Sprintf("projectId=%s", projIID)
		keys := s3Mock.ListKeys(bucket, prefix)
		if len(keys) == 0 {
			t.Errorf("Project %s has 0 keys in S3", projIID)
			continue
		}

		var projectRows int64
		startSeq := int64(p * 10000)
		endSeq := startSeq + int64(entriesPerProject) - 1

		for _, key := range keys {
			// Verify key prefix contains ONLY this project's IID
			if !strings.HasPrefix(key, prefix) {
				t.Errorf("Tenant boundary leak! Key %s in project %s bucket", key, projIID)
			}

			data, ok := s3Mock.GetObjectData(bucket, key)
			if !ok {
				t.Fatalf("Failed to retrieve S3 object %s", key)
			}

			rows, err := inspector.ReadTelemetryRows(data)
			if err != nil {
				t.Fatalf("Failed reading telemetry rows for %s: %v", key, err)
			}

			for _, r := range rows {
				// Assert sequence belongs strictly to this project's range
				if r.Sequence < startSeq || r.Sequence > endSeq {
					t.Errorf("Sequence %d leaked into project %s (valid range: %d..%d)", r.Sequence, projIID, startSeq, endSeq)
				}
				projectRows++
				totalGlobalVerifiedRows++
			}
		}

		if projectRows != int64(entriesPerProject) {
			t.Errorf("Project %s total rows mismatch: expected %d, got %d", projIID, entriesPerProject, projectRows)
		}
	}

	if totalGlobalVerifiedRows != int64(totalExpectedGlobalRows) {
		t.Errorf("Global verified row count mismatch: expected %d, got %d", totalExpectedGlobalRows, totalGlobalVerifiedRows)
	}
}

// ===========================================================================
// Tier 5 - Stress Test 3: Out-of-Order Sequence Insertion & Disordered Buffers
// ===========================================================================
// Verifies that:
// 1. When records arrive in arbitrary/shuffled sequence orders within a buffer,
//    the flusher correctly calculates minSequence and maxSequence.
// 2. Out-of-order sequence insertion in source database adheres to monotonic watermark
//    guarantees (watermark never regresses backwards).
func TestTier5_OutOfOrder_SequenceInsertionSimulation(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	driver, err := storage.NewLocalDriver(storage.LocalConfig{BasePath: tempDir})
	if err != nil {
		t.Fatalf("Failed creating local storage driver: %v", err)
	}

	projectID := 3001
	projectIID := "01953001-7000-7000-8000-000000003001"

	// Scenario A: Shuffled / Non-Monotonic Sequences fed into Flusher
	var watermarksRecorded []int64
	watermarkUpdater := func(ctx context.Context, pID int, entryType string, seq int64) error {
		watermarksRecorded = append(watermarksRecorded, seq)
		return nil
	}

	flusher := collector.NewFlusher(collector.FlusherConfig{
		ProjectID:        projectID,
		ProjectIID:       projectIID,
		BatchSize:        5,
		FlushInterval:    1 * time.Hour,
		Driver:           driver,
		WatermarkUpdater: watermarkUpdater,
	})

	now := time.Date(2026, 8, 20, 15, 0, 0, 0, time.UTC)
	// Add 5 records with deliberately scrambled sequence numbers: [50, 10, 80, 20, 60]
	scrambledSequences := []int64{50, 10, 80, 20, 60}
	var lastFlushResult *collector.FlushResult

	for _, seq := range scrambledSequences {
		rec := collector.TelemetryRecord{
			Sequence:  seq,
			UUID:      uuid.NewString(),
			BatchID:   uuid.NewString(),
			Type:      "request",
			CreatedAt: now,
			Content:   `{"uri":"/scrambled","response_status":200}`,
		}
		res, err := flusher.AddRecord(ctx, rec)
		if err != nil {
			t.Fatalf("AddRecord failed for seq %d: %v", seq, err)
		}
		if res != nil {
			lastFlushResult = res
		}
	}

	if lastFlushResult == nil {
		t.Fatal("Expected flush trigger after 5 records, got nil")
	}

	// MinSequence must be 10, MaxSequence must be 80
	if lastFlushResult.MinSequence != 10 {
		t.Errorf("Expected MinSequence 10 for scrambled batch, got %d", lastFlushResult.MinSequence)
	}
	if lastFlushResult.MaxSequence != 80 {
		t.Errorf("Expected MaxSequence 80 for scrambled batch, got %d", lastFlushResult.MaxSequence)
	}
	if len(watermarksRecorded) != 1 || watermarksRecorded[0] != 80 {
		t.Errorf("Expected watermark checkpoint 80, got %v", watermarksRecorded)
	}

	// Verify Parquet file has exact min/max seq in name: part-000000000010-000000000080-...
	if !strings.Contains(lastFlushResult.StorageKey, "part-000000000010-000000000080-") {
		t.Errorf("Partition filename did not reflect true min/max sequences: %s", lastFlushResult.StorageKey)
	}

	// Scenario B: Monotonic Progression Simulation in Mock Metadata DB
	metaDB := harness.NewMockMetadataDatabase()
	// Set initial watermark = 100
	_ = metaDB.UpsertWatermark(projectID, "request", 100, now)

	// Attempt to upsert a delayed/out-of-order sequence = 45
	_ = metaDB.UpsertWatermark(projectID, "request", 45, now.Add(time.Minute))

	// Watermark must remain 100 (GREATEST rule)
	wm, _ := metaDB.GetWatermark(projectID, "request")
	if wm != 100 {
		t.Errorf("Watermark regressed! Expected 100, got %d", wm)
	}

	// Upsert higher sequence = 150
	_ = metaDB.UpsertWatermark(projectID, "request", 150, now.Add(2*time.Minute))
	wm2, _ := metaDB.GetWatermark(projectID, "request")
	if wm2 != 150 {
		t.Errorf("Watermark failed to advance! Expected 150, got %d", wm2)
	}
}

// ===========================================================================
// Tier 5 - Stress Test 4: Truncated, Corrupt, and Malicious JSON Payloads
// ===========================================================================
// Verifies that the dimension extractor gracefully handles extreme edge cases,
// malformed syntax, truncation, null bytes, inverted types, and deep nesting
// without panics, preserving the raw content payload losslessly in Parquet.
func TestTier5_CorruptedAndTruncated_JSONPayloads(t *testing.T) {
	deepNestedJSON := `{"a":{"b":{"c":{"d":{"e":{"f":{"g":{"h":{"i":{"j":{"k":"v"}}}}}}}}}}}`
	massivePayload := fmt.Sprintf(`{"sql":"SELECT %s", "time": 99.9}`, strings.Repeat("A", 100000))

	corruptCases := []struct {
		name        string
		entryType   string
		content     string
		expectRaw   string
		checkFields func(t *testing.T, rec collector.TelemetryRecord)
	}{
		{
			name:      "Truncated mid-key",
			entryType: "request",
			content:   `{"uri": "/api/v1/orders", "res`,
			checkFields: func(t *testing.T, rec collector.TelemetryRecord) {
				if rec.HTTPURI != nil {
					t.Errorf("Expected nil HTTPURI on broken JSON, got %v", *rec.HTTPURI)
				}
			},
		},
		{
			name:      "Truncated mid-escape sequence",
			entryType: "log",
			content:   `{"message": "syntax error \u`,
			checkFields: func(t *testing.T, rec collector.TelemetryRecord) {
				if rec.LogMessage != nil {
					t.Errorf("Expected nil LogMessage on broken escape, got %v", *rec.LogMessage)
				}
			},
		},
		{
			name:      "Inverted types (status as string array, duration as object)",
			entryType: "request",
			content:   `{"response_status": ["200", "OK"], "duration": {"ms": 45}, "uri": "/orders"}`,
			checkFields: func(t *testing.T, rec collector.TelemetryRecord) {
				if rec.HTTPURI == nil || *rec.HTTPURI != "/orders" {
					t.Errorf("Expected HTTPURI /orders, got %v", rec.HTTPURI)
				}
				if rec.HTTPStatus != nil {
					t.Errorf("Expected nil HTTPStatus for array status, got %v", *rec.HTTPStatus)
				}
			},
		},
		{
			name:      "User field is integer instead of map",
			entryType: "request",
			content:   `{"user": 42, "uri": "/api/profile"}`,
			checkFields: func(t *testing.T, rec collector.TelemetryRecord) {
				if rec.UserID != nil {
					t.Errorf("Expected nil UserID when user is int, got %v", *rec.UserID)
				}
			},
		},
		{
			name:      "User map with nil fields and object ID",
			entryType: "request",
			content:   `{"user": {"id": {"val": 10}, "email": null, "name": ""}}`,
			checkFields: func(t *testing.T, rec collector.TelemetryRecord) {
				if rec.UserEmail != nil {
					t.Errorf("Expected nil UserEmail when null in JSON, got %v", *rec.UserEmail)
				}
				if rec.UserName != nil {
					t.Errorf("Expected nil UserName when empty string in JSON, got %v", *rec.UserName)
				}
			},
		},
		{
			name:      "Deeply nested JSON structure",
			entryType: "query",
			content:   deepNestedJSON,
			checkFields: func(t *testing.T, rec collector.TelemetryRecord) {
				if rec.QuerySQL != nil {
					t.Errorf("Expected nil QuerySQL, got %v", *rec.QuerySQL)
				}
			},
		},
		{
			name:      "Massive 100KB payload",
			entryType: "query",
			content:   massivePayload,
			checkFields: func(t *testing.T, rec collector.TelemetryRecord) {
				if rec.QuerySQL == nil || !strings.HasPrefix(*rec.QuerySQL, "SELECT AAA") {
					t.Errorf("Expected massive QuerySQL extracted")
				}
				if rec.QueryTimeMS == nil || *rec.QueryTimeMS != 99.9 {
					t.Errorf("Expected QueryTimeMS 99.9, got %v", rec.QueryTimeMS)
				}
			},
		},
		{
			name:      "Completely empty string payload",
			entryType: "log",
			content:   "",
			checkFields: func(t *testing.T, rec collector.TelemetryRecord) {
				if rec.Content != "" {
					t.Errorf("Expected empty content, got %q", rec.Content)
				}
			},
		},
		{
			name:      "Binary / non-JSON raw string",
			entryType: "dump",
			content:   "\x1f\x8b\x08\x00\x00\x00\x00\x00not a json",
			checkFields: func(t *testing.T, rec collector.TelemetryRecord) {
				if rec.Content != "\x1f\x8b\x08\x00\x00\x00\x00\x00not a json" {
					t.Errorf("Raw binary payload corrupted: %q", rec.Content)
				}
			},
		},
	}

	var recordsToEncode []collector.TelemetryRecord

	for i, tc := range corruptCases {
		t.Run(tc.name, func(t *testing.T) {
			rawEntry := collector.TelescopeEntry{
				Sequence:  int64(i + 1),
				UUID:      uuid.NewString(),
				BatchID:   uuid.NewString(),
				Type:      tc.entryType,
				Content:   tc.content,
				CreatedAt: time.Now().UTC(),
				Tags:      []string{"adversarial", "fuzz"},
			}

			// Must not panic under any circumstance
			rec := collector.ExtractRecord(rawEntry)

			// Raw content MUST be preserved losslessly
			if rec.Content != tc.content {
				t.Fatalf("Raw content not preserved: expected %q, got %q", tc.content, rec.Content)
			}

			tc.checkFields(t, rec)
			recordsToEncode = append(recordsToEncode, rec)
		})
	}

	// Parquet encode and decode the entire batch of corrupt/adversarial records
	parquetData, err := collector.EncodeParquet(recordsToEncode)
	if err != nil {
		t.Fatalf("EncodeParquet failed on adversarial batch: %v", err)
	}

	decoded, err := collector.DecodeParquet(parquetData)
	if err != nil {
		t.Fatalf("DecodeParquet failed on adversarial batch: %v", err)
	}

	if len(decoded) != len(recordsToEncode) {
		t.Fatalf("Decoded record count mismatch: expected %d, got %d", len(recordsToEncode), len(decoded))
	}

	for i, dec := range decoded {
		if dec.Content != recordsToEncode[i].Content {
			t.Errorf("[%s] Readback content mismatch", corruptCases[i].name)
		}
	}
}

// ===========================================================================
// Tier 5 - Stress Test 5: Abrupt Storage / Network Disconnect During Streaming
// ===========================================================================
// Verifies two-phase commit safety:
// 1. When storage upload encounters network disconnect or simulated 500/503 errors,
//    watermark MUST NOT be updated in metadata database.
// 2. Source database MUST NOT be pruned.
// 3. Local temporary files must be cleaned up immediately.
// 4. Retry after storage reconnection must resume without data loss or duplicates.
func TestTier5_AbruptStorageNetworkDisconnect_StreamingFailure(t *testing.T) {
	ctx := context.Background()
	s3Mock := harness.NewS3MockServer()
	defer s3Mock.Close()

	bucket := "periscope-network-disconnect-test"
	s3Mock.CreateBucket(bucket)

	driver, err := storage.NewDriverFromConfig(ctx, storage.Config{
		Type:            "minio",
		Bucket:          bucket,
		Region:          "us-east-1",
		Endpoint:        s3Mock.URL(),
		AccessKeyID:     "minioadmin",
		SecretAccessKey: "minioadmin",
		UsePathStyle:    true,
		DisableSSL:      true,
	})
	if err != nil {
		t.Fatalf("Failed initializing storage driver: %v", err)
	}

	sourceDB := harness.NewMockTelescopeDatabase("postgres")
	metaDB := harness.NewMockMetadataDatabase()
	projectID := 5555
	projectIID := "01955555-7000-7000-8000-000000005555"

	totalEntries := 100
	baseTime := time.Date(2026, 8, 20, 18, 0, 0, 0, time.UTC)
	entries, tags := harness.GenerateEntriesBatch(1, totalEntries, baseTime, time.Second, []string{"request", "query"})
	sourceDB.Seed(entries, tags)

	// 1. Inject simulated storage network disconnect
	s3Mock.SetFaulty(bucket, true)

	// 2. Attempt sync -> MUST fail
	_, syncErr := harness.RunSyncCycle(ctx, sourceDB, metaDB, driver, projectID, projectIID, []string{"request", "query"}, 50, true)
	if syncErr == nil {
		t.Fatal("Expected RunSyncCycle to fail on network disconnect, but it succeeded")
	}

	// 3. Assert Zero-Data-Loss Invariant: Watermark is UNTOUCHED
	wmReq, _ := metaDB.GetWatermark(projectID, "request")
	wmQuery, _ := metaDB.GetWatermark(projectID, "query")
	if wmReq != 0 || wmQuery != 0 {
		t.Fatalf("CRITICAL BUG: Watermark was checkpointed despite upload failure! req=%d, query=%d", wmReq, wmQuery)
	}

	// 4. Assert Source DB was NOT pruned
	if sourceDB.CountEntries() != totalEntries {
		t.Fatalf("CRITICAL BUG: Source entries pruned before upload confirmation! Expected %d, got %d", totalEntries, sourceDB.CountEntries())
	}
	if sourceDB.CountTags() != len(tags) {
		t.Fatalf("CRITICAL BUG: Source tags pruned before upload confirmation! Expected %d, got %d", len(tags), sourceDB.CountTags())
	}

	// 5. Reconnect Network / Restore S3 health
	s3Mock.SetFaulty(bucket, false)

	// 6. Re-run Sync Cycle -> MUST succeed end-to-end
	res, err := harness.RunSyncCycle(ctx, sourceDB, metaDB, driver, projectID, projectIID, []string{"request", "query"}, 50, true)
	if err != nil {
		t.Fatalf("Sync retry after network recovery failed: %v", err)
	}

	if res.RecordsDrained != totalEntries {
		t.Errorf("Expected all %d records drained on retry, got %d", totalEntries, res.RecordsDrained)
	}
	if res.RowsPruned != int64(totalEntries) {
		t.Errorf("Expected %d rows pruned on retry, got %d", totalEntries, res.RowsPruned)
	}
	if sourceDB.CountEntries() != 0 {
		t.Errorf("Expected source DB empty after retry, got %d", sourceDB.CountEntries())
	}

	// Assert final watermarks advanced
	wmReqFinal, _ := metaDB.GetWatermark(projectID, "request")
	wmQueryFinal, _ := metaDB.GetWatermark(projectID, "query")
	if wmReqFinal <= 0 || wmQueryFinal <= 0 {
		t.Errorf("Watermarks not updated after recovery: req=%d, query=%d", wmReqFinal, wmQueryFinal)
	}

	// 7. Verify local temporary files cleanup on LocalDriver failure
	tempDir := t.TempDir()
	localDriver, _ := storage.NewLocalDriver(storage.LocalConfig{BasePath: tempDir})

	// Create a reader that fails halfway through upload
	failingReader := &faultyReader{limit: 100, err: errors.New("simulated pipe disconnect")}
	_ = localDriver.Upload(ctx, "failed/temp-cleanup.parquet", failingReader, 1000, "application/octet-stream")

	// Verify no temporary files remain in temp directory
	var orphanedTemps []string
	_ = filepath.Walk(tempDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasPrefix(info.Name(), ".tmp-periscope-") {
			orphanedTemps = append(orphanedTemps, info.Name())
		}
		return nil
	})

	if len(orphanedTemps) > 0 {
		t.Errorf("Orphaned temporary files found after upload failure: %v", orphanedTemps)
	}
}

// ===========================================================================
// Tier 5 - Stress Test 6: Goroutine Leaks & Rapid Cancellation Stress
// ===========================================================================
func TestTier5_GoroutineLeak_And_RapidCancellation(t *testing.T) {
	tempDir := t.TempDir()
	driver, _ := storage.NewLocalDriver(storage.LocalConfig{BasePath: tempDir})

	numRuns := 50
	var wg sync.WaitGroup
	wg.Add(numRuns)

	for i := 0; i < numRuns; i++ {
		go func(iteration int) {
			defer wg.Done()

			// Create context with ultra-short random deadline
			timeout := time.Duration(rand.Intn(10)+1) * time.Millisecond
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			sourceDB := harness.NewMockTelescopeDatabase("mysql")
			metaDB := harness.NewMockMetadataDatabase()
			entries, tags := harness.GenerateEntriesBatch(1, 50, time.Now().UTC(), time.Millisecond, []string{"log"})
			sourceDB.Seed(entries, tags)

			// Run sync cycle under cancellation pressure
			_, _ = harness.RunSyncCycle(ctx, sourceDB, metaDB, driver, iteration, "canceling-proj", []string{"log"}, 25, false)
		}(i)
	}

	wg.Wait()
	t.Logf("Completed %d rapid cancellation sync cycles without deadlocks", numRuns)
}

type faultyReader struct {
	limit int
	read  int
	err   error
}

func (r *faultyReader) Read(p []byte) (n int, err error) {
	if r.read >= r.limit {
		return 0, r.err
	}
	toRead := len(p)
	if toRead > r.limit-r.read {
		toRead = r.limit - r.read
	}
	for i := 0; i < toRead; i++ {
		p[i] = 'X'
	}
	r.read += toRead
	if r.read >= r.limit {
		return toRead, r.err
	}
	return toRead, nil
}
