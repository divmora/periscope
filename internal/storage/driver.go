package storage

import (
	"context"
	"errors"
	"io"
	"time"
)

var (
	// ErrObjectNotFound is returned when an object does not exist in storage.
	ErrObjectNotFound = errors.New("storage: object not found")

	// ErrBucketNotFound is returned when a requested storage bucket/container does not exist.
	ErrBucketNotFound = errors.New("storage: bucket not found")

	// ErrInvalidConfig is returned when the storage configuration is missing or invalid.
	ErrInvalidConfig = errors.New("storage: invalid storage configuration")

	// ErrPathTraversal is returned when a requested path attempts to escape the root directory.
	ErrPathTraversal = errors.New("storage: path traversal detected")

	// ErrPermissionDenied is returned when access to the storage resource is forbidden.
	ErrPermissionDenied = errors.New("storage: permission denied")

	// ErrUnsupportedDriver is returned when an unrecognized storage driver type is requested.
	ErrUnsupportedDriver = errors.New("storage: unsupported storage driver")
)

// ObjectInfo describes metadata for an object stored in the storage backend.
type ObjectInfo struct {
	Key          string    `json:"key"`
	Size         int64     `json:"size"`
	LastModified time.Time `json:"last_modified"`
	ETag         string    `json:"etag,omitempty"`
}

// Driver provides a unified storage abstraction for object persistence,
// streaming writes, retrieval, and validation across Local Disk, AWS S3,
// MinIO, and Cloudflare R2 backends.
type Driver interface {
	// Type returns the storage driver identifier ("local", "s3", "minio", "r2").
	Type() string

	// Upload writes data from reader r to the object key with the given content type and size.
	// If size is negative, the size is inferred from the reader.
	Upload(ctx context.Context, key string, r io.Reader, size int64, contentType string) error

	// Get retrieves the object at key. The caller is responsible for closing the returned ReadCloser.
	Get(ctx context.Context, key string) (io.ReadCloser, error)

	// Exists returns true if the object at key exists, or false if not found.
	Exists(ctx context.Context, key string) (bool, error)

	// Delete removes the object at key. Returns nil if the object did not exist.
	Delete(ctx context.Context, key string) error

	// List returns all objects matching the specified prefix.
	List(ctx context.Context, prefix string) ([]ObjectInfo, error)

	// Ping validates configuration, authentication, and bucket/directory accessibility.
	Ping(ctx context.Context) error
}

// Config defines the generic storage configuration used by the factory.
type Config struct {
	Type            string `json:"type"`                        // "local", "s3", "minio", "r2"
	BasePath        string `json:"base_path,omitempty"`         // Root directory path for local driver
	Bucket          string `json:"bucket,omitempty"`            // S3 / MinIO / R2 bucket name
	Region          string `json:"region,omitempty"`            // S3 region (e.g., "us-east-1", "auto" for R2)
	Endpoint        string `json:"endpoint,omitempty"`          // Custom endpoint URL for MinIO or R2
	AccessKeyID     string `json:"access_key_id,omitempty"`     // S3 access key ID
	SecretAccessKey string `json:"secret_access_key,omitempty"` // S3 secret access key
	UsePathStyle    bool   `json:"use_path_style,omitempty"`    // Force path-style addressing (required for MinIO)
	DisableSSL      bool   `json:"disable_ssl,omitempty"`       // Disable SSL for local MinIO HTTP endpoints
}
