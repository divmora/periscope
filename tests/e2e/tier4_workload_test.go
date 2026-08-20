package e2e_test

import (
	"context"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/divmora/periscope/internal/storage"
	"github.com/divmora/periscope/tests/e2e/harness"
	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Tier 4 - Test 1: Full Laravel Application Workload Simulation (1,000+ entries)
// ---------------------------------------------------------------------------
func TestTier4_FullLaravelApp_WorkloadSimulation(t *testing.T) {
	ctx := context.Background()
	s3Mock := harness.NewS3MockServer()
	defer s3Mock.Close()

	bucket := "periscope-full-app-simulation"
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

	sourceDB := harness.NewMockTelescopeDatabase("mysql")
	metaDB := harness.NewMockMetadataDatabase()
	projectID := 1
	projectIID := uuid.NewString()

	// Generate 1,200 telemetry entries across all 16+ entry types
	totalCount := 1200
	baseTime := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	entries, tags := harness.GenerateEntriesBatch(1, totalCount, baseTime, 500*time.Millisecond, harness.AllKnownEntryTypes)
	sourceDB.Seed(entries, tags)

	// Run Sync
	res, err := harness.RunSyncCycle(ctx, sourceDB, metaDB, driver, projectID, projectIID, harness.AllKnownEntryTypes, 2000, true)
	if err != nil {
		t.Fatalf("RunSyncCycle failed: %v", err)
	}

	if res.RecordsDrained != totalCount {
		t.Errorf("Expected %d records drained, got %d", totalCount, res.RecordsDrained)
	}
	if res.RowsPruned != int64(totalCount) {
		t.Errorf("Expected %d rows pruned, got %d", totalCount, res.RowsPruned)
	}

	// Verify all Parquet files in S3
	keys := s3Mock.ListKeys(bucket, fmt.Sprintf("projectId=%s", projectIID))
	if len(keys) == 0 {
		t.Fatalf("No Parquet files found in S3 bucket")
	}

	inspector := &harness.ParquetInspector{}
	var totalVerifiedRows int64
	seenSequences := make(map[int64]bool)

	for _, key := range keys {
		data, ok := s3Mock.GetObjectData(bucket, key)
		if !ok {
			t.Fatalf("Failed getting object %s from S3", key)
		}

		rows, err := inspector.ReadTelemetryRows(data)
		if err != nil {
			t.Fatalf("ReadTelemetryRows failed for %s: %v", key, err)
		}

		for _, r := range rows {
			if seenSequences[r.Sequence] {
				t.Errorf("Duplicate sequence %d detected in Parquet file %s", r.Sequence, key)
			}
			seenSequences[r.Sequence] = true
			totalVerifiedRows++
		}
	}

	if totalVerifiedRows != int64(totalCount) {
		t.Errorf("Total verified Parquet rows mismatch: expected %d, got %d", totalCount, totalVerifiedRows)
	}
}

// ---------------------------------------------------------------------------
// Tier 4 - Test 2: Two-Stage Incremental Sync Run
// ---------------------------------------------------------------------------
func TestTier4_TwoStage_IncrementalSync(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	storageDriver, _ := storage.NewLocalDriver(storage.LocalConfig{BasePath: tempDir})

	sourceDB := harness.NewMockTelescopeDatabase("postgres")
	metaDB := harness.NewMockMetadataDatabase()
	projectID := 1
	projectIID := "incremental-proj-1"

	baseTime := time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC)

	// Stage 1: Seed initial 500 entries (seq 1..500)
	entries1, tags1 := harness.GenerateEntriesBatch(1, 500, baseTime, time.Second, []string{"request", "query"})
	sourceDB.Seed(entries1, tags1)

	// Run Sync 1 (prune_enabled = false to keep data)
	res1, err := harness.RunSyncCycle(ctx, sourceDB, metaDB, storageDriver, projectID, projectIID, []string{"request", "query"}, 1000, false)
	if err != nil {
		t.Fatalf("Stage 1 Sync failed: %v", err)
	}
	if res1.RecordsDrained != 500 {
		t.Errorf("Stage 1 expected 500 drained, got %d", res1.RecordsDrained)
	}

	// Verify watermarks after stage 1
	wmReq1, _ := metaDB.GetWatermark(projectID, "request")
	wmQuery1, _ := metaDB.GetWatermark(projectID, "query")
	if wmReq1 == 0 || wmQuery1 == 0 {
		t.Errorf("Stage 1 watermarks not set: req=%d, query=%d", wmReq1, wmQuery1)
	}

	// Stage 2: Add 300 new entries (seq 501..800)
	entries2, tags2 := harness.GenerateEntriesBatch(501, 300, baseTime.Add(time.Hour), time.Second, []string{"request", "query"})
	sourceDB.Seed(entries2, tags2)

	// Run Sync 2 -> must only drain new entries (seq > last_watermark)
	res2, err := harness.RunSyncCycle(ctx, sourceDB, metaDB, storageDriver, projectID, projectIID, []string{"request", "query"}, 1000, false)
	if err != nil {
		t.Fatalf("Stage 2 Sync failed: %v", err)
	}
	if res2.RecordsDrained != 300 {
		t.Errorf("Stage 2 expected exactly 300 drained new records, got %d", res2.RecordsDrained)
	}

	// Verify watermarks advanced
	wmReq2, _ := metaDB.GetWatermark(projectID, "request")
	wmQuery2, _ := metaDB.GetWatermark(projectID, "query")
	if wmReq2 <= wmReq1 || wmQuery2 <= wmQuery1 {
		t.Errorf("Watermarks did not advance in stage 2: req (%d -> %d), query (%d -> %d)", wmReq1, wmReq2, wmQuery1, wmQuery2)
	}

	// Verify total records in storage across all partition files
	objects, err := storageDriver.List(ctx, fmt.Sprintf("projectId=%s", projectIID))
	if err != nil {
		t.Fatalf("Storage list failed: %v", err)
	}

	inspector := &harness.ParquetInspector{}
	var totalRead int64
	seenSeqs := make(map[int64]bool)

	for _, obj := range objects {
		rc, err := storageDriver.Get(ctx, obj.Key)
		if err != nil {
			t.Fatalf("Failed getting %s: %v", obj.Key, err)
		}
		data, _ := io.ReadAll(rc)
		_ = rc.Close()

		rows, err := inspector.ReadTelemetryRows(data)
		if err != nil {
			t.Fatalf("ReadTelemetryRows failed for %s: %v", obj.Key, err)
		}
		for _, r := range rows {
			if seenSeqs[r.Sequence] {
				t.Errorf("Duplicate sequence %d across sync runs!", r.Sequence)
			}
			seenSeqs[r.Sequence] = true
			totalRead++
		}
	}

	if totalRead != 800 {
		t.Errorf("Total Parquet records in storage mismatch: expected 800, got %d", totalRead)
	}
}

// ---------------------------------------------------------------------------
// Tier 4 - Test 3: Pruned Source Incremental Sync Run
// ---------------------------------------------------------------------------
func TestTier4_PrunedSource_IncrementalSync(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	storageDriver, _ := storage.NewLocalDriver(storage.LocalConfig{BasePath: tempDir})

	sourceDB := harness.NewMockTelescopeDatabase("mysql")
	metaDB := harness.NewMockMetadataDatabase()
	projectID := 2
	projectIID := "pruned-incremental-proj"

	baseTime := time.Date(2026, 8, 20, 14, 0, 0, 0, time.UTC)

	// Stage 1: Seed 200 entries (seq 1..200) with prune_enabled = true
	entries1, tags1 := harness.GenerateEntriesBatch(1, 200, baseTime, time.Second, []string{"log", "exception"})
	sourceDB.Seed(entries1, tags1)

	res1, err := harness.RunSyncCycle(ctx, sourceDB, metaDB, storageDriver, projectID, projectIID, []string{"log", "exception"}, 500, true)
	if err != nil {
		t.Fatalf("Stage 1 Sync failed: %v", err)
	}
	if res1.RecordsDrained != 200 {
		t.Errorf("Stage 1 expected 200 drained, got %d", res1.RecordsDrained)
	}
	if sourceDB.CountEntries() != 0 {
		t.Errorf("Expected source DB to be empty after stage 1 prune, got %d", sourceDB.CountEntries())
	}

	// Stage 2: Seed 250 new entries (seq 201..450) into empty table
	entries2, tags2 := harness.GenerateEntriesBatch(201, 250, baseTime.Add(time.Hour), time.Second, []string{"log", "exception"})
	sourceDB.Seed(entries2, tags2)

	res2, err := harness.RunSyncCycle(ctx, sourceDB, metaDB, storageDriver, projectID, projectIID, []string{"log", "exception"}, 500, true)
	if err != nil {
		t.Fatalf("Stage 2 Sync failed: %v", err)
	}
	if res2.RecordsDrained != 250 {
		t.Errorf("Stage 2 expected 250 drained, got %d", res2.RecordsDrained)
	}
	if sourceDB.CountEntries() != 0 {
		t.Errorf("Expected source DB to be empty after stage 2 prune, got %d", sourceDB.CountEntries())
	}

	// Verify all 450 records exist in storage
	objects, _ := storageDriver.List(ctx, fmt.Sprintf("projectId=%s", projectIID))
	inspector := &harness.ParquetInspector{}
	var totalRead int64

	for _, obj := range objects {
		rc, _ := storageDriver.Get(ctx, obj.Key)
		data, _ := io.ReadAll(rc)
		_ = rc.Close()

		rows, err := inspector.ReadTelemetryRows(data)
		if err != nil {
			t.Fatalf("Parquet read failed: %v", err)
		}
		totalRead += int64(len(rows))
	}

	if totalRead != 450 {
		t.Errorf("Total Parquet records in storage mismatch: expected 450, got %d", totalRead)
	}
}

// ---------------------------------------------------------------------------
// Tier 4 - Test 4: Multi-Project Concurrent Sync Execution
// ---------------------------------------------------------------------------
func TestTier4_MultiProject_ConcurrentSync(t *testing.T) {
	ctx := context.Background()
	s3Mock := harness.NewS3MockServer()
	defer s3Mock.Close()

	bucket := "periscope-concurrent-lakehouse"
	s3Mock.CreateBucket(bucket)

	numProjects := 5
	entriesPerProject := 100

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

	var wg sync.WaitGroup
	errChan := make(chan error, numProjects)

	baseTime := time.Date(2026, 8, 20, 16, 0, 0, 0, time.UTC)

	for i := 1; i <= numProjects; i++ {
		wg.Add(1)
		go func(projID int) {
			defer wg.Done()

			projIID := fmt.Sprintf("01912345-000%d-7000-8000-00000000000%d", projID, projID)
			sourceDB := harness.NewMockTelescopeDatabase("postgres")

			entries, tags := harness.GenerateEntriesBatch(
				int64(projID*1000),
				entriesPerProject,
				baseTime,
				time.Second,
				[]string{"request", "query", "job"},
			)
			sourceDB.Seed(entries, tags)

			res, err := harness.RunSyncCycle(ctx, sourceDB, metaDB, driver, projID, projIID, []string{"request", "query", "job"}, 500, true)
			if err != nil {
				errChan <- fmt.Errorf("project %d sync failed: %w", projID, err)
				return
			}

			if res.RecordsDrained != entriesPerProject {
				errChan <- fmt.Errorf("project %d expected %d drained, got %d", projID, entriesPerProject, res.RecordsDrained)
				return
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		t.Errorf("Concurrent sync error: %v", err)
	}

	// Verify each project has isolated Hive partition paths and correct counts
	inspector := &harness.ParquetInspector{}
	for i := 1; i <= numProjects; i++ {
		projIID := fmt.Sprintf("01912345-000%d-7000-8000-00000000000%d", i, i)
		keys := s3Mock.ListKeys(bucket, fmt.Sprintf("projectId=%s", projIID))
		if len(keys) == 0 {
			t.Errorf("No files found for project %s", projIID)
		}

		var projectRows int64
		for _, key := range keys {
			data, ok := s3Mock.GetObjectData(bucket, key)
			if !ok {
				t.Fatalf("Failed reading %s from S3", key)
			}
			rows, err := inspector.ReadTelemetryRows(data)
			if err != nil {
				t.Fatalf("Failed reading rows for %s: %v", key, err)
			}
			projectRows += int64(len(rows))
		}

		if projectRows != int64(entriesPerProject) {
			t.Errorf("Project %s rows mismatch: expected %d, got %d", projIID, entriesPerProject, projectRows)
		}
	}
}
