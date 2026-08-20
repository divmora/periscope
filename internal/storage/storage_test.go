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
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/divmora/periscope/internal/storage"
)

// --- Local Driver Tests ---

func TestLocalDriver_UploadAndGet(t *testing.T) {
	tmpDir := t.TempDir()
	driver, err := storage.NewLocalDriver(storage.LocalConfig{BasePath: tmpDir})
	if err != nil {
		t.Fatalf("failed to create local driver: %v", err)
	}

	if driver.Type() != "local" {
		t.Errorf("expected type 'local', got '%s'", driver.Type())
	}

	ctx := context.Background()
	key := "projectId=test-proj/type=request/year=2026/month=08/day=20/hour=12/part-001.parquet"
	content := []byte("parquet-binary-payload-data-test-bytes")

	// Upload
	err = driver.Upload(ctx, key, bytes.NewReader(content), int64(len(content)), "application/octet-stream")
	if err != nil {
		t.Fatalf("unexpected upload error: %v", err)
	}

	// Exists
	exists, err := driver.Exists(ctx, key)
	if err != nil {
		t.Fatalf("unexpected exists error: %v", err)
	}
	if !exists {
		t.Fatalf("expected object to exist at key '%s'", key)
	}

	// Get
	rc, err := driver.Get(ctx, key)
	if err != nil {
		t.Fatalf("unexpected get error: %v", err)
	}
	defer rc.Close()

	readBytes, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("failed to read object body: %v", err)
	}
	if !bytes.Equal(readBytes, content) {
		t.Errorf("expected content %q, got %q", string(content), string(readBytes))
	}
}

func TestLocalDriver_AtomicReplacement(t *testing.T) {
	tmpDir := t.TempDir()
	driver, err := storage.NewLocalDriver(storage.LocalConfig{BasePath: tmpDir})
	if err != nil {
		t.Fatalf("failed to create local driver: %v", err)
	}

	ctx := context.Background()
	key := "batches/batch-1.dat"

	// Write initial content
	initialContent := []byte("v1-initial-content")
	if err := driver.Upload(ctx, key, bytes.NewReader(initialContent), int64(len(initialContent)), "text/plain"); err != nil {
		t.Fatalf("initial upload failed: %v", err)
	}

	// Overwrite atomically
	updatedContent := []byte("v2-updated-atomic-content")
	if err := driver.Upload(ctx, key, bytes.NewReader(updatedContent), int64(len(updatedContent)), "text/plain"); err != nil {
		t.Fatalf("overwrite upload failed: %v", err)
	}

	// Verify updated content
	rc, err := driver.Get(ctx, key)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if !bytes.Equal(data, updatedContent) {
		t.Errorf("expected %q, got %q", string(updatedContent), string(data))
	}
}

func TestLocalDriver_ExistsAndNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	driver, err := storage.NewLocalDriver(storage.LocalConfig{BasePath: tmpDir})
	if err != nil {
		t.Fatalf("failed to create local driver: %v", err)
	}

	ctx := context.Background()

	// Non-existent object
	exists, err := driver.Exists(ctx, "non/existent/key.parquet")
	if err != nil {
		t.Fatalf("unexpected exists error: %v", err)
	}
	if exists {
		t.Fatalf("expected non-existent object to return false")
	}

	// Get non-existent object
	_, err = driver.Get(ctx, "non/existent/key.parquet")
	if !errors.Is(err, storage.ErrObjectNotFound) {
		t.Fatalf("expected ErrObjectNotFound, got: %v", err)
	}
}

func TestLocalDriver_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	driver, err := storage.NewLocalDriver(storage.LocalConfig{BasePath: tmpDir})
	if err != nil {
		t.Fatalf("failed to create local driver: %v", err)
	}

	ctx := context.Background()
	key := "delete-me/file.txt"

	// Upload
	if err := driver.Upload(ctx, key, strings.NewReader("sample"), 6, "text/plain"); err != nil {
		t.Fatalf("upload failed: %v", err)
	}

	// Delete
	if err := driver.Delete(ctx, key); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	// Verify not found
	exists, err := driver.Exists(ctx, key)
	if err != nil {
		t.Fatalf("exists check failed: %v", err)
	}
	if exists {
		t.Fatalf("expected object to be deleted")
	}

	// Delete non-existent object should be idempotent and return nil
	if err := driver.Delete(ctx, key); err != nil {
		t.Fatalf("subsequent delete should return nil, got: %v", err)
	}
}

func TestLocalDriver_List(t *testing.T) {
	tmpDir := t.TempDir()
	driver, err := storage.NewLocalDriver(storage.LocalConfig{BasePath: tmpDir})
	if err != nil {
		t.Fatalf("failed to create local driver: %v", err)
	}

	ctx := context.Background()

	keys := []string{
		"projectId=1/type=request/year=2026/part-1.parquet",
		"projectId=1/type=request/year=2026/part-2.parquet",
		"projectId=1/type=query/year=2026/part-1.parquet",
		"projectId=2/type=request/year=2026/part-1.parquet",
	}

	for _, k := range keys {
		if err := driver.Upload(ctx, k, strings.NewReader("data"), 4, "application/octet-stream"); err != nil {
			t.Fatalf("upload '%s' failed: %v", k, err)
		}
	}

	// List all
	all, err := driver.List(ctx, "")
	if err != nil {
		t.Fatalf("list all failed: %v", err)
	}
	if len(all) != len(keys) {
		t.Errorf("expected %d objects, got %d", len(keys), len(all))
	}

	// List with prefix
	reqObjects, err := driver.List(ctx, "projectId=1/type=request/")
	if err != nil {
		t.Fatalf("list with prefix failed: %v", err)
	}
	if len(reqObjects) != 2 {
		t.Errorf("expected 2 request objects, got %d", len(reqObjects))
	}
}

func TestLocalDriver_PathTraversalProtection(t *testing.T) {
	tmpDir := t.TempDir()
	driver, err := storage.NewLocalDriver(storage.LocalConfig{BasePath: tmpDir})
	if err != nil {
		t.Fatalf("failed to create local driver: %v", err)
	}

	ctx := context.Background()
	payload := []byte("exploit-payload")

	attackKeys := []string{
		"../etc/passwd",
		"../../secret.key",
		"subdir/../../outside.txt",
		"subdir/../../../root.txt",
		"foo/bar/../../../../etc/shadow",
		"\x00nullbyte",
		"normal/path/\x00evil",
	}

	for _, attackKey := range attackKeys {
		t.Run("Upload_"+attackKey, func(t *testing.T) {
			err := driver.Upload(ctx, attackKey, bytes.NewReader(payload), int64(len(payload)), "text/plain")
			if !errors.Is(err, storage.ErrPathTraversal) && !errors.Is(err, storage.ErrInvalidConfig) {
				t.Fatalf("expected ErrPathTraversal or ErrInvalidConfig for key '%s', got: %v", attackKey, err)
			}
		})

		t.Run("Get_"+attackKey, func(t *testing.T) {
			_, err := driver.Get(ctx, attackKey)
			if !errors.Is(err, storage.ErrPathTraversal) && !errors.Is(err, storage.ErrInvalidConfig) {
				t.Fatalf("expected ErrPathTraversal or ErrInvalidConfig for key '%s', got: %v", attackKey, err)
			}
		})

		t.Run("Exists_"+attackKey, func(t *testing.T) {
			_, err := driver.Exists(ctx, attackKey)
			if !errors.Is(err, storage.ErrPathTraversal) && !errors.Is(err, storage.ErrInvalidConfig) {
				t.Fatalf("expected ErrPathTraversal or ErrInvalidConfig for key '%s', got: %v", attackKey, err)
			}
		})

		t.Run("Delete_"+attackKey, func(t *testing.T) {
			err := driver.Delete(ctx, attackKey)
			if !errors.Is(err, storage.ErrPathTraversal) && !errors.Is(err, storage.ErrInvalidConfig) {
				t.Fatalf("expected ErrPathTraversal or ErrInvalidConfig for key '%s', got: %v", attackKey, err)
			}
		})
	}
}

func TestLocalDriver_InvalidConfig(t *testing.T) {
	_, err := storage.NewLocalDriver(storage.LocalConfig{BasePath: ""})
	if !errors.Is(err, storage.ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig for empty BasePath, got: %v", err)
	}

	driver, _ := storage.NewLocalDriver(storage.LocalConfig{BasePath: t.TempDir()})
	if err := driver.Upload(context.Background(), "", strings.NewReader(""), 0, ""); !errors.Is(err, storage.ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig for empty key, got: %v", err)
	}
}

func TestLocalDriver_Ping(t *testing.T) {
	tmpDir := t.TempDir()
	driver, err := storage.NewLocalDriver(storage.LocalConfig{BasePath: tmpDir})
	if err != nil {
		t.Fatalf("failed to create local driver: %v", err)
	}

	if err := driver.Ping(context.Background()); err != nil {
		t.Fatalf("ping failed on valid directory: %v", err)
	}
}

func TestLocalDriver_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	driver, err := storage.NewLocalDriver(storage.LocalConfig{BasePath: tmpDir})
	if err != nil {
		t.Fatalf("failed to create local driver: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err = driver.Upload(ctx, "file.txt", strings.NewReader("data"), 4, "text/plain")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}

	_, err = driver.Get(ctx, "file.txt")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}

	_, err = driver.Exists(ctx, "file.txt")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}

	err = driver.Delete(ctx, "file.txt")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}

	_, err = driver.List(ctx, "")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}

	err = driver.Ping(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}
}

// --- Mock S3 HTTP Server & S3 Driver Tests ---

type mockS3Item struct {
	data        []byte
	contentType string
	modTime     time.Time
}

type mockS3Server struct {
	mu           sync.RWMutex
	buckets      map[string]map[string]mockS3Item
	deniedBucket string
}

func newMockS3Server() *mockS3Server {
	return &mockS3Server{
		buckets:      make(map[string]map[string]mockS3Item),
		deniedBucket: "forbidden-bucket",
	}
}

func (m *mockS3Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	path := strings.TrimPrefix(r.URL.Path, "/")
	parts := strings.SplitN(path, "/", 2)
	bucket := parts[0]
	var key string
	if len(parts) > 1 {
		key = parts[1]
	}

	if bucket == m.deniedBucket {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>AccessDenied</Code><Message>Access Denied</Message></Error>`))
		return
	}

	// HeadBucket / ListObjects
	if key == "" {
		switch r.Method {
		case http.MethodHead:
			if _, exists := m.buckets[bucket]; !exists {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.WriteHeader(http.StatusOK)
			return

		case http.MethodGet:
			// ListObjectsV2
			items, exists := m.buckets[bucket]
			if !exists {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			prefix := r.URL.Query().Get("prefix")
			type Contents struct {
				Key          string `xml:"Key"`
				Size         int64  `xml:"Size"`
				LastModified string `xml:"LastModified"`
				ETag         string `xml:"ETag"`
			}
			type ListBucketResult struct {
				XMLName     xml.Name   `xml:"ListBucketResult"`
				Name        string     `xml:"Name"`
				Prefix      string     `xml:"Prefix"`
				KeyCount    int        `xml:"KeyCount"`
				MaxKeys     int        `xml:"MaxKeys"`
				IsTruncated bool       `xml:"IsTruncated"`
				Contents    []Contents `xml:"Contents"`
			}

			res := ListBucketResult{
				Name:    bucket,
				Prefix:  prefix,
				MaxKeys: 1000,
			}

			for k, v := range items {
				if prefix == "" || strings.HasPrefix(k, prefix) {
					res.Contents = append(res.Contents, Contents{
						Key:          k,
						Size:         int64(len(v.data)),
						LastModified: v.modTime.UTC().Format(time.RFC3339),
						ETag:         `"mock-etag"`,
					})
				}
			}
			res.KeyCount = len(res.Contents)

			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusOK)
			_ = xml.NewEncoder(w).Encode(res)
			return

		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
	}

	// Object operations
	items, bucketExists := m.buckets[bucket]
	if !bucketExists && r.Method != http.MethodPut {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>NoSuchBucket</Code><Message>The specified bucket does not exist</Message></Error>`))
		return
	}

	switch r.Method {
	case http.MethodPut:
		if !bucketExists {
			m.buckets[bucket] = make(map[string]mockS3Item)
			items = m.buckets[bucket]
		}
		data, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		items[key] = mockS3Item{
			data:        data,
			contentType: r.Header.Get("Content-Type"),
			modTime:     time.Now().UTC(),
		}
		w.Header().Set("ETag", `"mock-etag"`)
		w.WriteHeader(http.StatusOK)

	case http.MethodGet:
		item, exists := items[key]
		if !exists {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>NoSuchKey</Code><Message>The specified key does not exist.</Message></Error>`))
			return
		}
		w.Header().Set("Content-Type", item.contentType)
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(item.data)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(item.data)

	case http.MethodHead:
		item, exists := items[key]
		if !exists {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", item.contentType)
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(item.data)))
		w.WriteHeader(http.StatusOK)

	case http.MethodDelete:
		delete(items, key)
		w.WriteHeader(http.StatusNoContent)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func TestS3Driver_EndToEndWithMockServer(t *testing.T) {
	mockServer := newMockS3Server()
	ts := httptest.NewServer(mockServer)
	defer ts.Close()

	ctx := context.Background()
	bucketName := "telemetry-bucket"

	// Create bucket in mock
	mockServer.mu.Lock()
	mockServer.buckets[bucketName] = make(map[string]mockS3Item)
	mockServer.mu.Unlock()

	driver, err := storage.NewS3Driver(ctx, storage.S3Config{
		Type:            "s3",
		Bucket:          bucketName,
		Region:          "us-east-1",
		Endpoint:        ts.URL,
		AccessKeyID:     "minioadmin",
		SecretAccessKey: "minioadmin",
		UsePathStyle:    true,
	})
	if err != nil {
		t.Fatalf("failed to create S3 driver: %v", err)
	}

	if driver.Type() != "s3" {
		t.Errorf("expected type 's3', got '%s'", driver.Type())
	}

	// 1. Ping
	if err := driver.Ping(ctx); err != nil {
		t.Fatalf("ping failed: %v", err)
	}

	// 2. Upload
	key := "projectId=p1/type=query/year=2026/month=08/day=20/part.parquet"
	payload := []byte("parquet-test-content-for-s3")
	err = driver.Upload(ctx, key, bytes.NewReader(payload), int64(len(payload)), "application/octet-stream")
	if err != nil {
		t.Fatalf("upload failed: %v", err)
	}

	// 3. Exists
	exists, err := driver.Exists(ctx, key)
	if err != nil {
		t.Fatalf("exists check failed: %v", err)
	}
	if !exists {
		t.Fatalf("expected key to exist")
	}

	// 4. Get
	rc, err := driver.Get(ctx, key)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if !bytes.Equal(data, payload) {
		t.Errorf("expected %q, got %q", string(payload), string(data))
	}

	// 5. List
	objects, err := driver.List(ctx, "projectId=p1/")
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(objects) != 1 {
		t.Fatalf("expected 1 object, got %d", len(objects))
	}
	if objects[0].Key != key {
		t.Errorf("expected key '%s', got '%s'", key, objects[0].Key)
	}

	// 6. Delete
	if err := driver.Delete(ctx, key); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	// Verify not exists
	exists, err = driver.Exists(ctx, key)
	if err != nil {
		t.Fatalf("exists check failed: %v", err)
	}
	if exists {
		t.Fatalf("expected object to be deleted")
	}

	// Get deleted object returns ErrObjectNotFound
	_, err = driver.Get(ctx, key)
	if !errors.Is(err, storage.ErrObjectNotFound) {
		t.Fatalf("expected ErrObjectNotFound, got: %v", err)
	}
}

func TestS3Driver_PingErrors(t *testing.T) {
	mockServer := newMockS3Server()
	ts := httptest.NewServer(mockServer)
	defer ts.Close()

	ctx := context.Background()

	// 1. Missing bucket -> ErrBucketNotFound
	driverMissing, err := storage.NewS3Driver(ctx, storage.S3Config{
		Bucket:          "missing-bucket",
		Endpoint:        ts.URL,
		AccessKeyID:     "test",
		SecretAccessKey: "test",
		UsePathStyle:    true,
	})
	if err != nil {
		t.Fatalf("failed to create driver: %v", err)
	}
	err = driverMissing.Ping(ctx)
	if !errors.Is(err, storage.ErrBucketNotFound) && !errors.Is(err, storage.ErrObjectNotFound) {
		t.Fatalf("expected ErrBucketNotFound or ErrObjectNotFound for missing bucket, got: %v", err)
	}

	// 2. Forbidden bucket -> ErrPermissionDenied
	driverForbidden, err := storage.NewS3Driver(ctx, storage.S3Config{
		Bucket:          "forbidden-bucket",
		Endpoint:        ts.URL,
		AccessKeyID:     "test",
		SecretAccessKey: "test",
		UsePathStyle:    true,
	})
	if err != nil {
		t.Fatalf("failed to create driver: %v", err)
	}
	err = driverForbidden.Ping(ctx)
	if !errors.Is(err, storage.ErrPermissionDenied) {
		t.Fatalf("expected ErrPermissionDenied for forbidden bucket, got: %v", err)
	}
}

func TestS3Driver_InvalidConfig(t *testing.T) {
	ctx := context.Background()
	_, err := storage.NewS3Driver(ctx, storage.S3Config{Bucket: ""})
	if !errors.Is(err, storage.ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig for empty bucket, got: %v", err)
	}
}

// --- Factory Tests ---

func TestFactory_DriverCreation(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	// 1. Local Driver
	localDriver, err := storage.NewDriverFromConfig(ctx, storage.Config{
		Type:     "local",
		BasePath: tmpDir,
	})
	if err != nil {
		t.Fatalf("factory local driver failed: %v", err)
	}
	if localDriver.Type() != "local" {
		t.Errorf("expected type 'local', got '%s'", localDriver.Type())
	}

	// 2. S3 Driver
	s3Driver, err := storage.NewDriverFromConfig(ctx, storage.Config{
		Type:   "s3",
		Bucket: "my-s3-bucket",
		Region: "us-west-2",
	})
	if err != nil {
		t.Fatalf("factory s3 driver failed: %v", err)
	}
	if s3Driver.Type() != "s3" {
		t.Errorf("expected type 's3', got '%s'", s3Driver.Type())
	}

	// 3. MinIO Driver
	minioDriver, err := storage.NewDriverFromConfig(ctx, storage.Config{
		Type:     "minio",
		Bucket:   "my-minio-bucket",
		Endpoint: "http://localhost:9000",
	})
	if err != nil {
		t.Fatalf("factory minio driver failed: %v", err)
	}
	if minioDriver.Type() != "minio" {
		t.Errorf("expected type 'minio', got '%s'", minioDriver.Type())
	}

	// 4. Cloudflare R2 Driver
	r2Driver, err := storage.NewDriverFromConfig(ctx, storage.Config{
		Type:     "r2",
		Bucket:   "my-r2-bucket",
		Endpoint: "https://account.r2.cloudflarestorage.com",
	})
	if err != nil {
		t.Fatalf("factory r2 driver failed: %v", err)
	}
	if r2Driver.Type() != "r2" {
		t.Errorf("expected type 'r2', got '%s'", r2Driver.Type())
	}

	// 5. Inferred Local
	inferredLocal, err := storage.NewDriverFromConfig(ctx, storage.Config{
		BasePath: tmpDir,
	})
	if err != nil {
		t.Fatalf("factory inferred local failed: %v", err)
	}
	if inferredLocal.Type() != "local" {
		t.Errorf("expected inferred 'local', got '%s'", inferredLocal.Type())
	}

	// 6. Inferred S3
	inferredS3, err := storage.NewDriverFromConfig(ctx, storage.Config{
		Bucket: "auto-s3-bucket",
	})
	if err != nil {
		t.Fatalf("factory inferred s3 failed: %v", err)
	}
	if inferredS3.Type() != "s3" {
		t.Errorf("expected inferred 's3', got '%s'", inferredS3.Type())
	}

	// 7. Unsupported Driver
	_, err = storage.NewDriverFromConfig(ctx, storage.Config{
		Type: "azure_blob",
	})
	if !errors.Is(err, storage.ErrUnsupportedDriver) {
		t.Fatalf("expected ErrUnsupportedDriver, got: %v", err)
	}

	// 8. Invalid Config (missing both base path and bucket)
	_, err = storage.NewDriverFromConfig(ctx, storage.Config{})
	if !errors.Is(err, storage.ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig for empty config, got: %v", err)
	}
}
