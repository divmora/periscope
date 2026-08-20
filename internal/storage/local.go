package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// LocalConfig holds configuration settings for the local filesystem storage driver.
type LocalConfig struct {
	BasePath string      `json:"base_path"`
	DirMode  os.FileMode `json:"dir_mode,omitempty"`
	FileMode os.FileMode `json:"file_mode,omitempty"`
}

// LocalDriver implements the Driver interface using the local filesystem.
type LocalDriver struct {
	basePath string
	dirMode  os.FileMode
	fileMode os.FileMode
}

// NewLocalDriver instantiates a new LocalDriver rooted at the specified BasePath.
func NewLocalDriver(cfg LocalConfig) (*LocalDriver, error) {
	if strings.TrimSpace(cfg.BasePath) == "" {
		return nil, ErrInvalidConfig
	}

	absPath, err := filepath.Abs(filepath.Clean(cfg.BasePath))
	if err != nil {
		return nil, fmt.Errorf("%w: failed to resolve base path: %v", ErrInvalidConfig, err)
	}

	dirMode := cfg.DirMode
	if dirMode == 0 {
		dirMode = 0755
	}

	fileMode := cfg.FileMode
	if fileMode == 0 {
		fileMode = 0644
	}

	return &LocalDriver{
		basePath: absPath,
		dirMode:  dirMode,
		fileMode: fileMode,
	}, nil
}

// Type returns the driver identifier "local".
func (d *LocalDriver) Type() string {
	return "local"
}

// resolvePath converts an object key into a secure local filesystem path
// and verifies strict path containment within BasePath to prevent traversal attacks.
func (d *LocalDriver) resolvePath(key string) (string, error) {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return "", ErrInvalidConfig
	}

	// Reject null byte injection
	if strings.Contains(trimmed, "\x00") {
		return "", ErrPathTraversal
	}

	slashNormalized := filepath.ToSlash(trimmed)
	if strings.HasPrefix(slashNormalized, "/") || strings.HasPrefix(slashNormalized, "../") || slashNormalized == ".." ||
		strings.Contains(slashNormalized, "/../") || strings.HasSuffix(slashNormalized, "/..") {
		return "", ErrPathTraversal
	}

	cleanRel := slashNormalized
	if cleanRel == "" || cleanRel == "." {
		return "", ErrInvalidConfig
	}

	target := filepath.Join(d.basePath, filepath.FromSlash(cleanRel))
	target = filepath.Clean(target)

	rel, err := filepath.Rel(d.basePath, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", ErrPathTraversal
	}

	return target, nil
}

// Upload writes data from reader r to the object key.
// It executes an atomic write by writing to a temporary file first, syncing to disk,
// and atomically renaming to the target path.
func (d *LocalDriver) Upload(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
	targetPath, err := d.resolvePath(key)
	if err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, d.dirMode); err != nil {
		if os.IsPermission(err) {
			return ErrPermissionDenied
		}
		return err
	}

	tmpFile, err := os.CreateTemp(dir, fmt.Sprintf(".tmp-periscope-%s-*", uuid.NewString()))
	if err != nil {
		if os.IsPermission(err) {
			return ErrPermissionDenied
		}
		return err
	}
	tmpPath := tmpFile.Name()

	var success bool
	defer func() {
		if !success {
			_ = tmpFile.Close()
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := io.Copy(tmpFile, r); err != nil {
		return err
	}

	if err := tmpFile.Sync(); err != nil {
		return err
	}

	if err := tmpFile.Close(); err != nil {
		return err
	}

	if err := os.Chmod(tmpPath, d.fileMode); err != nil {
		if os.IsPermission(err) {
			return ErrPermissionDenied
		}
		return err
	}

	if err := os.Rename(tmpPath, targetPath); err != nil {
		if os.IsPermission(err) {
			return ErrPermissionDenied
		}
		return err
	}

	success = true
	return nil
}

// Get opens and returns a reader for the object at key.
func (d *LocalDriver) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	targetPath, err := d.resolvePath(key)
	if err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	f, err := os.Open(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrObjectNotFound
		}
		if os.IsPermission(err) {
			return nil, ErrPermissionDenied
		}
		return nil, err
	}

	stat, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}

	if stat.IsDir() {
		_ = f.Close()
		return nil, ErrObjectNotFound
	}

	return f, nil
}

// Exists checks if an object exists at the specified key.
func (d *LocalDriver) Exists(ctx context.Context, key string) (bool, error) {
	targetPath, err := d.resolvePath(key)
	if err != nil {
		return false, err
	}

	if err := ctx.Err(); err != nil {
		return false, err
	}

	stat, err := os.Stat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		if os.IsPermission(err) {
			return false, ErrPermissionDenied
		}
		return false, err
	}

	if stat.IsDir() {
		return false, nil
	}

	return true, nil
}

// Delete removes the object at the specified key.
func (d *LocalDriver) Delete(ctx context.Context, key string) error {
	targetPath, err := d.resolvePath(key)
	if err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	err = os.Remove(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		if os.IsPermission(err) {
			return ErrPermissionDenied
		}
		return err
	}

	return nil
}

// List recursively traverses the storage root and returns objects matching the specified prefix.
func (d *LocalDriver) List(ctx context.Context, prefix string) ([]ObjectInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	normalizedPrefix := filepath.ToSlash(strings.TrimPrefix(prefix, "/"))

	var objects []ObjectInfo
	err := filepath.Walk(d.basePath, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				return nil
			}
			if os.IsPermission(walkErr) {
				return ErrPermissionDenied
			}
			return walkErr
		}

		if err := ctx.Err(); err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Skip temporary files and ping probe files
		name := info.Name()
		if strings.HasPrefix(name, ".tmp-periscope-") || strings.HasPrefix(name, ".periscope_ping_") {
			return nil
		}

		rel, err := filepath.Rel(d.basePath, path)
		if err != nil {
			return err
		}

		key := filepath.ToSlash(rel)
		if normalizedPrefix != "" && !strings.HasPrefix(key, normalizedPrefix) {
			return nil
		}

		objects = append(objects, ObjectInfo{
			Key:          key,
			Size:         info.Size(),
			LastModified: info.ModTime().UTC(),
		})

		return nil
	})

	if err != nil {
		return nil, err
	}

	return objects, nil
}

// Ping validates directory availability and write/read permissions by probing a temporary file.
func (d *LocalDriver) Ping(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if err := os.MkdirAll(d.basePath, d.dirMode); err != nil {
		if os.IsPermission(err) {
			return ErrPermissionDenied
		}
		return err
	}

	probePath := filepath.Join(d.basePath, fmt.Sprintf(".periscope_ping_%s", uuid.NewString()))
	if err := os.WriteFile(probePath, []byte("ping"), d.fileMode); err != nil {
		if os.IsPermission(err) {
			return ErrPermissionDenied
		}
		return err
	}
	defer func() {
		_ = os.Remove(probePath)
	}()

	data, err := os.ReadFile(probePath)
	if err != nil {
		if os.IsPermission(err) {
			return ErrPermissionDenied
		}
		return err
	}

	if string(data) != "ping" {
		return fmt.Errorf("%w: probe verification failed", ErrPermissionDenied)
	}

	return nil
}
