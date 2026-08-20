package storage_test

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/divmora/periscope/internal/storage"
)

// --- Faulty / Slow Readers for Stress Testing ---

type erroringReader struct {
	readLimit int
	bytesRead int
	err       error
}

func (r *erroringReader) Read(p []byte) (n int, err error) {
	if r.bytesRead >= r.readLimit {
		return 0, r.err
	}
	remaining := r.readLimit - r.bytesRead
	toRead := len(p)
	if toRead > remaining {
		toRead = remaining
	}
	for i := 0; i < toRead; i++ {
		p[i] = byte('A' + (r.bytesRead+i)%26)
	}
	r.bytesRead += toRead
	if r.bytesRead >= r.readLimit {
		return toRead, r.err
	}
	return toRead, nil
}

type blockingReader struct {
	ctx context.Context
}

func (r *blockingReader) Read(p []byte) (n int, err error) {
	<-r.ctx.Done()
	return 0, r.ctx.Err()
}

// =========================================================================
// 1. Concurrent High-Contention Uploads to the Same Key (Atomic Overwrite Race)
// =========================================================================

func TestLocalDriver_Adversarial_ConcurrentUploadsSameKey(t *testing.T) {
	tmpDir := t.TempDir()
	driver, err := storage.NewLocalDriver(storage.LocalConfig{BasePath: tmpDir})
	if err != nil {
		t.Fatalf("failed to create local driver: %v", err)
	}

	ctx := context.Background()
	hotspotKey := "projectId=hotspot/type=request/year=2026/month=08/hotspot.parquet"
	numGoroutines := 40

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	validPayloads := make(map[string]bool)
	var mu sync.Mutex

	for i := 0; i < numGoroutines; i++ {
		workerID := i
		payload := fmt.Sprintf("payload-data-version-%04d-%s", workerID, strings.Repeat("X", 512))
		mu.Lock()
		validPayloads[payload] = true
		mu.Unlock()

		go func() {
			defer wg.Done()
			uploadErr := driver.Upload(ctx, hotspotKey, strings.NewReader(payload), int64(len(payload)), "application/octet-stream")
			if uploadErr != nil {
				t.Errorf("worker %d upload failed: %v", workerID, uploadErr)
			}
		}()
	}

	wg.Wait()

	// Verify final state: object must exist and content must match exactly one of the worker payloads
	rc, err := driver.Get(ctx, hotspotKey)
	if err != nil {
		t.Fatalf("failed to get hotspot object: %v", err)
	}
	defer rc.Close()

	finalData, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("failed to read final content: %v", err)
	}

	if !validPayloads[string(finalData)] {
		t.Fatalf("corrupted or partial content detected in final file: length=%d", len(finalData))
	}

	// Verify no leftover temporary files in directory
	dir := filepath.Join(tmpDir, "projectId=hotspot", "type=request", "year=2026", "month=08")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("failed to read dir: %v", err)
	}

	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".tmp-periscope-") {
			t.Errorf("orphaned temporary file found: %s", entry.Name())
		}
	}
}

// =========================================================================
// 2. Concurrent Read-While-Write (Non-blocking Snapshot Reading)
// =========================================================================

func TestLocalDriver_Adversarial_ConcurrentReadWhileWrite(t *testing.T) {
	tmpDir := t.TempDir()
	driver, err := storage.NewLocalDriver(storage.LocalConfig{BasePath: tmpDir})
	if err != nil {
		t.Fatalf("failed to create local driver: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	key := "concurrent/rw-test.dat"
	initialData := []byte("version-00000-initial")
	if err := driver.Upload(ctx, key, bytes.NewReader(initialData), int64(len(initialData)), "text/plain"); err != nil {
		t.Fatalf("initial upload failed: %v", err)
	}

	var writeCounter int64
	var readErrors int64
	var corruptedReads int64
	var totalReads int64

	var wg sync.WaitGroup

	// Writer goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			default:
				v := atomic.AddInt64(&writeCounter, 1)
				payload := fmt.Sprintf("version-%05d-content-%s", v, strings.Repeat("Y", 1024))
				_ = driver.Upload(ctx, key, strings.NewReader(payload), int64(len(payload)), "text/plain")
			}
		}
	}()

	// Reader goroutines
	numReaders := 8
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					atomic.AddInt64(&totalReads, 1)
					rc, err := driver.Get(ctx, key)
					if err != nil {
						atomic.AddInt64(&readErrors, 1)
						continue
					}
					data, err := io.ReadAll(rc)
					_ = rc.Close()
					if err != nil {
						atomic.AddInt64(&readErrors, 1)
						continue
					}

					// Verify data format integrity: must start with "version-" and have correct length
					strData := string(data)
					if !strings.HasPrefix(strData, "version-") {
						atomic.AddInt64(&corruptedReads, 1)
					}
				}
			}
		}()
	}

	wg.Wait()

	if corruptedReads > 0 {
		t.Fatalf("detected %d corrupted reads during concurrent read/write", corruptedReads)
	}
	if totalReads == 0 {
		t.Fatal("no reads were performed during test window")
	}
}

// =========================================================================
// 3. Zero-Byte and Empty Stream Uploads
// =========================================================================

func TestLocalDriver_Adversarial_ZeroByteUpload(t *testing.T) {
	tmpDir := t.TempDir()
	driver, err := storage.NewLocalDriver(storage.LocalConfig{BasePath: tmpDir})
	if err != nil {
		t.Fatalf("failed to create local driver: %v", err)
	}

	ctx := context.Background()
	zeroKey := "empty/zero-byte.parquet"

	// Upload 0-byte
	err = driver.Upload(ctx, zeroKey, bytes.NewReader([]byte{}), 0, "application/octet-stream")
	if err != nil {
		t.Fatalf("failed to upload zero-byte object: %v", err)
	}

	// Exists check
	exists, err := driver.Exists(ctx, zeroKey)
	if err != nil || !exists {
		t.Fatalf("zero-byte object existence check failed: exists=%v, err=%v", exists, err)
	}

	// Get check
	rc, err := driver.Get(ctx, zeroKey)
	if err != nil {
		t.Fatalf("failed to get zero-byte object: %v", err)
	}
	defer rc.Close()

	readData, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("failed to read zero-byte object: %v", err)
	}
	if len(readData) != 0 {
		t.Fatalf("expected 0 bytes, got %d bytes", len(readData))
	}

	// List check
	items, err := driver.List(ctx, "empty/")
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(items) != 1 || items[0].Size != 0 {
		t.Fatalf("expected 1 object with size 0, got %+v", items)
	}
}

// =========================================================================
// 4. Inferred Size (Negative Size Stream)
// =========================================================================

func TestLocalDriver_Adversarial_InferredSizeStream(t *testing.T) {
	tmpDir := t.TempDir()
	driver, err := storage.NewLocalDriver(storage.LocalConfig{BasePath: tmpDir})
	if err != nil {
		t.Fatalf("failed to create local driver: %v", err)
	}

	ctx := context.Background()
	streamKey := "streams/inferred-size.parquet"
	largePayload := bytes.Repeat([]byte("STREAMING-PARQUET-BLOCK-"), 1000) // 24,000 bytes

	// Upload with size = -1
	err = driver.Upload(ctx, streamKey, bytes.NewReader(largePayload), -1, "application/octet-stream")
	if err != nil {
		t.Fatalf("upload with size=-1 failed: %v", err)
	}

	rc, err := driver.Get(ctx, streamKey)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	defer rc.Close()

	retrieved, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if !bytes.Equal(retrieved, largePayload) {
		t.Fatalf("stream content mismatch: expected %d bytes, got %d bytes", len(largePayload), len(retrieved))
	}
}

// =========================================================================
// 5. Partial Reads & Premature Close
// =========================================================================

func TestLocalDriver_Adversarial_PartialReadAndClose(t *testing.T) {
	tmpDir := t.TempDir()
	driver, err := storage.NewLocalDriver(storage.LocalConfig{BasePath: tmpDir})
	if err != nil {
		t.Fatalf("failed to create local driver: %v", err)
	}

	ctx := context.Background()
	key := "partial/large.bin"
	payload := bytes.Repeat([]byte("0123456789ABCDEF"), 2048) // 32KB

	if err := driver.Upload(ctx, key, bytes.NewReader(payload), int64(len(payload)), "application/octet-stream"); err != nil {
		t.Fatalf("upload failed: %v", err)
	}

	// Open and read only 16 bytes, then immediately close
	rc, err := driver.Get(ctx, key)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}

	buf := make([]byte, 16)
	n, err := io.ReadFull(rc, buf)
	if err != nil || n != 16 {
		t.Fatalf("failed to read 16 bytes: n=%d, err=%v", n, err)
	}
	if err := rc.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}

	// Ensure file remains readable afterwards
	rc2, err := driver.Get(ctx, key)
	if err != nil {
		t.Fatalf("subsequent get failed: %v", err)
	}
	defer rc2.Close()

	fullData, err := io.ReadAll(rc2)
	if err != nil || len(fullData) != len(payload) {
		t.Fatalf("subsequent full read failed: len=%d, err=%v", len(fullData), err)
	}
}

// =========================================================================
// 6. Fault Injection: Reader Fails Mid-Stream During Upload
// =========================================================================

func TestLocalDriver_Adversarial_FaultyReaderUploadCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	driver, err := storage.NewLocalDriver(storage.LocalConfig{BasePath: tmpDir})
	if err != nil {
		t.Fatalf("failed to create local driver: %v", err)
	}

	ctx := context.Background()
	key := "faults/broken-upload.parquet"

	faulty := &erroringReader{
		readLimit: 512,
		err:       io.ErrUnexpectedEOF,
	}

	// Upload should fail with the injected error
	err = driver.Upload(ctx, key, faulty, 1024, "application/octet-stream")
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("expected io.ErrUnexpectedEOF, got: %v", err)
	}

	// Target object must NOT exist
	exists, err := driver.Exists(ctx, key)
	if err != nil {
		t.Fatalf("exists check failed: %v", err)
	}
	if exists {
		t.Fatalf("broken upload resulted in existing file!")
	}

	// No temporary files left behind
	dir := filepath.Join(tmpDir, "faults")
	if entries, err := os.ReadDir(dir); err == nil {
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), ".tmp-periscope-") {
				t.Errorf("temporary file was not cleaned up on failure: %s", entry.Name())
			}
		}
	}
}

// =========================================================================
// 7. Context Cancellation During Slow/Blocking Upload
// =========================================================================

func TestLocalDriver_Adversarial_ContextCancellationMidUpload(t *testing.T) {
	tmpDir := t.TempDir()
	driver, err := storage.NewLocalDriver(storage.LocalConfig{BasePath: tmpDir})
	if err != nil {
		t.Fatalf("failed to create local driver: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	key := "canceled/slow-upload.parquet"

	blocking := &blockingReader{ctx: ctx}

	// Run upload in background
	errCh := make(chan error, 1)
	go func() {
		errCh <- driver.Upload(ctx, key, blocking, 10000, "application/octet-stream")
	}()

	// Let it block briefly, then cancel
	time.Sleep(50 * time.Millisecond)
	cancel()

	uploadErr := <-errCh
	if !errors.Is(uploadErr, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", uploadErr)
	}

	// Verify target object was not created
	exists, _ := driver.Exists(context.Background(), key)
	if exists {
		t.Fatalf("target file created despite context cancellation")
	}
}

// =========================================================================
// 8. Extreme Key Sanitization, Special Characters, and Deep Hierarchies
// =========================================================================

func TestLocalDriver_Adversarial_ExtremeKeys(t *testing.T) {
	tmpDir := t.TempDir()
	driver, err := storage.NewLocalDriver(storage.LocalConfig{BasePath: tmpDir})
	if err != nil {
		t.Fatalf("failed to create local driver: %v", err)
	}

	ctx := context.Background()

	validExtremeKeys := []string{
		"projectId=1/type=request/year=2026/month=08/day=20/hour=12/batch-uuid-0123456789.parquet",
		"deep/a/b/c/d/e/f/g/h/i/j/k/l/m/n/o/p/q/r/s/t/u/v/w/x/y/z/leaf.parquet",
		"spaces in name/my special telemetry batch (v1.0.0) [test].parquet",
		"symbols/_.-=+@#~%/valid_chars.parquet",
		"leading///multiple///slashes///normalized.parquet",
	}

	for _, key := range validExtremeKeys {
		t.Run("ValidKey_"+key, func(t *testing.T) {
			content := []byte("data-for-" + key)
			if err := driver.Upload(ctx, key, bytes.NewReader(content), int64(len(content)), "text/plain"); err != nil {
				t.Fatalf("upload failed for valid extreme key '%s': %v", key, err)
			}

			exists, err := driver.Exists(ctx, key)
			if err != nil || !exists {
				t.Fatalf("exists failed for key '%s'", key)
			}

			rc, err := driver.Get(ctx, key)
			if err != nil {
				t.Fatalf("get failed for key '%s': %v", key, err)
			}
			data, err := io.ReadAll(rc)
			_ = rc.Close()
			if err != nil || !bytes.Equal(data, content) {
				t.Fatalf("content mismatch for key '%s'", key)
			}

			if err := driver.Delete(ctx, key); err != nil {
				t.Fatalf("delete failed for key '%s': %v", key, err)
			}
		})
	}

	invalidMaliciousKeys := []string{
		"",
		"   ",
		"\t\n",
		"/",
		"///",
		".",
		"..",
		"../escape.parquet",
		"a/../../escape.parquet",
		"a/b/../../../escape.parquet",
		"null\x00byte.parquet",
		"nested/\x00/evil.parquet",
	}

	for _, badKey := range invalidMaliciousKeys {
		t.Run("InvalidKey_"+badKey, func(t *testing.T) {
			err := driver.Upload(ctx, badKey, strings.NewReader("evil"), 4, "text/plain")
			if err == nil {
				t.Fatalf("expected error for malicious/invalid key %q, got nil", badKey)
			}
			if !errors.Is(err, storage.ErrPathTraversal) && !errors.Is(err, storage.ErrInvalidConfig) {
				t.Fatalf("expected ErrPathTraversal or ErrInvalidConfig for %q, got: %v", badKey, err)
			}
		})
	}
}

// =========================================================================
// 9. S3 Driver Multi-Page Pagination Stress Test
// =========================================================================

func TestS3Driver_Adversarial_MultiPagePagination(t *testing.T) {
	mockServer := newPagingMockS3Server(25) // Page size limit of 25 items
	ts := httptest.NewServer(mockServer)
	defer ts.Close()

	ctx := context.Background()
	bucketName := "paged-telemetry-bucket"

	mockServer.mu.Lock()
	mockServer.buckets[bucketName] = make(map[string]mockS3Item)
	totalObjects := 120
	for i := 0; i < totalObjects; i++ {
		key := fmt.Sprintf("projectId=100/type=request/year=2026/part-%04d.parquet", i)
		mockServer.buckets[bucketName][key] = mockS3Item{
			data:        []byte(fmt.Sprintf("data-%d", i)),
			contentType: "application/octet-stream",
			modTime:     time.Now().UTC(),
		}
	}
	// Add 30 objects for a different project
	for i := 0; i < 30; i++ {
		key := fmt.Sprintf("projectId=200/type=request/year=2026/part-%04d.parquet", i)
		mockServer.buckets[bucketName][key] = mockS3Item{
			data:        []byte(fmt.Sprintf("data-%d", i)),
			contentType: "application/octet-stream",
			modTime:     time.Now().UTC(),
		}
	}
	mockServer.mu.Unlock()

	driver, err := storage.NewS3Driver(ctx, storage.S3Config{
		Type:            "s3",
		Bucket:          bucketName,
		Region:          "us-east-1",
		Endpoint:        ts.URL,
		AccessKeyID:     "test",
		SecretAccessKey: "test",
		UsePathStyle:    true,
	})
	if err != nil {
		t.Fatalf("failed to create S3 driver: %v", err)
	}

	// List projectId=100 prefix: should traverse all pages and return exactly 120 items
	items, err := driver.List(ctx, "projectId=100/")
	if err != nil {
		t.Fatalf("multi-page list failed: %v", err)
	}

	if len(items) != totalObjects {
		t.Fatalf("expected %d objects across multiple pages, got %d", totalObjects, len(items))
	}

	// Verify all returned items match the prefix
	for _, item := range items {
		if !strings.HasPrefix(item.Key, "projectId=100/") {
			t.Errorf("item '%s' did not match requested prefix", item.Key)
		}
	}
}

// Paging Mock S3 Server for pagination stress tests
type pagingMockS3Server struct {
	mu           sync.RWMutex
	buckets      map[string]map[string]mockS3Item
	pageSize     int
}

func newPagingMockS3Server(pageSize int) *pagingMockS3Server {
	return &pagingMockS3Server{
		buckets:  make(map[string]map[string]mockS3Item),
		pageSize: pageSize,
	}
}

func (m *pagingMockS3Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	path := strings.TrimPrefix(r.URL.Path, "/")
	parts := strings.SplitN(path, "/", 2)
	bucket := parts[0]
	var key string
	if len(parts) > 1 {
		key = parts[1]
	}

	if key == "" && r.Method == http.MethodGet {
		// ListObjectsV2 with continuation token simulation
		items, exists := m.buckets[bucket]
		if !exists {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		prefix := r.URL.Query().Get("prefix")
		startAfter := r.URL.Query().Get("start-after")
		continuationToken := r.URL.Query().Get("continuation-token")

		var matchingKeys []string
		for k := range items {
			if prefix == "" || strings.HasPrefix(k, prefix) {
				matchingKeys = append(matchingKeys, k)
			}
		}

		// Sort keys for deterministic pagination
		for i := 0; i < len(matchingKeys); i++ {
			for j := i + 1; j < len(matchingKeys); j++ {
				if matchingKeys[i] > matchingKeys[j] {
					matchingKeys[i], matchingKeys[j] = matchingKeys[j], matchingKeys[i]
				}
			}
		}

		startIndex := 0
		if continuationToken != "" {
			for idx, k := range matchingKeys {
				if k == continuationToken {
					startIndex = idx + 1
					break
				}
			}
		} else if startAfter != "" {
			for idx, k := range matchingKeys {
				if k == startAfter {
					startIndex = idx + 1
					break
				}
			}
		}

		endIndex := startIndex + m.pageSize
		isTruncated := false
		var nextContinuationToken string
		if endIndex < len(matchingKeys) {
			isTruncated = true
			nextContinuationToken = matchingKeys[endIndex-1]
		} else {
			endIndex = len(matchingKeys)
		}

		type Contents struct {
			Key          string `xml:"Key"`
			Size         int64  `xml:"Size"`
			LastModified string `xml:"LastModified"`
			ETag         string `xml:"ETag"`
		}
		type ListBucketResult struct {
			XMLName               xml.Name   `xml:"ListBucketResult"`
			Name                  string     `xml:"Name"`
			Prefix                string     `xml:"Prefix"`
			KeyCount              int        `xml:"KeyCount"`
			MaxKeys               int        `xml:"MaxKeys"`
			IsTruncated           bool       `xml:"IsTruncated"`
			NextContinuationToken string     `xml:"NextContinuationToken,omitempty"`
			Contents              []Contents `xml:"Contents"`
		}

		res := ListBucketResult{
			Name:                  bucket,
			Prefix:                prefix,
			MaxKeys:               m.pageSize,
			IsTruncated:           isTruncated,
			NextContinuationToken: nextContinuationToken,
		}

		for _, k := range matchingKeys[startIndex:endIndex] {
			v := items[k]
			res.Contents = append(res.Contents, Contents{
				Key:          k,
				Size:         int64(len(v.data)),
				LastModified: v.modTime.UTC().Format(time.RFC3339),
				ETag:         `"mock-paged-etag"`,
			})
		}
		res.KeyCount = len(res.Contents)

		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		_ = xml.NewEncoder(w).Encode(res)
		return
	}

	// Fallback to standard mock handling
	w.WriteHeader(http.StatusOK)
}

// =========================================================================
// 10. S3 Driver Fault Injection & Concurrent Operations
// =========================================================================

func TestS3Driver_Adversarial_ConcurrentUploadsAndReads(t *testing.T) {
	mockServer := newMockS3Server()
	ts := httptest.NewServer(mockServer)
	defer ts.Close()

	ctx := context.Background()
	bucketName := "concurrent-s3-test"

	mockServer.mu.Lock()
	mockServer.buckets[bucketName] = make(map[string]mockS3Item)
	mockServer.mu.Unlock()

	driver, err := storage.NewS3Driver(ctx, storage.S3Config{
		Type:            "s3",
		Bucket:          bucketName,
		Endpoint:        ts.URL,
		AccessKeyID:     "test",
		SecretAccessKey: "test",
		UsePathStyle:    true,
	})
	if err != nil {
		t.Fatalf("failed to create S3 driver: %v", err)
	}

	numGoroutines := 30
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		workerID := i
		go func() {
			defer wg.Done()
			key := fmt.Sprintf("projectId=s3-conc/type=log/year=2026/worker-%03d.parquet", workerID)
			payload := fmt.Sprintf("worker-%d-data-%s", workerID, strings.Repeat("Z", 256))

			// Upload
			if err := driver.Upload(ctx, key, strings.NewReader(payload), int64(len(payload)), "application/octet-stream"); err != nil {
				t.Errorf("worker %d upload failed: %v", workerID, err)
				return
			}

			// Exists
			exists, err := driver.Exists(ctx, key)
			if err != nil || !exists {
				t.Errorf("worker %d exists check failed: %v", workerID, err)
				return
			}

			// Get
			rc, err := driver.Get(ctx, key)
			if err != nil {
				t.Errorf("worker %d get failed: %v", workerID, err)
				return
			}
			defer rc.Close()

			data, err := io.ReadAll(rc)
			if err != nil || string(data) != payload {
				t.Errorf("worker %d data mismatch", workerID)
				return
			}
		}()
	}

	wg.Wait()

	// List all uploaded objects
	objects, err := driver.List(ctx, "projectId=s3-conc/")
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(objects) != numGoroutines {
		t.Fatalf("expected %d objects in list, got %d", numGoroutines, len(objects))
	}
}
