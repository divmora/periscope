package e2e_test

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/divmora/periscope/internal/storage"
	"github.com/divmora/periscope/tests/e2e/harness"
)

// ---------------------------------------------------------------------------
// Tier 3 - Test 1: Multi-Dialect Drain with S3 Upload and Source Pruning
// ---------------------------------------------------------------------------
func TestTier3_MultiDialect_Drain_With_S3Upload_And_Pruning(t *testing.T) {
	ctx := context.Background()
	s3Mock := harness.NewS3MockServer()
	defer s3Mock.Close()

	bucketName := "periscope-cross-tier3"
	s3Mock.CreateBucket(bucketName)

	driver, err := storage.NewDriverFromConfig(ctx, storage.Config{
		Type:            "minio",
		Bucket:          bucketName,
		Region:          "us-east-1",
		Endpoint:        s3Mock.URL(),
		AccessKeyID:     "minioadmin",
		SecretAccessKey: "minioadmin",
		UsePathStyle:    true,
		DisableSSL:      true,
	})
	if err != nil {
		t.Fatalf("Failed to initialize S3 driver: %v", err)
	}

	dialects := []struct {
		name       string
		projectID  int
		projectIID string
	}{
		{name: "mysql", projectID: 1, projectIID: "01912345-0001-7000-8000-mysql0000001"},
		{name: "postgres", projectID: 2, projectIID: "01912345-0002-7000-8000-postgres0002"},
	}

	metaDB := harness.NewMockMetadataDatabase()
	inspector := &harness.ParquetInspector{}

	for _, d := range dialects {
		t.Run(d.name, func(t *testing.T) {
			sourceDB := harness.NewMockTelescopeDatabase(d.name)
			baseTime := time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC)
			entryTypes := []string{"request", "query", "exception"}
			entries, tags := harness.GenerateEntriesBatch(1, 150, baseTime, 10*time.Second, entryTypes)
			sourceDB.Seed(entries, tags)

			// Run sync with prune_enabled = true
			res, err := harness.RunSyncCycle(ctx, sourceDB, metaDB, driver, d.projectID, d.projectIID, entryTypes, 500, true)
			if err != nil {
				t.Fatalf("[%s] RunSyncCycle failed: %v", d.name, err)
			}

			if res.RecordsDrained != 150 {
				t.Errorf("[%s] Expected 150 records drained, got %d", d.name, res.RecordsDrained)
			}
			if res.RowsPruned != 150 {
				t.Errorf("[%s] Expected 150 rows pruned, got %d", d.name, res.RowsPruned)
			}
			if sourceDB.CountEntries() != 0 {
				t.Errorf("[%s] Expected 0 remaining entries in source DB", d.name)
			}

			// Verify watermarks for each entry type in metadata DB
			for _, et := range entryTypes {
				wm, err := metaDB.GetWatermark(d.projectID, et)
				if err != nil || wm == 0 {
					t.Errorf("[%s] Watermark for %s not updated (got %d)", d.name, et, wm)
				}
			}

			// Inspect uploaded Parquet files in S3
			prefix := fmt.Sprintf("projectId=%s", d.projectIID)
			keys := s3Mock.ListKeys(bucketName, prefix)
			if len(keys) == 0 {
				t.Fatalf("[%s] No files found under prefix %s", d.name, prefix)
			}

			var verifiedRows int64
			for _, key := range keys {
				data, ok := s3Mock.GetObjectData(bucketName, key)
				if !ok {
					t.Fatalf("[%s] Failed getting S3 object data for %s", d.name, key)
				}
				insp, err := inspector.InspectParquetBytes(data)
				if err != nil {
					t.Fatalf("[%s] InspectParquetBytes failed for %s: %v", d.name, key, err)
				}
				if !insp.HasZSTD {
					t.Errorf("[%s] S3 file %s does not have ZSTD compression", d.name, key)
				}
				verifiedRows += insp.NumRows
			}

			if verifiedRows != 150 {
				t.Errorf("[%s] Total verified Parquet rows mismatch: expected 150, got %d", d.name, verifiedRows)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tier 3 - Test 2: Watermark Rollback on Storage Upload Failure
// ---------------------------------------------------------------------------
func TestTier3_WatermarkRollback_On_StorageUploadFailure(t *testing.T) {
	ctx := context.Background()
	s3Mock := harness.NewS3MockServer()
	defer s3Mock.Close()

	bucketName := "periscope-fault-test"
	s3Mock.CreateBucket(bucketName)

	driver, err := storage.NewDriverFromConfig(ctx, storage.Config{
		Type:            "minio",
		Bucket:          bucketName,
		Region:          "us-east-1",
		Endpoint:        s3Mock.URL(),
		AccessKeyID:     "minioadmin",
		SecretAccessKey: "minioadmin",
		UsePathStyle:    true,
		DisableSSL:      true,
	})
	if err != nil {
		t.Fatalf("Failed to initialize S3 driver: %v", err)
	}

	sourceDB := harness.NewMockTelescopeDatabase("postgres")
	metaDB := harness.NewMockMetadataDatabase()
	baseTime := time.Date(2026, 8, 20, 11, 0, 0, 0, time.UTC)
	entries, tags := harness.GenerateEntriesBatch(1, 50, baseTime, time.Second, []string{"request"})
	sourceDB.Seed(entries, tags)

	projectID := 10
	projectIID := "fault-test-proj"

	// 1. Inject storage failure (simulate network drop or 500 error)
	s3Mock.SetFaulty(bucketName, true)

	// 2. Attempt sync -> must fail
	_, err = harness.RunSyncCycle(ctx, sourceDB, metaDB, driver, projectID, projectIID, []string{"request"}, 100, true)
	if err == nil {
		t.Fatalf("Expected RunSyncCycle to fail when S3 is faulty, but it succeeded")
	}

	// 3. Assert Watermark is UNTOUCHED (0)
	wm, err := metaDB.GetWatermark(projectID, "request")
	if err != nil {
		t.Fatalf("GetWatermark failed: %v", err)
	}
	if wm != 0 {
		t.Errorf("Watermark was improperly updated to %d on upload failure! Zero data loss rule violated.", wm)
	}

	// 4. Assert Source DB is UNTOUCHED (all 50 entries and tags still present)
	if sourceDB.CountEntries() != 50 {
		t.Errorf("Source entries were prematurely deleted on upload failure! Expected 50, got %d", sourceDB.CountEntries())
	}
	if sourceDB.CountTags() != len(tags) {
		t.Errorf("Source tags were prematurely deleted on upload failure! Expected %d, got %d", len(tags), sourceDB.CountTags())
	}

	// 5. Restore S3 health
	s3Mock.SetFaulty(bucketName, false)

	// 6. Re-run sync -> must now succeed cleanly
	res, err := harness.RunSyncCycle(ctx, sourceDB, metaDB, driver, projectID, projectIID, []string{"request"}, 100, true)
	if err != nil {
		t.Fatalf("Re-run of RunSyncCycle after recovery failed: %v", err)
	}

	if res.RecordsDrained != 50 {
		t.Errorf("Expected 50 records drained on retry, got %d", res.RecordsDrained)
	}
	if res.RowsPruned != 50 {
		t.Errorf("Expected 50 rows pruned on retry, got %d", res.RowsPruned)
	}

	wmAfter, _ := metaDB.GetWatermark(projectID, "request")
	if wmAfter != 50 {
		t.Errorf("Expected watermark 50 after recovery, got %d", wmAfter)
	}
	if sourceDB.CountEntries() != 0 {
		t.Errorf("Expected source DB to be empty after successful retry, got %d", sourceDB.CountEntries())
	}
}

// ---------------------------------------------------------------------------
// Tier 3 - Test 3: Multi-Hour Batch Partition Splitting
// ---------------------------------------------------------------------------
func TestTier3_MultiHour_Batch_PartitionSplitting(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	storageDriver, _ := storage.NewLocalDriver(storage.LocalConfig{BasePath: tempDir})

	sourceDB := harness.NewMockTelescopeDatabase("postgres")
	metaDB := harness.NewMockMetadataDatabase()

	// Seed entries spanning across 3 different hours:
	// Hour 10 (seq 1..50), Hour 11 (seq 51..100), Hour 12 (seq 101..150)
	timeH10 := time.Date(2026, 8, 20, 10, 15, 0, 0, time.UTC)
	timeH11 := time.Date(2026, 8, 20, 11, 20, 0, 0, time.UTC)
	timeH12 := time.Date(2026, 8, 20, 12, 45, 0, 0, time.UTC)

	entries1, tags1 := harness.GenerateEntriesBatch(1, 50, timeH10, 10*time.Second, []string{"request"})
	entries2, tags2 := harness.GenerateEntriesBatch(51, 50, timeH11, 10*time.Second, []string{"request"})
	entries3, tags3 := harness.GenerateEntriesBatch(101, 50, timeH12, 10*time.Second, []string{"request"})

	sourceDB.Seed(entries1, tags1)
	sourceDB.Seed(entries2, tags2)
	sourceDB.Seed(entries3, tags3)

	projectIID := "multi-hour-proj"
	res, err := harness.RunSyncCycle(ctx, sourceDB, metaDB, storageDriver, 1, projectIID, []string{"request"}, 200, true)
	if err != nil {
		t.Fatalf("RunSyncCycle failed: %v", err)
	}

	if res.RecordsDrained != 150 {
		t.Errorf("Expected 150 records drained, got %d", res.RecordsDrained)
	}
	if res.ParquetFilesUploaded != 3 {
		t.Errorf("Expected exactly 3 hourly Parquet partition files, got %d", res.ParquetFilesUploaded)
	}

	// Verify all 3 hour partitions exist in storage
	hoursFound := make(map[int]bool)
	inspector := &harness.ParquetInspector{}

	for _, path := range res.PartitionPathsCreated {
		parsed, err := harness.ParseHivePath(path)
		if err != nil {
			t.Fatalf("Failed parsing partition path %s: %v", path, err)
		}
		hoursFound[parsed.Hour] = true

		rc, err := storageDriver.Get(ctx, path)
		if err != nil {
			t.Fatalf("Failed reading partition file %s: %v", path, err)
		}
		data, _ := io.ReadAll(rc)
		_ = rc.Close()

		insp, err := inspector.InspectParquetBytes(data)
		if err != nil {
			t.Fatalf("Parquet inspect failed on %s: %v", path, err)
		}
		if insp.NumRows != 50 {
			t.Errorf("Partition %s expected 50 rows, got %d", path, insp.NumRows)
		}
	}

	if !hoursFound[10] || !hoursFound[11] || !hoursFound[12] {
		t.Errorf("Expected partitions for hours 10, 11, 12. Found: %v", hoursFound)
	}
}

// ---------------------------------------------------------------------------
// Tier 3 - Test 4: Multi-Type Batch Routing
// ---------------------------------------------------------------------------
func TestTier3_MultiType_Batch_Routing(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	storageDriver, _ := storage.NewLocalDriver(storage.LocalConfig{BasePath: tempDir})

	sourceDB := harness.NewMockTelescopeDatabase("postgres")
	metaDB := harness.NewMockMetadataDatabase()
	baseTime := time.Date(2026, 8, 20, 14, 0, 0, 0, time.UTC)

	typesToSync := []string{"request", "query", "exception", "job", "log"}
	entries, tags := harness.GenerateEntriesBatch(1, 200, baseTime, 5*time.Second, typesToSync)
	sourceDB.Seed(entries, tags)

	projectIID := "multi-type-proj"
	res, err := harness.RunSyncCycle(ctx, sourceDB, metaDB, storageDriver, 1, projectIID, typesToSync, 500, true)
	if err != nil {
		t.Fatalf("RunSyncCycle failed: %v", err)
	}

	if res.RecordsDrained != 200 {
		t.Errorf("Expected 200 records drained across types, got %d", res.RecordsDrained)
	}

	// Verify each type has its own Hive partition path
	typesInPaths := make(map[string]bool)
	for _, path := range res.PartitionPathsCreated {
		parsed, err := harness.ParseHivePath(path)
		if err != nil {
			t.Fatalf("ParseHivePath failed for %s: %v", path, err)
		}
		typesInPaths[parsed.Type] = true
	}

	for _, et := range typesToSync {
		if !typesInPaths[et] {
			t.Errorf("Missing Hive partition path for entry type %s", et)
		}
		wm, _ := metaDB.GetWatermark(1, et)
		if wm == 0 {
			t.Errorf("Watermark for type %s was not updated", et)
		}
	}
}

// ---------------------------------------------------------------------------
// Tier 3 - Test 5: Prune Failure Fault Tolerance
// ---------------------------------------------------------------------------
func TestTier3_PruneFailure_Tolerance(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	storageDriver, _ := storage.NewLocalDriver(storage.LocalConfig{BasePath: tempDir})

	sourceDB := harness.NewMockTelescopeDatabase("postgres")
	metaDB := harness.NewMockMetadataDatabase()
	baseTime := time.Date(2026, 8, 20, 15, 0, 0, 0, time.UTC)
	entries, tags := harness.GenerateEntriesBatch(1, 30, baseTime, time.Second, []string{"request"})
	sourceDB.Seed(entries, tags)

	// Inject prune failure (e.g. database lock timeout)
	sourceDB.SetFailures(false, true)

	projectIID := "prune-fault-proj"
	_, err := harness.RunSyncCycle(ctx, sourceDB, metaDB, storageDriver, 1, projectIID, []string{"request"}, 100, true)
	if err == nil {
		t.Fatalf("Expected RunSyncCycle to report prune error, got nil")
	}

	// Parquet file was already uploaded
	objects, err := storageDriver.List(ctx, fmt.Sprintf("projectId=%s", projectIID))
	if err != nil || len(objects) == 0 {
		t.Errorf("Parquet file should have been uploaded before prune attempt")
	}

	// Watermark was committed prior to prune
	wm, _ := metaDB.GetWatermark(1, "request")
	if wm != 30 {
		t.Errorf("Watermark should have been committed (expected 30, got %d)", wm)
	}
}
