package cmd

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/divmora/periscope/internal/collector"
	"github.com/divmora/periscope/internal/storage"
	"github.com/divmora/periscope/internal/store"
	"github.com/spf13/cobra"
)

const testValidAESKey = "0123456789abcdef0123456789abcdef" // exactly 32 bytes
const testValidUUID = "01912345-6789-7abc-def0-1234567890ab"

// Mock storage driver for unit tests
type mockStorageDriver struct {
	mu        sync.Mutex
	uploaded  map[string][]byte
	pingError error
}

func newMockStorageDriver() *mockStorageDriver {
	return &mockStorageDriver{
		uploaded: make(map[string][]byte),
	}
}

func (m *mockStorageDriver) Type() string { return "mock" }

func (m *mockStorageDriver) Upload(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	m.uploaded[key] = data
	return nil
}

func (m *mockStorageDriver) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.uploaded[key]
	if !ok {
		return nil, storage.ErrObjectNotFound
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (m *mockStorageDriver) Exists(ctx context.Context, key string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.uploaded[key]
	return ok, nil
}

func (m *mockStorageDriver) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.uploaded, key)
	return nil
}

func (m *mockStorageDriver) List(ctx context.Context, prefix string) ([]storage.ObjectInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var list []storage.ObjectInfo
	for k, v := range m.uploaded {
		if strings.HasPrefix(k, prefix) {
			list = append(list, storage.ObjectInfo{
				Key:          k,
				Size:         int64(len(v)),
				LastModified: time.Now().UTC(),
			})
		}
	}
	return list, nil
}

func (m *mockStorageDriver) Ping(ctx context.Context) error {
	return m.pingError
}

func TestIsValidUUID(t *testing.T) {
	validUUIDs := []string{
		"01912345-6789-7abc-def0-1234567890ab",
		"a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11",
		"00000000-0000-0000-0000-000000000000",
		"FFFFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF",
	}
	for _, u := range validUUIDs {
		if !IsValidUUID(u) {
			t.Errorf("expected UUID %q to be valid", u)
		}
	}

	invalidUUIDs := []string{
		"",
		"not-a-uuid",
		"12345",
		"01912345-6789-7abc-def0-1234567890zz", // invalid hex
		"01912345-6789-7abc-def0",               // too short
		"01912345-6789-7abc-def0-1234567890abcdef", // too long
		"01912345_6789_7abc_def0_1234567890ab", // underscores instead of hyphens
	}
	for _, u := range invalidUUIDs {
		if IsValidUUID(u) {
			t.Errorf("expected UUID %q to be invalid", u)
		}
	}
}

func TestSyncCommandFlagParsing(t *testing.T) {
	testCmd := &cobra.Command{Use: "test"}
	testCmd.AddCommand(syncCmd)

	// Verify command definition
	if syncCmd.Use != "sync <project_iid>" {
		t.Errorf("expected Use 'sync <project_iid>', got %q", syncCmd.Use)
	}

	// Verify aliases
	foundAlias := false
	for _, a := range syncCmd.Aliases {
		if a == "telescope:sync" {
			foundAlias = true
			break
		}
	}
	if !foundAlias {
		t.Errorf("expected Alias 'telescope:sync', got %v", syncCmd.Aliases)
	}

	// Verify flag defaults
	dryRunFlag, err := syncCmd.Flags().GetBool("dry-run")
	if err != nil || dryRunFlag != false {
		t.Errorf("expected dry-run default false, got %v", dryRunFlag)
	}

	batchSizeFlag, err := syncCmd.Flags().GetInt("batch-size")
	if err != nil || batchSizeFlag != 10000 {
		t.Errorf("expected batch-size default 10000, got %v", batchSizeFlag)
	}

	timeoutFlag, err := syncCmd.Flags().GetDuration("timeout")
	if err != nil || timeoutFlag != 10*time.Minute {
		t.Errorf("expected timeout default 10m, got %v", timeoutFlag)
	}

	verboseFlag, err := syncCmd.Flags().GetBool("verbose")
	if err != nil || verboseFlag != false {
		t.Errorf("expected verbose default false, got %v", verboseFlag)
	}
}

func TestSyncCommandArgValidation(t *testing.T) {
	os.Setenv("AES_ENCRYPTION_KEY", testValidAESKey)
	defer os.Unsetenv("AES_ENCRYPTION_KEY")

	// 1. Missing project IID (0 args)
	var errBuf bytes.Buffer
	code, err := RunSync(context.Background(), SyncOptions{
		ProjectIID: "",
		ErrOut:     &errBuf,
	})
	if code != ExitCodeConfigError {
		t.Errorf("expected exit code %d for empty IID, got %d", ExitCodeConfigError, code)
	}
	if err == nil {
		t.Error("expected error for empty IID, got nil")
	}

	// 2. Invalid UUID format
	errBuf.Reset()
	code, err = RunSync(context.Background(), SyncOptions{
		ProjectIID: "invalid-uuid-format",
		ErrOut:     &errBuf,
	})
	if code != ExitCodeConfigError {
		t.Errorf("expected exit code %d for invalid UUID, got %d", ExitCodeConfigError, code)
	}
	if err == nil || !strings.Contains(err.Error(), "invalid project UUID") {
		t.Errorf("expected invalid UUID error, got: %v", err)
	}
}

func TestSyncCommandConflictingFlags(t *testing.T) {
	os.Setenv("AES_ENCRYPTION_KEY", testValidAESKey)
	defer os.Unsetenv("AES_ENCRYPTION_KEY")

	// Conflicting prune flags
	var errBuf bytes.Buffer
	code, err := RunSync(context.Background(), SyncOptions{
		ProjectIID: testValidUUID,
		ForcePrune: true,
		NoPrune:    true,
		BatchSize:  1000,
		ErrOut:     &errBuf,
	})
	if code != ExitCodeConfigError {
		t.Errorf("expected exit code %d for conflicting prune flags, got %d", ExitCodeConfigError, code)
	}
	if err == nil || !strings.Contains(err.Error(), "cannot specify both --force-prune and --no-prune") {
		t.Errorf("expected conflicting flags error, got: %v", err)
	}

	// Invalid batch size (<= 0)
	errBuf.Reset()
	code, err = RunSync(context.Background(), SyncOptions{
		ProjectIID: testValidUUID,
		BatchSize:  0,
		ErrOut:     &errBuf,
	})
	if code != ExitCodeConfigError {
		t.Errorf("expected exit code %d for batch-size <= 0, got %d", ExitCodeConfigError, code)
	}
	if err == nil || !strings.Contains(err.Error(), "batch-size must be greater than 0") {
		t.Errorf("expected batch size error, got: %v", err)
	}
}

func TestSyncCommandMissingAESKey(t *testing.T) {
	// 1. Unset AES key
	os.Unsetenv("AES_ENCRYPTION_KEY")

	var errBuf bytes.Buffer
	code, err := RunSync(context.Background(), SyncOptions{
		ProjectIID: testValidUUID,
		BatchSize:  1000,
		ErrOut:     &errBuf,
	})
	if code != ExitCodeConfigError {
		t.Errorf("expected exit code %d for missing AES key, got %d", ExitCodeConfigError, code)
	}
	if err == nil || !strings.Contains(err.Error(), "AES_ENCRYPTION_KEY environment variable is not set") {
		t.Errorf("expected missing AES key error, got: %v", err)
	}

	// 2. Invalid length AES key (e.g. 16 bytes instead of 32)
	os.Setenv("AES_ENCRYPTION_KEY", "too-short-key")
	defer os.Unsetenv("AES_ENCRYPTION_KEY")

	errBuf.Reset()
	code, err = RunSync(context.Background(), SyncOptions{
		ProjectIID: testValidUUID,
		BatchSize:  1000,
		ErrOut:     &errBuf,
	})
	if code != ExitCodeConfigError {
		t.Errorf("expected exit code %d for invalid length AES key, got %d", ExitCodeConfigError, code)
	}
	if err == nil || !strings.Contains(err.Error(), "AES_ENCRYPTION_KEY must be exactly 32 bytes") {
		t.Errorf("expected 32 bytes AES key error, got: %v", err)
	}
}

func TestSyncCommandProjectNotFound(t *testing.T) {
	os.Setenv("AES_ENCRYPTION_KEY", testValidAESKey)
	defer os.Unsetenv("AES_ENCRYPTION_KEY")

	var errBuf bytes.Buffer
	code, err := RunSync(context.Background(), SyncOptions{
		ProjectIID: testValidUUID,
		BatchSize:  1000,
		ErrOut:     &errBuf,
		ProjectGetter: func(ctx context.Context, iid string) (*store.Project, error) {
			return nil, nil // not found
		},
	})
	if code != ExitCodeProjectNotFound {
		t.Errorf("expected exit code %d for project not found, got %d", ExitCodeProjectNotFound, code)
	}
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected project not found error, got: %v", err)
	}

	// Check JSON error log
	var logMsg map[string]interface{}
	if err := json.Unmarshal(errBuf.Bytes(), &logMsg); err != nil {
		t.Fatalf("failed to parse JSON error output: %v, raw: %s", err, errBuf.String())
	}
	if logMsg["level"] != "ERROR" || logMsg["exit_code"] != float64(ExitCodeProjectNotFound) {
		t.Errorf("unexpected JSON error log: %+v", logMsg)
	}
}

func TestSyncCommandStorageDriverFailure(t *testing.T) {
	os.Setenv("AES_ENCRYPTION_KEY", testValidAESKey)
	defer os.Unsetenv("AES_ENCRYPTION_KEY")

	mockDriver := newMockStorageDriver()
	mockDriver.pingError = errors.New("S3 bucket not accessible")

	var errBuf bytes.Buffer
	code, err := RunSync(context.Background(), SyncOptions{
		ProjectIID: testValidUUID,
		BatchSize:  1000,
		ErrOut:     &errBuf,
		ProjectGetter: func(ctx context.Context, iid string) (*store.Project, error) {
			return &store.Project{
				ID:            1,
				IID:           iid,
				Name:          "Test Project",
				SourceDSN:     "postgres://user:pass@localhost:5432/db",
				StorageType:   "s3",
				StorageBucket: "test-bucket",
			}, nil
		},
		StorageDriverFactory: func(ctx context.Context, cfg storage.Config) (storage.Driver, error) {
			return mockDriver, nil
		},
	})

	if code != ExitCodeStorageError {
		t.Errorf("expected exit code %d for storage driver failure, got %d", ExitCodeStorageError, code)
	}
	if err == nil || !strings.Contains(err.Error(), "storage driver ping failed") {
		t.Errorf("expected storage driver ping failure error, got: %v", err)
	}
}

func TestSyncCommandSourceDBConnectionFailure(t *testing.T) {
	os.Setenv("AES_ENCRYPTION_KEY", testValidAESKey)
	defer os.Unsetenv("AES_ENCRYPTION_KEY")

	mockDriver := newMockStorageDriver()

	var errBuf bytes.Buffer
	code, err := RunSync(context.Background(), SyncOptions{
		ProjectIID: testValidUUID,
		BatchSize:  1000,
		ErrOut:     &errBuf,
		ProjectGetter: func(ctx context.Context, iid string) (*store.Project, error) {
			return &store.Project{
				ID:            1,
				IID:           iid,
				Name:          "Test Project",
				SourceDSN:     "mysql://root:wrongpassword@127.0.0.1:3306/db",
				StorageType:   "s3",
				StorageBucket: "test-bucket",
			}, nil
		},
		StorageDriverFactory: func(ctx context.Context, cfg storage.Config) (storage.Driver, error) {
			return mockDriver, nil
		},
		SourceDBOpener: func(dsn string) (*sql.DB, collector.Dialect, error) {
			return nil, collector.DialectMySQL, errors.New("access denied for user 'root'@'127.0.0.1'")
		},
	})

	if code != ExitCodeSourceDBError {
		t.Errorf("expected exit code %d for source DB failure, got %d", ExitCodeSourceDBError, code)
	}
	if err == nil || !strings.Contains(err.Error(), "failed to open source database") {
		t.Errorf("expected source database error, got: %v", err)
	}
}

func TestSyncCommandWatermarkUpdateFailure(t *testing.T) {
	os.Setenv("AES_ENCRYPTION_KEY", testValidAESKey)
	defer os.Unsetenv("AES_ENCRYPTION_KEY")

	mockDriver := newMockStorageDriver()

	var errBuf bytes.Buffer
	code, err := RunSync(context.Background(), SyncOptions{
		ProjectIID: testValidUUID,
		BatchSize:  1000,
		ErrOut:     &errBuf,
		ProjectGetter: func(ctx context.Context, iid string) (*store.Project, error) {
			return &store.Project{
				ID:            1,
				IID:           iid,
				Name:          "Test Project",
				SourceDSN:     "postgres://user:pass@localhost:5432/db",
				StorageType:   "s3",
				StorageBucket: "test-bucket",
			}, nil
		},
		StorageDriverFactory: func(ctx context.Context, cfg storage.Config) (storage.Driver, error) {
			return mockDriver, nil
		},
		SourceDBOpener: func(dsn string) (*sql.DB, collector.Dialect, error) {
			db, err := sql.Open("pgx", "")
			return db, collector.DialectPostgres, err
		},
		CollectorRunner: func(ctx context.Context, cfg collector.CollectorConfig) (*collector.SyncResult, error) {
			return nil, errors.New("failed to update watermark: database connection closed")
		},
	})

	if code != ExitCodeWatermarkError {
		t.Errorf("expected exit code %d for watermark failure, got %d", ExitCodeWatermarkError, code)
	}
	if err == nil || !strings.Contains(err.Error(), "watermark") {
		t.Errorf("expected watermark error, got: %v", err)
	}
}

func TestSyncCommandSuccessfulExecution(t *testing.T) {
	os.Setenv("AES_ENCRYPTION_KEY", testValidAESKey)
	defer os.Unsetenv("AES_ENCRYPTION_KEY")

	mockDriver := newMockStorageDriver()
	var outBuf bytes.Buffer
	var errBuf bytes.Buffer

	code, err := RunSync(context.Background(), SyncOptions{
		ProjectIID: testValidUUID,
		BatchSize:  10000,
		EntryTypes: []string{"request", "query"},
		Verbose:    true,
		Out:        &outBuf,
		ErrOut:     &errBuf,
		ProjectGetter: func(ctx context.Context, iid string) (*store.Project, error) {
			return &store.Project{
				ID:            42,
				IID:           iid,
				Name:          "Production App",
				SourceDSN:     "postgres://user:pass@localhost:5432/laravel",
				StorageType:   "s3",
				StorageBucket: "telemetry-parquet",
				PruneEnabled:  true,
			}, nil
		},
		StorageDriverFactory: func(ctx context.Context, cfg storage.Config) (storage.Driver, error) {
			return mockDriver, nil
		},
		SourceDBOpener: func(dsn string) (*sql.DB, collector.Dialect, error) {
			db, err := sql.Open("pgx", "")
			return db, collector.DialectPostgres, err
		},
		CollectorRunner: func(ctx context.Context, cfg collector.CollectorConfig) (*collector.SyncResult, error) {
			if cfg.ProjectID != 42 || cfg.ProjectIID != testValidUUID {
				t.Fatalf("unexpected collector project ID: %d, IID: %s", cfg.ProjectID, cfg.ProjectIID)
			}
			if len(cfg.EntryTypes) != 2 || cfg.EntryTypes[0] != "request" || cfg.EntryTypes[1] != "query" {
				t.Fatalf("unexpected entry types: %v", cfg.EntryTypes)
			}
			if !cfg.PruneEnabled {
				t.Error("expected PruneEnabled to be true from project config")
			}

			return &collector.SyncResult{
				ProjectIID:           testValidUUID,
				EntriesPolled:        1500,
				TotalRecordsWritten:  1500,
				TotalBytesCompressed: 45200,
				TotalPruned:          1500,
				FlushedBatches: []collector.FlushResult{
					{
						PartitionKey: collector.PartitionKey{
							ProjectIID: testValidUUID,
							Type:       "request",
							Year:       2026,
							Month:      8,
							Day:        20,
							Hour:       12,
						},
						RecordCount:     1000,
						MinSequence:     1,
						MaxSequence:     1000,
						BytesCompressed: 30000,
						StorageKey:      "projectId=01912345-6789-7abc-def0-1234567890ab/type=request/year=2026/month=08/day=20/hour=12/part-001.parquet",
						Duration:        120 * time.Millisecond,
						PrunedCount:     1000,
					},
					{
						PartitionKey: collector.PartitionKey{
							ProjectIID: testValidUUID,
							Type:       "query",
							Year:       2026,
							Month:      8,
							Day:        20,
							Hour:       12,
						},
						RecordCount:     500,
						MinSequence:     1001,
						MaxSequence:     1500,
						BytesCompressed: 15200,
						StorageKey:      "projectId=01912345-6789-7abc-def0-1234567890ab/type=query/year=2026/month=08/day=20/hour=12/part-002.parquet",
						Duration:        85 * time.Millisecond,
						PrunedCount:     500,
					},
				},
				Watermarks: map[string]int64{
					"request": 1000,
					"query":   1500,
				},
				Duration: 250 * time.Millisecond,
			}, nil
		},
	})

	if code != ExitCodeSuccess {
		t.Errorf("expected exit code %d for successful run, got %d, err: %v", ExitCodeSuccess, code, err)
	}
	if err != nil {
		t.Errorf("expected nil error, got: %v", err)
	}

	// Verify structured output lines
	lines := strings.Split(strings.TrimSpace(outBuf.String()), "\n")
	if len(lines) < 4 {
		t.Fatalf("expected at least 4 log lines (start, 2 batches, completion), got %d lines: %s", len(lines), outBuf.String())
	}

	// Check start line
	var startLog map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &startLog); err != nil {
		t.Fatalf("failed to unmarshal start log: %v", err)
	}
	if startLog["msg"] != "starting periscope sync" || startLog["project_iid"] != testValidUUID {
		t.Errorf("unexpected start log: %+v", startLog)
	}

	// Check batch lines
	var batch1Log map[string]interface{}
	if err := json.Unmarshal([]byte(lines[1]), &batch1Log); err != nil {
		t.Fatalf("failed to unmarshal batch 1 log: %v", err)
	}
	if batch1Log["msg"] != "batch flushed" || batch1Log["type"] != "request" || batch1Log["records"] != float64(1000) {
		t.Errorf("unexpected batch 1 log: %+v", batch1Log)
	}

	// Check completion line
	var compLog map[string]interface{}
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &compLog); err != nil {
		t.Fatalf("failed to unmarshal completion log: %v", err)
	}
	if compLog["msg"] != "sync completed successfully" || compLog["entries_polled"] != float64(1500) {
		t.Errorf("unexpected completion log: %+v", compLog)
	}
}

func TestSyncCommandDryRun(t *testing.T) {
	os.Setenv("AES_ENCRYPTION_KEY", testValidAESKey)
	defer os.Unsetenv("AES_ENCRYPTION_KEY")

	mockDriver := newMockStorageDriver()
	var outBuf bytes.Buffer

	code, err := RunSync(context.Background(), SyncOptions{
		ProjectIID: testValidUUID,
		BatchSize:  1000,
		DryRun:     true,
		Out:        &outBuf,
		ProjectGetter: func(ctx context.Context, iid string) (*store.Project, error) {
			return &store.Project{
				ID:            1,
				IID:           iid,
				Name:          "Dry Run App",
				SourceDSN:     "postgres://user:pass@localhost:5432/laravel",
				StorageType:   "s3",
				StorageBucket: "test-bucket",
			}, nil
		},
		StorageDriverFactory: func(ctx context.Context, cfg storage.Config) (storage.Driver, error) {
			return mockDriver, nil
		},
		SourceDBOpener: func(dsn string) (*sql.DB, collector.Dialect, error) {
			db, err := sql.Open("pgx", "")
			return db, collector.DialectPostgres, err
		},
		CollectorRunner: func(ctx context.Context, cfg collector.CollectorConfig) (*collector.SyncResult, error) {
			if !cfg.DryRun {
				t.Error("expected DryRun = true on collector config")
			}
			return &collector.SyncResult{
				ProjectIID:          testValidUUID,
				EntriesPolled:       100,
				TotalRecordsWritten: 0,
				TotalPruned:         0,
				Watermarks:          map[string]int64{},
				Duration:            50 * time.Millisecond,
			}, nil
		},
	})

	if code != ExitCodeSuccess || err != nil {
		t.Errorf("expected success on dry run, got code %d, err %v", code, err)
	}

	if !strings.Contains(outBuf.String(), `"dry_run":true`) {
		t.Errorf("expected dry_run:true in log output: %s", outBuf.String())
	}
}

func TestSyncCommandPruneOverrides(t *testing.T) {
	os.Setenv("AES_ENCRYPTION_KEY", testValidAESKey)
	defer os.Unsetenv("AES_ENCRYPTION_KEY")

	mockDriver := newMockStorageDriver()

	// 1. Force Prune overrides project.PruneEnabled = false
	var pruneEnabledObserved bool
	code, err := RunSync(context.Background(), SyncOptions{
		ProjectIID: testValidUUID,
		BatchSize:  1000,
		ForcePrune: true,
		ProjectGetter: func(ctx context.Context, iid string) (*store.Project, error) {
			return &store.Project{
				ID:           1,
				IID:          iid,
				SourceDSN:    "postgres://user:pass@localhost:5432/laravel",
				PruneEnabled: false,
			}, nil
		},
		StorageDriverFactory: func(ctx context.Context, cfg storage.Config) (storage.Driver, error) {
			return mockDriver, nil
		},
		SourceDBOpener: func(dsn string) (*sql.DB, collector.Dialect, error) {
			db, err := sql.Open("pgx", "")
			return db, collector.DialectPostgres, err
		},
		CollectorRunner: func(ctx context.Context, cfg collector.CollectorConfig) (*collector.SyncResult, error) {
			pruneEnabledObserved = cfg.PruneEnabled
			return &collector.SyncResult{}, nil
		},
	})
	if code != ExitCodeSuccess || err != nil || !pruneEnabledObserved {
		t.Errorf("expected ForcePrune to override PruneEnabled to true, got %v", pruneEnabledObserved)
	}

	// 2. No Prune overrides project.PruneEnabled = true
	code, err = RunSync(context.Background(), SyncOptions{
		ProjectIID: testValidUUID,
		BatchSize:  1000,
		NoPrune:    true,
		ProjectGetter: func(ctx context.Context, iid string) (*store.Project, error) {
			return &store.Project{
				ID:           1,
				IID:          iid,
				SourceDSN:    "postgres://user:pass@localhost:5432/laravel",
				PruneEnabled: true,
			}, nil
		},
		StorageDriverFactory: func(ctx context.Context, cfg storage.Config) (storage.Driver, error) {
			return mockDriver, nil
		},
		SourceDBOpener: func(dsn string) (*sql.DB, collector.Dialect, error) {
			db, err := sql.Open("pgx", "")
			return db, collector.DialectPostgres, err
		},
		CollectorRunner: func(ctx context.Context, cfg collector.CollectorConfig) (*collector.SyncResult, error) {
			pruneEnabledObserved = cfg.PruneEnabled
			return &collector.SyncResult{}, nil
		},
	})
	if code != ExitCodeSuccess || err != nil || pruneEnabledObserved {
		t.Errorf("expected NoPrune to override PruneEnabled to false, got %v", pruneEnabledObserved)
	}
}

func TestSyncCommandWithLocalStorageDirOverride(t *testing.T) {
	os.Setenv("AES_ENCRYPTION_KEY", testValidAESKey)
	defer os.Unsetenv("AES_ENCRYPTION_KEY")

	tempDir := t.TempDir()
	var observedDriver storage.Driver

	code, err := RunSync(context.Background(), SyncOptions{
		ProjectIID: testValidUUID,
		BatchSize:  1000,
		StorageDir: tempDir,
		ProjectGetter: func(ctx context.Context, iid string) (*store.Project, error) {
			return &store.Project{
				ID:            1,
				IID:           iid,
				SourceDSN:     "postgres://user:pass@localhost:5432/laravel",
				StorageType:   "s3",
				StorageBucket: "ignored-s3-bucket",
			}, nil
		},
		SourceDBOpener: func(dsn string) (*sql.DB, collector.Dialect, error) {
			db, err := sql.Open("pgx", "")
			return db, collector.DialectPostgres, err
		},
		CollectorRunner: func(ctx context.Context, cfg collector.CollectorConfig) (*collector.SyncResult, error) {
			observedDriver = cfg.StorageDriver
			return &collector.SyncResult{}, nil
		},
	})

	if code != ExitCodeSuccess || err != nil {
		t.Errorf("expected success with storage-dir override, got code %d, err %v", code, err)
	}
	if observedDriver == nil || observedDriver.Type() != "local" {
		t.Errorf("expected local storage driver, got: %v", observedDriver)
	}
}

func TestSyncCommandCobraExitInterception(t *testing.T) {
	os.Setenv("AES_ENCRYPTION_KEY", testValidAESKey)
	defer os.Unsetenv("AES_ENCRYPTION_KEY")

	origExit := osExit
	var lastExitCode int
	osExit = func(code int) {
		lastExitCode = code
	}
	defer func() { osExit = origExit }()

	// Execute command with 0 args
	syncCmd.Run(syncCmd, []string{})
	if lastExitCode != ExitCodeConfigError {
		t.Errorf("expected exit code %d for 0 args, got %d", ExitCodeConfigError, lastExitCode)
	}

	// Execute command with 2 args
	lastExitCode = -1
	syncCmd.Run(syncCmd, []string{"uuid1", "uuid2"})
	if lastExitCode != ExitCodeConfigError {
		t.Errorf("expected exit code %d for 2 args, got %d", ExitCodeConfigError, lastExitCode)
	}
}

func TestSyncPipelineWithRealLocalDriver(t *testing.T) {
	os.Setenv("AES_ENCRYPTION_KEY", testValidAESKey)
	defer os.Unsetenv("AES_ENCRYPTION_KEY")

	tempDir := t.TempDir()
	var outBuf bytes.Buffer

	now := time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC)
	watermarkStore := make(map[string]int64)

	code, err := RunSync(context.Background(), SyncOptions{
		ProjectIID: testValidUUID,
		BatchSize:  2,
		StorageDir: tempDir,
		Verbose:    true,
		Out:        &outBuf,
		ProjectGetter: func(ctx context.Context, iid string) (*store.Project, error) {
			return &store.Project{
				ID:           99,
				IID:          iid,
				Name:         "E2E Real Local Storage App",
				SourceDSN:    "postgres://user:pass@localhost:5432/laravel",
				PruneEnabled: false,
			}, nil
		},
		SourceDBOpener: func(dsn string) (*sql.DB, collector.Dialect, error) {
			db, err := sql.Open("pgx", "")
			return db, collector.DialectPostgres, err
		},
		CollectorRunner: func(ctx context.Context, cfg collector.CollectorConfig) (*collector.SyncResult, error) {
			// Ingest 3 real records using flusher
			flusher := collector.NewFlusher(collector.FlusherConfig{
				ProjectID:     cfg.ProjectID,
				ProjectIID:    cfg.ProjectIID,
				BatchSize:     cfg.BatchSize,
				Driver:        cfg.StorageDriver,
				PruneEnabled:  cfg.PruneEnabled,
				DryRun:        cfg.DryRun,
				WatermarkUpdater: func(ctx context.Context, projectID int, entryType string, seq int64) error {
					watermarkStore[entryType] = seq
					return nil
				},
			})

			resList := make([]collector.FlushResult, 0)
			for seq := int64(1); seq <= 3; seq++ {
				rec := collector.ExtractRecord(collector.TelescopeEntry{
					Sequence:  seq,
					UUID:      fmt.Sprintf("e2e-uuid-%d", seq),
					Type:      "request",
					CreatedAt: now,
					Tags:      []string{"env:test"},
					Content:   fmt.Sprintf(`{"uri":"/api/orders/%d","response_status":200,"duration":25.0}`, seq),
				})
				fRes, err := flusher.AddRecord(ctx, rec)
				if err != nil {
					return nil, err
				}
				if fRes != nil {
					resList = append(resList, *fRes)
				}
			}

			finalFlushes, err := flusher.FlushAll(ctx)
			if err != nil {
				return nil, err
			}
			resList = append(resList, finalFlushes...)

			var totalWritten int
			var totalBytes int64
			for _, f := range resList {
				totalWritten += f.RecordCount
				totalBytes += f.BytesCompressed
			}

			return &collector.SyncResult{
				ProjectIID:           cfg.ProjectIID,
				EntriesPolled:        3,
				TotalRecordsWritten:  totalWritten,
				TotalBytesCompressed: totalBytes,
				FlushedBatches:       resList,
				Watermarks:           watermarkStore,
				Duration:             10 * time.Millisecond,
			}, nil
		},
	})

	if code != ExitCodeSuccess || err != nil {
		t.Fatalf("expected success, got code %d, err %v", code, err)
	}

	if watermarkStore["request"] != 3 {
		t.Errorf("expected watermark for request to be 3, got %d", watermarkStore["request"])
	}

	// Verify Parquet file was actually created on local disk
	files, err := filepath.Glob(filepath.Join(tempDir, "projectId="+testValidUUID, "**", "*.parquet"))
	if err != nil || len(files) == 0 {
		// Also check without double glob if walk
		var found []string
		_ = filepath.Walk(tempDir, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() && strings.HasSuffix(path, ".parquet") {
				found = append(found, path)
			}
			return nil
		})
		if len(found) == 0 {
			t.Fatalf("expected parquet files in %s, found none", tempDir)
		}
		files = found
	}

	t.Logf("Successfully wrote %d parquet file(s) to %s", len(files), tempDir)
}
