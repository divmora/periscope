package harness

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"time"
)

// S3MockServer provides an in-process, HTTP-compliant mock AWS S3 / MinIO server
// for end-to-end testing of storage drivers and collector upload pipelines.
type S3MockServer struct {
	server       *httptest.Server
	mu           sync.RWMutex
	buckets      map[string]map[string]*MockS3Object
	faulty       map[string]bool
	unauthorized map[string]bool
}

// MockS3Object stores object payload and metadata.
type MockS3Object struct {
	Key          string
	Data         []byte
	ContentType  string
	LastModified time.Time
	ETag         string
}

// ListBucketResultXML represents S3 ListObjectsV2 XML response payload.
type ListBucketResultXML struct {
	XMLName     xml.Name        `xml:"http://s3.amazonaws.com/doc/2006-03-01/ ListBucketResult"`
	Name        string          `xml:"Name"`
	Prefix      string          `xml:"Prefix"`
	KeyCount    int             `xml:"KeyCount"`
	MaxKeys     int             `xml:"MaxKeys"`
	IsTruncated bool            `xml:"IsTruncated"`
	Contents    []S3ObjectEntry `xml:"Contents"`
}

// S3ObjectEntry represents an object entry in ListObjectsV2 XML response.
type S3ObjectEntry struct {
	Key          string `xml:"Key"`
	LastModified string `xml:"LastModified"`
	ETag         string `xml:"ETag"`
	Size         int64  `xml:"Size"`
	StorageClass string `xml:"StorageClass"`
}

// S3ErrorXML represents standard S3 error payload.
type S3ErrorXML struct {
	XMLName xml.Name `xml:"Error"`
	Code    string   `xml:"Code"`
	Message string   `xml:"Message"`
	Key     string   `xml:"Key,omitempty"`
	Bucket  string   `xml:"BucketName,omitempty"`
}

// NewS3MockServer starts an httptest.Server that handles S3 protocol requests.
func NewS3MockServer() *S3MockServer {
	mock := &S3MockServer{
		buckets:      make(map[string]map[string]*MockS3Object),
		faulty:       make(map[string]bool),
		unauthorized: make(map[string]bool),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", mock.handleS3Request)

	mock.server = httptest.NewServer(mux)
	return mock
}

// URL returns the mock HTTP server's base URL (e.g. "http://127.0.0.1:port").
func (m *S3MockServer) URL() string {
	return m.server.URL
}

// Close shuts down the mock server.
func (m *S3MockServer) Close() {
	m.server.Close()
}

// CreateBucket initializes an empty bucket.
func (m *S3MockServer) CreateBucket(bucket string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.buckets[bucket]; !exists {
		m.buckets[bucket] = make(map[string]*MockS3Object)
	}
}

// SetFaulty configures the bucket to return 500 Internal Server Error for testing error handling.
func (m *S3MockServer) SetFaulty(bucket string, faulty bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.faulty[bucket] = faulty
}

// SetUnauthorized configures the bucket to return 403 Forbidden.
func (m *S3MockServer) SetUnauthorized(bucket string, unauthorized bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.unauthorized[bucket] = unauthorized
}

// GetObjectData retrieves stored object bytes directly for assertion.
func (m *S3MockServer) GetObjectData(bucket, key string) ([]byte, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	b, exists := m.buckets[bucket]
	if !exists {
		return nil, false
	}
	obj, exists := b[key]
	if !exists {
		return nil, false
	}
	res := make([]byte, len(obj.Data))
	copy(res, obj.Data)
	return res, true
}

// ListKeys returns all keys in the given bucket matching prefix.
func (m *S3MockServer) ListKeys(bucket, prefix string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	b, exists := m.buckets[bucket]
	if !exists {
		return nil
	}

	var keys []string
	for k := range b {
		if prefix == "" || strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys
}

// Reset clears all buckets and fault injection flags.
func (m *S3MockServer) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.buckets = make(map[string]map[string]*MockS3Object)
	m.faulty = make(map[string]bool)
	m.unauthorized = make(map[string]bool)
}

func (m *S3MockServer) handleS3Request(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	parts := strings.SplitN(path, "/", 2)

	bucket := parts[0]
	var key string
	if len(parts) > 1 {
		key = parts[1]
	}

	m.mu.RLock()
	isFaulty := m.faulty[bucket]
	isUnauthorized := m.unauthorized[bucket]
	m.mu.RUnlock()

	if isFaulty {
		m.writeError(w, http.StatusInternalServerError, "InternalError", "Simulated internal storage fault", bucket, key)
		return
	}

	if isUnauthorized {
		m.writeError(w, http.StatusForbidden, "AccessDenied", "Simulated access denied", bucket, key)
		return
	}

	// 1. Root / Bucket-level operations
	if key == "" {
		switch r.Method {
		case http.MethodHead:
			// HeadBucket
			m.mu.RLock()
			_, exists := m.buckets[bucket]
			m.mu.RUnlock()

			if !exists {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.WriteHeader(http.StatusOK)
			return

		case http.MethodGet:
			// ListObjectsV2
			m.mu.RLock()
			b, exists := m.buckets[bucket]
			m.mu.RUnlock()

			if !exists {
				m.writeError(w, http.StatusNotFound, "NoSuchBucket", "The specified bucket does not exist", bucket, "")
				return
			}

			prefix := r.URL.Query().Get("prefix")
			var contents []S3ObjectEntry
			m.mu.RLock()
			for k, obj := range b {
				if prefix == "" || strings.HasPrefix(k, prefix) {
					contents = append(contents, S3ObjectEntry{
						Key:          k,
						LastModified: obj.LastModified.UTC().Format(time.RFC3339),
						ETag:         fmt.Sprintf("\"%s\"", obj.ETag),
						Size:         int64(len(obj.Data)),
						StorageClass: "STANDARD",
					})
				}
			}
			m.mu.RUnlock()

			sort.Slice(contents, func(i, j int) bool {
				return contents[i].Key < contents[j].Key
			})

			res := ListBucketResultXML{
				Name:        bucket,
				Prefix:      prefix,
				KeyCount:    len(contents),
				MaxKeys:     1000,
				IsTruncated: false,
				Contents:    contents,
			}

			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusOK)
			_ = xml.NewEncoder(w).Encode(res)
			return

		case http.MethodPut:
			// CreateBucket
			m.mu.Lock()
			if _, exists := m.buckets[bucket]; !exists {
				m.buckets[bucket] = make(map[string]*MockS3Object)
			}
			m.mu.Unlock()
			w.WriteHeader(http.StatusOK)
			return

		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
	}

	// 2. Object-level operations
	switch r.Method {
	case http.MethodPut:
		// PutObject
		data, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		_ = r.Body.Close()

		h := md5.Sum(data)
		etag := hex.EncodeToString(h[:])

		m.mu.Lock()
		b, exists := m.buckets[bucket]
		if !exists {
			b = make(map[string]*MockS3Object)
			m.buckets[bucket] = b
		}
		b[key] = &MockS3Object{
			Key:          key,
			Data:         data,
			ContentType:  r.Header.Get("Content-Type"),
			LastModified: time.Now().UTC(),
			ETag:         etag,
		}
		m.mu.Unlock()

		w.Header().Set("ETag", fmt.Sprintf("\"%s\"", etag))
		w.WriteHeader(http.StatusOK)

	case http.MethodGet:
		// GetObject
		m.mu.RLock()
		b, bExists := m.buckets[bucket]
		var obj *MockS3Object
		if bExists {
			obj = b[key]
		}
		m.mu.RUnlock()

		if !bExists {
			m.writeError(w, http.StatusNotFound, "NoSuchBucket", "The specified bucket does not exist", bucket, key)
			return
		}
		if obj == nil {
			m.writeError(w, http.StatusNotFound, "NoSuchKey", "The specified key does not exist", bucket, key)
			return
		}

		w.Header().Set("Content-Type", obj.ContentType)
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(obj.Data)))
		w.Header().Set("ETag", fmt.Sprintf("\"%s\"", obj.ETag))
		w.Header().Set("Last-Modified", obj.LastModified.Format(http.TimeFormat))
		w.WriteHeader(http.StatusOK)
		_, _ = io.Copy(w, bytes.NewReader(obj.Data))

	case http.MethodHead:
		// HeadObject
		m.mu.RLock()
		b, bExists := m.buckets[bucket]
		var obj *MockS3Object
		if bExists {
			obj = b[key]
		}
		m.mu.RUnlock()

		if !bExists || obj == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", obj.ContentType)
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(obj.Data)))
		w.Header().Set("ETag", fmt.Sprintf("\"%s\"", obj.ETag))
		w.Header().Set("Last-Modified", obj.LastModified.Format(http.TimeFormat))
		w.WriteHeader(http.StatusOK)

	case http.MethodDelete:
		// DeleteObject
		m.mu.Lock()
		if b, exists := m.buckets[bucket]; exists {
			delete(b, key)
		}
		m.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (m *S3MockServer) writeError(w http.ResponseWriter, statusCode int, code, message, bucket, key string) {
	errPayload := S3ErrorXML{
		Code:    code,
		Message: message,
		Bucket:  bucket,
		Key:     key,
	}
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(statusCode)
	_ = xml.NewEncoder(w).Encode(errPayload)
}
