package e2e_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/divmora/periscope/internal/storage"
	"github.com/divmora/periscope/tests/e2e/harness"
	"github.com/google/uuid"
)

// setupMasterKey sets up a valid AES master key for crypto tests.
func setupMasterKey(t *testing.T) {
	t.Helper()
	orig := os.Getenv("AES_ENCRYPTION_KEY")
	t.Setenv("AES_ENCRYPTION_KEY", "12345678901234567890123456789012") // 32 bytes
	t.Cleanup(func() {
		if orig != "" {
			_ = os.Setenv("AES_ENCRYPTION_KEY", orig)
		}
	})
}

// ---------------------------------------------------------------------------
// Tier 1 - Test 1: Local Storage Driver Full Lifecycle
// ---------------------------------------------------------------------------
func TestTier1_LocalStorageDriver_FullLifecycle(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	driver, err := storage.NewLocalDriver(storage.LocalConfig{
		BasePath: tempDir,
	})
	if err != nil {
		t.Fatalf("Failed to instantiate LocalDriver: %v", err)
	}

	if driver.Type() != "local" {
		t.Errorf("Expected driver type 'local', got '%s'", driver.Type())
	}

	// 1. Ping
	if err := driver.Ping(ctx); err != nil {
		t.Fatalf("LocalDriver.Ping failed: %v", err)
	}

	// 2. Upload
	testKey := "projectId=p1/type=request/year=2026/month=08/day=20/hour=12/part-001.parquet"
	testContent := []byte("PARQUET_TEST_PAYLOAD_ZSTD_MOCK")

	err = driver.Upload(ctx, testKey, bytes.NewReader(testContent), int64(len(testContent)), "application/octet-stream")
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	// 3. Exists
	exists, err := driver.Exists(ctx, testKey)
	if err != nil {
		t.Fatalf("Exists check failed: %v", err)
	}
	if !exists {
		t.Errorf("Expected object %s to exist, but Exists returned false", testKey)
	}

	// 4. Get & Readback
	rc, err := driver.Get(ctx, testKey)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	readBack, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil {
		t.Fatalf("Failed reading object stream: %v", err)
	}
	if !bytes.Equal(readBack, testContent) {
		t.Errorf("Content mismatch: expected %q, got %q", string(testContent), string(readBack))
	}

	// 5. List
	objects, err := driver.List(ctx, "projectId=p1/type=request")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(objects) != 1 {
		t.Fatalf("Expected 1 listed object, got %d", len(objects))
	}
	if objects[0].Key != testKey {
		t.Errorf("Listed key mismatch: expected %s, got %s", testKey, objects[0].Key)
	}
	if objects[0].Size != int64(len(testContent)) {
		t.Errorf("Listed size mismatch: expected %d, got %d", len(testContent), objects[0].Size)
	}

	// 6. Delete
	err = driver.Delete(ctx, testKey)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	existsAfter, err := driver.Exists(ctx, testKey)
	if err != nil {
		t.Fatalf("Exists after delete failed: %v", err)
	}
	if existsAfter {
		t.Errorf("Expected object %s to be deleted, but Exists returned true", testKey)
	}
}

// ---------------------------------------------------------------------------
// Tier 1 - Test 2: S3 / MinIO Storage Driver Full Lifecycle
// ---------------------------------------------------------------------------
func TestTier1_S3StorageDriver_FullLifecycle(t *testing.T) {
	ctx := context.Background()
	s3Mock := harness.NewS3MockServer()
	defer s3Mock.Close()

	bucketName := "periscope-lakehouse-test"
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
		t.Fatalf("Failed to initialize S3 driver via factory: %v", err)
	}

	// 1. Ping valid bucket
	if err := driver.Ping(ctx); err != nil {
		t.Fatalf("Driver.Ping on valid bucket failed: %v", err)
	}

	// 2. Upload
	s3Key := "projectId=019123/type=query/year=2026/month=08/day=20/hour=14/part-0001.parquet"
	payload := []byte("S3_COMPRESSED_PARQUET_BATCH_DATA")
	err = driver.Upload(ctx, s3Key, bytes.NewReader(payload), int64(len(payload)), "application/vnd.apache.parquet")
	if err != nil {
		t.Fatalf("S3 Upload failed: %v", err)
	}

	// 3. Exists
	exists, err := driver.Exists(ctx, s3Key)
	if err != nil {
		t.Fatalf("S3 Exists failed: %v", err)
	}
	if !exists {
		t.Errorf("Expected object %s to exist in S3", s3Key)
	}

	// 4. Get & Readback
	rc, err := driver.Get(ctx, s3Key)
	if err != nil {
		t.Fatalf("S3 Get failed: %v", err)
	}
	data, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil {
		t.Fatalf("S3 ReadAll failed: %v", err)
	}
	if !bytes.Equal(data, payload) {
		t.Errorf("S3 Readback mismatch: expected %q, got %q", string(payload), string(data))
	}

	// 5. List with prefix
	listRes, err := driver.List(ctx, "projectId=019123/type=query")
	if err != nil {
		t.Fatalf("S3 List failed: %v", err)
	}
	if len(listRes) != 1 {
		t.Fatalf("Expected 1 object in S3 list, got %d", len(listRes))
	}
	if listRes[0].Key != s3Key {
		t.Errorf("S3 List key mismatch: expected %s, got %s", s3Key, listRes[0].Key)
	}

	// 6. Delete
	if err := driver.Delete(ctx, s3Key); err != nil {
		t.Fatalf("S3 Delete failed: %v", err)
	}
	existsAfter, err := driver.Exists(ctx, s3Key)
	if err != nil {
		t.Fatalf("S3 Exists check after delete failed: %v", err)
	}
	if existsAfter {
		t.Errorf("Expected S3 object %s to be deleted", s3Key)
	}
}

// ---------------------------------------------------------------------------
// Tier 1 - Test 3: Multi-Dialect Telescope Extraction (MySQL & PostgreSQL)
// ---------------------------------------------------------------------------
func TestTier1_MultiDialect_TelescopeExtraction(t *testing.T) {
	dialects := []string{"mysql", "postgres"}

	for _, dialect := range dialects {
		t.Run(dialect, func(t *testing.T) {
			db := harness.NewMockTelescopeDatabase(dialect)
			baseTime := time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC)
			entries, tags := harness.GenerateEntriesBatch(1, len(harness.AllKnownEntryTypes)*2, baseTime, time.Minute, harness.AllKnownEntryTypes)

			db.Seed(entries, tags)

			// Verify total seeded
			if db.CountEntries() != len(entries) {
				t.Fatalf("Seeded entry count mismatch: expected %d, got %d", len(entries), db.CountEntries())
			}

			// Drain entries per type
			for _, entryType := range harness.AllKnownEntryTypes {
				drained, err := db.DrainEntries(entryType, 0, 100)
				if err != nil {
					t.Fatalf("DrainEntries failed for dialect %s, type %s: %v", dialect, entryType, err)
				}
				if len(drained) != 2 {
					t.Errorf("[%s] Expected 2 entries for type %s, got %d", dialect, entryType, len(drained))
				}
				for _, e := range drained {
					if e.Type != entryType {
						t.Errorf("[%s] Entry type mismatch: expected %s, got %s", dialect, entryType, e.Type)
					}
					if len(e.Tags) == 0 {
						t.Errorf("[%s] Expected tags attached to entry %s, got 0", dialect, e.UUID)
					}
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tier 1 - Test 4: Tag Aggregation Pipeline (No Row Multiplication)
// ---------------------------------------------------------------------------
func TestTier1_TagAggregation(t *testing.T) {
	db := harness.NewMockTelescopeDatabase("postgres")
	baseTime := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)

	// Create 10 request entries
	entries, tags := harness.GenerateEntriesBatch(1, 10, baseTime, 5*time.Second, []string{"request"})
	db.Seed(entries, tags)

	drained, err := db.DrainEntries("request", 0, 50)
	if err != nil {
		t.Fatalf("DrainEntries failed: %v", err)
	}

	if len(drained) != 10 {
		t.Fatalf("Expected exactly 10 distinct entries without row multiplication, got %d", len(drained))
	}

	// Verify each entry has distinct tags matching the seed
	for _, entry := range drained {
		if len(entry.Tags) == 0 {
			t.Errorf("Entry %s has 0 tags", entry.UUID)
		}
		// Verify tags contain Auth:, Status:, Method:
		var hasStatusTag bool
		for _, tag := range entry.Tags {
			if len(tag) > 7 && tag[:7] == "Status:" {
				hasStatusTag = true
			}
		}
		if !hasStatusTag {
			t.Errorf("Entry %s missing expected Status: tag, tags: %v", entry.UUID, entry.Tags)
		}
	}
}

// ---------------------------------------------------------------------------
// Tier 1 - Test 5: Dimension Extraction Across 16+ Types
// ---------------------------------------------------------------------------
func TestTier1_DimensionExtraction(t *testing.T) {
	baseTime := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	entries, _ := harness.GenerateEntriesBatch(1, len(harness.AllKnownEntryTypes), baseTime, time.Minute, harness.AllKnownEntryTypes)

	for _, entry := range entries {
		rec := harness.ExtractDimensions(entry)

		if rec.Sequence != entry.Sequence {
			t.Errorf("Sequence mismatch: expected %d, got %d", entry.Sequence, rec.Sequence)
		}
		if rec.Type != entry.Type {
			t.Errorf("Type mismatch: expected %s, got %s", entry.Type, rec.Type)
		}
		if rec.Content != entry.Content {
			t.Errorf("Content raw JSON mismatch for type %s", entry.Type)
		}

		switch entry.Type {
		case "request":
			if rec.HTTPStatus == nil || *rec.HTTPStatus == 0 {
				t.Errorf("request dimension HTTPStatus is missing or 0")
			}
			if rec.HTTPMethod == nil || *rec.HTTPMethod == "" {
				t.Errorf("request dimension HTTPMethod is missing")
			}
			if rec.HTTPURI == nil || *rec.HTTPURI == "" {
				t.Errorf("request dimension HTTPURI is missing")
			}
			if rec.DurationMS == nil {
				t.Errorf("request dimension DurationMS is missing")
			}

		case "query":
			if rec.QuerySQL == nil || *rec.QuerySQL == "" {
				t.Errorf("query dimension QuerySQL is missing")
			}
			if rec.QueryConnection == nil || *rec.QueryConnection == "" {
				t.Errorf("query dimension QueryConnection is missing")
			}
			if rec.QueryTimeMS == nil {
				t.Errorf("query dimension QueryTimeMS is missing")
			}

		case "exception":
			if rec.ExceptionClass == nil || *rec.ExceptionClass == "" {
				t.Errorf("exception dimension ExceptionClass is missing")
			}
			if rec.ExceptionMessage == nil || *rec.ExceptionMessage == "" {
				t.Errorf("exception dimension ExceptionMessage is missing")
			}

		case "log":
			if rec.LogLevel == nil || *rec.LogLevel == "" {
				t.Errorf("log dimension LogLevel is missing")
			}
			if rec.LogMessage == nil || *rec.LogMessage == "" {
				t.Errorf("log dimension LogMessage is missing")
			}

		case "job":
			if rec.JobName == nil || *rec.JobName == "" {
				t.Errorf("job dimension JobName is missing")
			}
			if rec.JobStatus == nil || *rec.JobStatus == "" {
				t.Errorf("job dimension JobStatus is missing")
			}

		case "command":
			if rec.CommandName == nil || *rec.CommandName == "" {
				t.Errorf("command dimension CommandName is missing")
			}
			if rec.CommandExitCode == nil {
				t.Errorf("command dimension CommandExitCode is missing")
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Tier 1 - Test 6: Parquet ZSTD Encoding and Schema Inspection
// ---------------------------------------------------------------------------
func TestTier1_ParquetZSTDEncoding_AndSchema(t *testing.T) {
	baseTime := time.Date(2026, 8, 20, 14, 0, 0, 0, time.UTC)
	rawEntries, tags := harness.GenerateEntriesBatch(1, 100, baseTime, 10*time.Second, []string{"request", "query", "exception", "job"})

	// Attach tags
	tagMap := make(map[string][]string)
	for _, t := range tags {
		tagMap[t.EntryUUID] = append(tagMap[t.EntryUUID], t.Tag)
	}
	records := make([]harness.TelemetryRecord, len(rawEntries))
	for i, re := range rawEntries {
		re.Tags = tagMap[re.UUID]
		records[i] = harness.ExtractDimensions(re)
	}

	parquetBytes, err := harness.EncodeParquet(records)
	if err != nil {
		t.Fatalf("EncodeParquet failed: %v", err)
	}

	if len(parquetBytes) == 0 {
		t.Fatalf("Encoded Parquet bytes are empty")
	}

	// Inspect with ParquetInspector
	inspector := &harness.ParquetInspector{}
	inspection, err := inspector.InspectParquetBytes(parquetBytes)
	if err != nil {
		t.Fatalf("InspectParquetBytes failed: %v", err)
	}

	if inspection.NumRows != 100 {
		t.Errorf("Expected 100 rows in Parquet file, got %d", inspection.NumRows)
	}
	if !inspection.HasZSTD {
		t.Errorf("Parquet file does not have ZSTD compression active")
	}

	// Verify required columns exist
	requiredCols := []string{
		"sequence", "uuid", "batch_id", "type", "created_at", "tags", "content",
		"http_status", "http_method", "query_sql", "exception_class", "job_name",
	}
	for _, col := range requiredCols {
		if !inspection.HasColumn(col) {
			t.Errorf("Parquet schema missing required column: %s", col)
		}
	}

	// Read rows back and assert fidelity
	readRows, err := inspector.ReadTelemetryRows(parquetBytes)
	if err != nil {
		t.Fatalf("ReadTelemetryRows failed: %v", err)
	}
	if len(readRows) != 100 {
		t.Fatalf("ReadTelemetryRows count mismatch: expected 100, got %d", len(readRows))
	}
	if readRows[0].Sequence != 1 || readRows[99].Sequence != 100 {
		t.Errorf("Sequence order mismatch: first=%d, last=%d", readRows[0].Sequence, readRows[99].Sequence)
	}
}

// ---------------------------------------------------------------------------
// Tier 1 - Test 7: Hive Partition Path Format
// ---------------------------------------------------------------------------
func TestTier1_HivePartitionPathFormat(t *testing.T) {
	projectIID := "01912345-6789-7abc-def0-1234567890ab"
	entryType := "request"
	timestamp := time.Date(2026, 8, 20, 15, 30, 45, 0, time.UTC)
	minSeq := int64(10001)
	maxSeq := int64(20000)
	batchUUID := "019123456789"

	path := harness.GenerateHivePartitionPath(projectIID, entryType, timestamp, minSeq, maxSeq, batchUUID)

	expectedPath := "projectId=01912345-6789-7abc-def0-1234567890ab/type=request/year=2026/month=08/day=20/hour=15/part-000000010001-000000020000-019123456789.parquet"
	if path != expectedPath {
		t.Errorf("Generated Hive path mismatch:\nExpected: %s\nGot:      %s", expectedPath, path)
	}

	// Validate with Hive path parser
	parsed, err := harness.ParseHivePath(path)
	if err != nil {
		t.Fatalf("ParseHivePath failed: %v", err)
	}
	if parsed.ProjectID != projectIID {
		t.Errorf("Parsed ProjectID mismatch: %s vs %s", parsed.ProjectID, projectIID)
	}
	if parsed.Type != entryType {
		t.Errorf("Parsed Type mismatch: %s vs %s", parsed.Type, entryType)
	}
	if parsed.Year != 2026 || parsed.Month != 8 || parsed.Day != 20 || parsed.Hour != 15 {
		t.Errorf("Parsed timestamp partition components mismatch: %d-%d-%d %d:00", parsed.Year, parsed.Month, parsed.Day, parsed.Hour)
	}
}

// ---------------------------------------------------------------------------
// Tier 1 - Test 8: Watermark Checkpointing Progression
// ---------------------------------------------------------------------------
func TestTier1_WatermarkCheckpointing(t *testing.T) {
	metaDB := harness.NewMockMetadataDatabase()
	projectID := 42
	entryType := "request"

	// Initial watermark should be 0
	wm, err := metaDB.GetWatermark(projectID, entryType)
	if err != nil {
		t.Fatalf("GetWatermark failed: %v", err)
	}
	if wm != 0 {
		t.Errorf("Expected initial watermark 0, got %d", wm)
	}

	// Update to sequence 500
	if err := metaDB.UpsertWatermark(projectID, entryType, 500, time.Now()); err != nil {
		t.Fatalf("UpsertWatermark failed: %v", err)
	}
	wm, _ = metaDB.GetWatermark(projectID, entryType)
	if wm != 500 {
		t.Errorf("Expected watermark 500, got %d", wm)
	}

	// Ensure monotonicity: updating with smaller sequence (300) does NOT regress watermark
	if err := metaDB.UpsertWatermark(projectID, entryType, 300, time.Now()); err != nil {
		t.Fatalf("UpsertWatermark with smaller seq failed: %v", err)
	}
	wm, _ = metaDB.GetWatermark(projectID, entryType)
	if wm != 500 {
		t.Errorf("Watermark regressed! Expected 500, got %d", wm)
	}

	// Advance to sequence 1200
	if err := metaDB.UpsertWatermark(projectID, entryType, 1200, time.Now()); err != nil {
		t.Fatalf("UpsertWatermark to 1200 failed: %v", err)
	}
	wm, _ = metaDB.GetWatermark(projectID, entryType)
	if wm != 1200 {
		t.Errorf("Expected watermark 1200, got %d", wm)
	}
}

// ---------------------------------------------------------------------------
// Tier 1 - Test 9: Source Table Pruning Behavior
// ---------------------------------------------------------------------------
func TestTier1_SourcePruning_Behavior(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	storageDriver, _ := storage.NewLocalDriver(storage.LocalConfig{BasePath: tempDir})

	baseTime := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)

	// Scenario A: Project with prune_enabled = true
	t.Run("Pruning_Enabled", func(t *testing.T) {
		sourceDB := harness.NewMockTelescopeDatabase("postgres")
		metaDB := harness.NewMockMetadataDatabase()
		entries, tags := harness.GenerateEntriesBatch(1, 50, baseTime, time.Second, []string{"request"})
		sourceDB.Seed(entries, tags)

		res, err := harness.RunSyncCycle(ctx, sourceDB, metaDB, storageDriver, 1, "p-prune", []string{"request"}, 100, true)
		if err != nil {
			t.Fatalf("RunSyncCycle failed: %v", err)
		}
		if res.RowsPruned != 50 {
			t.Errorf("Expected 50 rows pruned, got %d", res.RowsPruned)
		}
		if sourceDB.CountEntries() != 0 {
			t.Errorf("Expected 0 remaining entries in source DB, got %d", sourceDB.CountEntries())
		}
		if sourceDB.CountTags() != 0 {
			t.Errorf("Expected 0 remaining tags in source DB, got %d", sourceDB.CountTags())
		}
	})

	// Scenario B: Project with prune_enabled = false
	t.Run("Pruning_Disabled", func(t *testing.T) {
		sourceDB := harness.NewMockTelescopeDatabase("postgres")
		metaDB := harness.NewMockMetadataDatabase()
		entries, tags := harness.GenerateEntriesBatch(1, 50, baseTime, time.Second, []string{"request"})
		sourceDB.Seed(entries, tags)

		res, err := harness.RunSyncCycle(ctx, sourceDB, metaDB, storageDriver, 2, "p-noprune", []string{"request"}, 100, false)
		if err != nil {
			t.Fatalf("RunSyncCycle failed: %v", err)
		}
		if res.RowsPruned != 0 {
			t.Errorf("Expected 0 rows pruned, got %d", res.RowsPruned)
		}
		if sourceDB.CountEntries() != 50 {
			t.Errorf("Expected 50 remaining entries in source DB, got %d", sourceDB.CountEntries())
		}
		if sourceDB.CountTags() != len(tags) {
			t.Errorf("Expected %d remaining tags in source DB, got %d", len(tags), sourceDB.CountTags())
		}
	})
}

// ---------------------------------------------------------------------------
// Tier 1 - Test 10: Complete Sync CLI Workflow Execution
// ---------------------------------------------------------------------------
func TestTier1_CLISync_Workflow(t *testing.T) {
	setupMasterKey(t)
	ctx := context.Background()

	// 1. Setup mock S3 Lakehouse
	s3Mock := harness.NewS3MockServer()
	defer s3Mock.Close()
	bucket := "periscope-e2e-sync"
	s3Mock.CreateBucket(bucket)

	// 2. Setup project with encrypted credentials
	projectIID := uuid.NewString()
	project, err := harness.CreateEncryptedProjectHelper(
		projectIID,
		"Production App",
		"postgres://laravel:secret@127.0.0.1:5432/laravel_db",
		"minio",
		bucket,
		"us-east-1",
		s3Mock.URL(),
		"minioadmin",
		"minioadmin",
		true,
	)
	if err != nil {
		t.Fatalf("Failed creating encrypted project: %v", err)
	}

	metaDB := harness.NewMockMetadataDatabase()
	_ = metaDB.AddProject(101, project)

	// 3. Setup source DB and seed 300 entries across request, query, exception, job, log
	sourceDB := harness.NewMockTelescopeDatabase("postgres")
	baseTime := time.Date(2026, 8, 20, 16, 0, 0, 0, time.UTC)
	selectedTypes := []string{"request", "query", "exception", "job", "log"}
	entries, tags := harness.GenerateEntriesBatch(1, 300, baseTime, 5*time.Second, selectedTypes)
	sourceDB.Seed(entries, tags)

	// 4. Initialize storage driver using decrypted project config
	storageDriver, err := storage.NewDriverFromConfig(ctx, storage.Config{
		Type:            project.StorageType,
		Bucket:          project.StorageBucket,
		Region:          project.StorageRegion,
		Endpoint:        project.StorageEndpoint,
		AccessKeyID:     project.StorageAccessKey,
		SecretAccessKey: project.StorageSecretKey,
		UsePathStyle:    true,
		DisableSSL:      true,
	})
	if err != nil {
		t.Fatalf("NewDriverFromConfig failed: %v", err)
	}

	// 5. Execute Sync
	syncRes, err := harness.RunSyncCycle(ctx, sourceDB, metaDB, storageDriver, 101, project.IID, selectedTypes, 1000, project.PruneEnabled)
	if err != nil {
		t.Fatalf("RunSyncCycle failed: %v", err)
	}

	if syncRes.RecordsDrained != 300 {
		t.Errorf("Expected 300 records drained, got %d", syncRes.RecordsDrained)
	}
	if syncRes.ParquetFilesUploaded == 0 {
		t.Errorf("Expected >= 1 Parquet files uploaded")
	}
	if syncRes.RowsPruned != 300 {
		t.Errorf("Expected 300 rows pruned, got %d", syncRes.RowsPruned)
	}

	// 6. Verify Watermarks in Metadata DB
	for _, et := range selectedTypes {
		wm, err := metaDB.GetWatermark(101, et)
		if err != nil {
			t.Fatalf("Failed getting watermark for %s: %v", et, err)
		}
		if wm == 0 {
			t.Errorf("Watermark for type %s was not advanced (got 0)", et)
		}
	}

	// 7. Verify Parquet files in S3 Mock Server
	keys := s3Mock.ListKeys(bucket, fmt.Sprintf("projectId=%s", projectIID))
	if len(keys) == 0 {
		t.Fatalf("No Parquet files found in S3 bucket under projectId=%s", projectIID)
	}

	inspector := &harness.ParquetInspector{}
	var totalVerifiedRows int64
	for _, key := range keys {
		data, ok := s3Mock.GetObjectData(bucket, key)
		if !ok {
			t.Fatalf("Failed reading object %s from S3 mock", key)
		}
		res, err := inspector.InspectParquetBytes(data)
		if err != nil {
			t.Fatalf("Parquet inspection failed for %s: %v", key, err)
		}
		if !res.HasZSTD {
			t.Errorf("Parquet file %s missing ZSTD compression", key)
		}
		totalVerifiedRows += res.NumRows
	}

	if totalVerifiedRows != 300 {
		t.Errorf("Total verified Parquet rows in storage mismatch: expected 300, got %d", totalVerifiedRows)
	}
}
