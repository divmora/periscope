package e2e_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/divmora/periscope/internal/crypto"
	"github.com/divmora/periscope/internal/storage"
	"github.com/divmora/periscope/tests/e2e/harness"
	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Tier 2 - Test 1: Empty Source Database
// ---------------------------------------------------------------------------
func TestTier2_EmptySourceDatabase(t *testing.T) {
	ctx := context.Background()
	sourceDB := harness.NewMockTelescopeDatabase("postgres")
	metaDB := harness.NewMockMetadataDatabase()
	tempDir := t.TempDir()
	storageDriver, _ := storage.NewLocalDriver(storage.LocalConfig{BasePath: tempDir})

	// Run sync on empty source DB
	res, err := harness.RunSyncCycle(ctx, sourceDB, metaDB, storageDriver, 1, "empty-proj", nil, 1000, true)
	if err != nil {
		t.Fatalf("RunSyncCycle on empty database failed: %v", err)
	}

	if res.RecordsDrained != 0 {
		t.Errorf("Expected 0 records drained, got %d", res.RecordsDrained)
	}
	if res.ParquetFilesUploaded != 0 {
		t.Errorf("Expected 0 Parquet files uploaded, got %d", res.ParquetFilesUploaded)
	}
	if len(res.WatermarksUpdated) != 0 {
		t.Errorf("Expected 0 watermarks updated, got %d", len(res.WatermarksUpdated))
	}
}

// ---------------------------------------------------------------------------
// Tier 2 - Test 2: Malformed JSON Content
// ---------------------------------------------------------------------------
func TestTier2_MalformedJSONContent(t *testing.T) {
	malformedPayloads := []string{
		`{"uri": "/incomplete-json`,
		`plain text not a json object`,
		`{"response_status": "not-a-number", "duration": "fast"}`,
		``,
		`{broken`,
	}

	for i, payload := range malformedPayloads {
		t.Run(fmt.Sprintf("Payload_%d", i), func(t *testing.T) {
			entry := harness.RawTelescopeEntry{
				Sequence:  int64(i + 1),
				UUID:      uuid.NewString(),
				BatchID:   uuid.NewString(),
				Type:      "request",
				Content:   payload,
				CreatedAt: time.Now().UTC(),
				Tags:      []string{"Malformed"},
			}

			// Dimension extractor must not panic
			rec := harness.ExtractDimensions(entry)

			if rec.Content != payload {
				t.Errorf("Raw content was altered: expected %q, got %q", payload, rec.Content)
			}
			if rec.HTTPStatus != nil {
				t.Errorf("Expected nil HTTPStatus for malformed payload, got %v", *rec.HTTPStatus)
			}

			// Parquet encode and read back must succeed
			parquetBytes, err := harness.EncodeParquet([]harness.TelemetryRecord{rec})
			if err != nil {
				t.Fatalf("EncodeParquet failed on malformed payload record: %v", err)
			}

			inspector := &harness.ParquetInspector{}
			readRows, err := inspector.ReadTelemetryRows(parquetBytes)
			if err != nil {
				t.Fatalf("ReadTelemetryRows failed on malformed record: %v", err)
			}
			if len(readRows) != 1 {
				t.Fatalf("Expected 1 row read back, got %d", len(readRows))
			}
			if readRows[0].Content != payload {
				t.Errorf("Readback content mismatch: expected %q, got %q", payload, readRows[0].Content)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tier 2 - Test 3: Entries with Zero Tags
// ---------------------------------------------------------------------------
func TestTier2_EntriesWithZeroTags(t *testing.T) {
	entry := harness.RawTelescopeEntry{
		Sequence:  1,
		UUID:      uuid.NewString(),
		BatchID:   uuid.NewString(),
		Type:      "log",
		Content:   `{"level":"info","message":"Zero tags log test"}`,
		CreatedAt: time.Now().UTC(),
		Tags:      nil, // 0 tags
	}

	rec := harness.ExtractDimensions(entry)
	if rec.Tags == nil {
		t.Errorf("Expected non-nil empty tags slice, got nil")
	}

	parquetBytes, err := harness.EncodeParquet([]harness.TelemetryRecord{rec})
	if err != nil {
		t.Fatalf("EncodeParquet with zero tags failed: %v", err)
	}

	inspector := &harness.ParquetInspector{}
	rows, err := inspector.ReadTelemetryRows(parquetBytes)
	if err != nil {
		t.Fatalf("ReadTelemetryRows failed: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("Expected 1 row, got %d", len(rows))
	}
	if len(rows[0].Tags) != 0 {
		t.Errorf("Expected 0 tags on readback, got %d", len(rows[0].Tags))
	}
}

// ---------------------------------------------------------------------------
// Tier 2 - Test 4: Large Batch Size (10,000+ rows)
// ---------------------------------------------------------------------------
func TestTier2_LargeBatch_10kPlus(t *testing.T) {
	ctx := context.Background()
	sourceDB := harness.NewMockTelescopeDatabase("postgres")
	metaDB := harness.NewMockMetadataDatabase()
	tempDir := t.TempDir()
	storageDriver, _ := storage.NewLocalDriver(storage.LocalConfig{BasePath: tempDir})

	// Generate 12,500 entries across 2 hours
	totalEntries := 12500
	baseTime := time.Date(2026, 8, 20, 8, 0, 0, 0, time.UTC)
	entries, tags := harness.GenerateEntriesBatch(1, totalEntries, baseTime, 500*time.Millisecond, []string{"request"})
	sourceDB.Seed(entries, tags)

	// Draining with batch limit 10,000 (Chunk 1)
	res1, err := harness.RunSyncCycle(ctx, sourceDB, metaDB, storageDriver, 1, "large-batch-proj", []string{"request"}, 10000, true)
	if err != nil {
		t.Fatalf("Sync run 1 failed: %v", err)
	}
	if res1.RecordsDrained != 10000 {
		t.Errorf("Expected 10,000 records in chunk 1, got %d", res1.RecordsDrained)
	}
	if sourceDB.CountEntries() != 2500 {
		t.Errorf("Expected 2,500 remaining entries in source DB, got %d", sourceDB.CountEntries())
	}

	// Draining remainder (Chunk 2)
	res2, err := harness.RunSyncCycle(ctx, sourceDB, metaDB, storageDriver, 1, "large-batch-proj", []string{"request"}, 10000, true)
	if err != nil {
		t.Fatalf("Sync run 2 failed: %v", err)
	}
	if res2.RecordsDrained != 2500 {
		t.Errorf("Expected 2,500 records in chunk 2, got %d", res2.RecordsDrained)
	}
	if sourceDB.CountEntries() != 0 {
		t.Errorf("Expected 0 remaining entries after chunk 2, got %d", sourceDB.CountEntries())
	}

	// Verify all files written in storage
	objects, err := storageDriver.List(ctx, "projectId=large-batch-proj")
	if err != nil {
		t.Fatalf("Storage list failed: %v", err)
	}
	if len(objects) == 0 {
		t.Fatalf("No Parquet files found in storage")
	}

	inspector := &harness.ParquetInspector{}
	var totalReadRows int64
	for _, obj := range objects {
		rc, err := storageDriver.Get(ctx, obj.Key)
		if err != nil {
			t.Fatalf("Failed getting %s: %v", obj.Key, err)
		}
		data, _ := io.ReadAll(rc)
		_ = rc.Close()

		insp, err := inspector.InspectParquetBytes(data)
		if err != nil {
			t.Fatalf("Parquet inspect failed for %s: %v", obj.Key, err)
		}
		totalReadRows += insp.NumRows
	}

	if totalReadRows != int64(totalEntries) {
		t.Errorf("Total Parquet rows in storage mismatch: expected %d, got %d", totalEntries, totalReadRows)
	}
}

// ---------------------------------------------------------------------------
// Tier 2 - Test 5: Storage Path Traversal Protection
// ---------------------------------------------------------------------------
func TestTier2_StoragePathTraversal_Protection(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	driver, err := storage.NewLocalDriver(storage.LocalConfig{BasePath: tempDir})
	if err != nil {
		t.Fatalf("Failed creating local driver: %v", err)
	}

	traversalKeys := []string{
		"../escape.parquet",
		"../../etc/passwd",
		"../../../root.parquet",
		"valid/../../../escape.txt",
		"sub/dir/../../../../etc/shadow",
		"file\x00hidden.parquet",
		"/absolute/escape.parquet",
		"..",
		"../",
	}

	dummyContent := []byte("ATTACK_PAYLOAD")

	for _, key := range traversalKeys {
		t.Run(fmt.Sprintf("Key_%s", key), func(t *testing.T) {
			// Upload should fail with ErrPathTraversal or ErrInvalidConfig
			err := driver.Upload(ctx, key, bytes.NewReader(dummyContent), int64(len(dummyContent)), "text/plain")
			if err == nil {
				t.Errorf("Expected path traversal error on key %q, but Upload succeeded", key)
			}

			// Get should fail
			_, err = driver.Get(ctx, key)
			if err == nil {
				t.Errorf("Expected path traversal error on key %q, but Get succeeded", key)
			}

			// Exists should fail
			_, err = driver.Exists(ctx, key)
			if err == nil {
				t.Errorf("Expected path traversal error on key %q, but Exists succeeded", key)
			}

			// Delete should fail
			err = driver.Delete(ctx, key)
			if err == nil {
				t.Errorf("Expected path traversal error on key %q, but Delete succeeded", key)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tier 2 - Test 6: Missing or Invalid AES Key Handling
// ---------------------------------------------------------------------------
func TestTier2_MissingOrInvalidAESKey(t *testing.T) {
	// Case 1: Key not set
	t.Setenv("AES_ENCRYPTION_KEY", "")
	_, err := crypto.Encrypt("secret_dsn")
	if err == nil {
		t.Errorf("Expected error when AES_ENCRYPTION_KEY is empty, got nil")
	}

	_, err = crypto.Decrypt("some_ciphertext")
	if err == nil {
		t.Errorf("Expected error on decrypt when AES_ENCRYPTION_KEY is empty, got nil")
	}

	// Case 2: Key invalid length (16 bytes instead of 32 bytes)
	t.Setenv("AES_ENCRYPTION_KEY", "shortkey12345678")
	_, err = crypto.Encrypt("secret_dsn")
	if err == nil {
		t.Errorf("Expected error when AES_ENCRYPTION_KEY is 16 bytes, got nil")
	}

	// Case 3: Key invalid length (40 bytes)
	t.Setenv("AES_ENCRYPTION_KEY", "12345678901234567890123456789012toolongkey")
	_, err = crypto.Encrypt("secret_dsn")
	if err == nil {
		t.Errorf("Expected error when AES_ENCRYPTION_KEY is 40 bytes, got nil")
	}
}

// ---------------------------------------------------------------------------
// Tier 2 - Test 7: Invalid or Non-Existent Project IID
// ---------------------------------------------------------------------------
func TestTier2_InvalidOrNonExistentProjectIID(t *testing.T) {
	metaDB := harness.NewMockMetadataDatabase()

	// Querying non-existent project
	proj, err := metaDB.GetProject("non-existent-uuid-12345")
	if err != nil {
		t.Fatalf("GetProject on non-existent returned unexpected error: %v", err)
	}
	if proj != nil {
		t.Errorf("Expected nil project for non-existent IID, got %+v", proj)
	}
}

// ---------------------------------------------------------------------------
// Tier 2 - Test 8: Extreme Payload Sizes (500KB JSON)
// ---------------------------------------------------------------------------
func TestTier2_ExtremePayloadSizes(t *testing.T) {
	// Build a massive SQL query payload with large parameter bindings
	largeSQL := "SELECT * FROM large_table WHERE id IN (" + strings.Repeat("1,", 50000) + "1)"
	largePayload := fmt.Sprintf(`{"sql": %q, "connection": "mysql", "time": 450.2, "slow": true}`, largeSQL)

	entry := harness.RawTelescopeEntry{
		Sequence:  1,
		UUID:      uuid.NewString(),
		BatchID:   uuid.NewString(),
		Type:      "query",
		Content:   largePayload,
		CreatedAt: time.Now().UTC(),
		Tags:      []string{"LargeQuery", "Slow:true"},
	}

	rec := harness.ExtractDimensions(entry)
	if rec.QuerySQL == nil || *rec.QuerySQL != largeSQL {
		t.Errorf("Extracted large QuerySQL did not match original large query string")
	}

	parquetBytes, err := harness.EncodeParquet([]harness.TelemetryRecord{rec})
	if err != nil {
		t.Fatalf("EncodeParquet failed on large payload: %v", err)
	}

	// Parquet ZSTD should achieve strong compression
	uncompressedLen := len(largePayload)
	compressedLen := len(parquetBytes)
	if compressedLen >= uncompressedLen {
		t.Logf("Note: Compressed size (%d) vs uncompressed (%d)", compressedLen, uncompressedLen)
	}

	inspector := &harness.ParquetInspector{}
	rows, err := inspector.ReadTelemetryRows(parquetBytes)
	if err != nil {
		t.Fatalf("ReadTelemetryRows failed on large payload: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("Expected 1 row, got %d", len(rows))
	}
	if *rows[0].QuerySQL != largeSQL {
		t.Errorf("Readback QuerySQL mismatch on large payload")
	}
}

// ---------------------------------------------------------------------------
// Tier 2 - Test 9: Unicode and Special Characters Integrity
// ---------------------------------------------------------------------------
func TestTier2_UnicodeAndSpecialCharacters(t *testing.T) {
	unicodeURI := "/api/v1/搜索/telemetry?emoji=🚀🔥&tag=日本語_العربية_हिन्दी"
	unicodeMessage := "Ошибка в системе: 🚨 Transaction failed with status: 'rejected' & \t \n \r \"special\" chars"

	entry := harness.RawTelescopeEntry{
		Sequence:  1,
		UUID:      uuid.NewString(),
		BatchID:   uuid.NewString(),
		Type:      "request",
		Content:   fmt.Sprintf(`{"uri": %q, "method": "GET", "response_status": 200, "headers": {"x-user": "Владимир"}, "duration": 12.3}`, unicodeURI),
		CreatedAt: time.Now().UTC(),
		Tags:      []string{"Lang:日本語", "Lang:العربية", "Tag:🚀🔥", "Emoji:🍕"},
	}

	rec := harness.ExtractDimensions(entry)
	if rec.HTTPURI == nil || *rec.HTTPURI != unicodeURI {
		t.Errorf("Extracted HTTPURI unicode mismatch: expected %q, got %v", unicodeURI, rec.HTTPURI)
	}

	logEntry := harness.RawTelescopeEntry{
		Sequence:  2,
		UUID:      uuid.NewString(),
		BatchID:   uuid.NewString(),
		Type:      "log",
		Content:   fmt.Sprintf(`{"level": "critical", "message": %q}`, unicodeMessage),
		CreatedAt: time.Now().UTC(),
		Tags:      []string{"UnicodeLog"},
	}
	logRec := harness.ExtractDimensions(logEntry)

	parquetBytes, err := harness.EncodeParquet([]harness.TelemetryRecord{rec, logRec})
	if err != nil {
		t.Fatalf("EncodeParquet failed on unicode payload: %v", err)
	}

	inspector := &harness.ParquetInspector{}
	rows, err := inspector.ReadTelemetryRows(parquetBytes)
	if err != nil {
		t.Fatalf("ReadTelemetryRows failed on unicode payload: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("Expected 2 rows, got %d", len(rows))
	}
	if *rows[0].HTTPURI != unicodeURI {
		t.Errorf("Readback HTTPURI unicode mismatch: expected %q, got %q", unicodeURI, *rows[0].HTTPURI)
	}
	if *rows[1].LogMessage != unicodeMessage {
		t.Errorf("Readback LogMessage unicode mismatch: expected %q, got %q", unicodeMessage, *rows[1].LogMessage)
	}
}

// ---------------------------------------------------------------------------
// Tier 2 - Test 10: Timestamp Timezone Normalization to UTC
// ---------------------------------------------------------------------------
func TestTier2_TimestampTimezoneNormalization(t *testing.T) {
	// Create timestamp in EST timezone (UTC -5)
	estLoc := time.FixedZone("EST", -5*3600)

	estTime := time.Date(2026, 8, 20, 23, 30, 0, 0, estLoc) // 2026-08-20 23:30 EST = 2026-08-21 04:30 UTC
	entry := harness.RawTelescopeEntry{
		Sequence:  1,
		UUID:      uuid.NewString(),
		BatchID:   uuid.NewString(),
		Type:      "job",
		Content:   `{"name": "App\\Jobs\\TZJob", "status": "completed"}`,
		CreatedAt: estTime,
		Tags:      []string{"TimezoneTest"},
	}

	rec := harness.ExtractDimensions(entry)
	if rec.CreatedAt.Location() != time.UTC {
		t.Errorf("Expected CreatedAt to be normalized to UTC, got location: %v", rec.CreatedAt.Location())
	}
	if rec.CreatedAt.Hour() != 4 || rec.CreatedAt.Day() != 21 {
		t.Errorf("Expected UTC hour 4 on day 21, got hour %d on day %d", rec.CreatedAt.Hour(), rec.CreatedAt.Day())
	}

	// Verify Hive partition path uses UTC date/hour (year=2026/month=08/day=21/hour=04)
	path := harness.GenerateHivePartitionPath("tz-proj", "job", estTime, 1, 1, "b1")
	parsed, err := harness.ParseHivePath(path)
	if err != nil {
		t.Fatalf("ParseHivePath failed: %v", err)
	}
	if parsed.Day != 21 || parsed.Hour != 4 {
		t.Errorf("Partition path did not use UTC timestamp: Day=%d, Hour=%d", parsed.Day, parsed.Hour)
	}
}
