package cmd

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/divmora/periscope/internal/collector"
	"github.com/divmora/periscope/internal/storage"
	"github.com/divmora/periscope/internal/store"
	"github.com/spf13/cobra"
)

// Structured exit codes for periscope sync CLI command
const (
	ExitCodeSuccess         = 0 // Success (synced entries or no new entries)
	ExitCodeRuntimeError    = 1 // Runtime / collector failure
	ExitCodeConfigError     = 2 // Configuration / argument error (invalid UUID, missing AES key, flag error)
	ExitCodeProjectNotFound = 3 // Project not found
	ExitCodeSourceDBError   = 4 // Source DB connection failure
	ExitCodeStorageError    = 5 // Storage driver initialization / ping failure
	ExitCodeWatermarkError  = 6 // Watermark update failure
)

// osExit allows test suites to intercept process termination.
var osExit = os.Exit

// uuidRegex validates RFC 4122 UUID strings (UUID v4 / v7).
var uuidRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// IsValidUUID returns true if s is a valid 36-character hyphenated UUID.
func IsValidUUID(s string) bool {
	return uuidRegex.MatchString(strings.TrimSpace(s))
}

// LogEntry defines the standard structured JSON log schema for periscope sync.
type LogEntry struct {
	Time       string                 `json:"time"`
	Level      string                 `json:"level"`
	Msg        string                 `json:"msg"`
	ProjectIID string                 `json:"project_iid,omitempty"`
	ExitCode   *int                   `json:"exit_code,omitempty"`
	Error      string                 `json:"error,omitempty"`
	Extra      map[string]interface{} `json:"-"`
}

func logJSON(w io.Writer, level, msg string, fields map[string]interface{}) {
	if w == nil {
		w = os.Stdout
	}
	m := make(map[string]interface{}, len(fields)+3)
	m["time"] = time.Now().UTC().Format(time.RFC3339)
	m["level"] = level
	m["msg"] = msg
	for k, v := range fields {
		m[k] = v
	}
	enc := json.NewEncoder(w)
	_ = enc.Encode(m)
}

// SyncOptions holds all runtime configuration and dependency hooks for executing sync.
type SyncOptions struct {
	ProjectIID string
	DryRun     bool
	BatchSize  int
	EntryTypes []string
	Timeout    time.Duration
	Verbose    bool
	ForcePrune bool
	NoPrune    bool
	StorageDir string

	Out    io.Writer
	ErrOut io.Writer

	// Dependency injection hooks for testing
	ProjectGetter        func(ctx context.Context, iid string) (*store.Project, error)
	SourceDBOpener       func(dsn string) (*sql.DB, collector.Dialect, error)
	StorageDriverFactory func(ctx context.Context, cfg storage.Config) (storage.Driver, error)
	CollectorRunner      func(ctx context.Context, cfg collector.CollectorConfig) (*collector.SyncResult, error)
}

// RunSync executes the sync workflow end-to-end and returns a structured exit code.
func RunSync(ctx context.Context, opts SyncOptions) (int, error) {
	out := opts.Out
	if out == nil {
		out = os.Stdout
	}
	errOut := opts.ErrOut
	if errOut == nil {
		errOut = os.Stderr
	}

	projectIID := strings.TrimSpace(opts.ProjectIID)

	// 1. Argument Validation: Project UUID
	if projectIID == "" || !IsValidUUID(projectIID) {
		err := fmt.Errorf("invalid project UUID format: %q", opts.ProjectIID)
		exitCode := ExitCodeConfigError
		logJSON(errOut, "ERROR", err.Error(), map[string]interface{}{
			"project_iid": opts.ProjectIID,
			"exit_code":   exitCode,
		})
		return exitCode, err
	}

	// 2. Flag Validation
	if opts.ForcePrune && opts.NoPrune {
		err := errors.New("cannot specify both --force-prune and --no-prune")
		exitCode := ExitCodeConfigError
		logJSON(errOut, "ERROR", err.Error(), map[string]interface{}{
			"project_iid": projectIID,
			"exit_code":   exitCode,
		})
		return exitCode, err
	}

	if opts.BatchSize <= 0 {
		err := fmt.Errorf("batch-size must be greater than 0, got %d", opts.BatchSize)
		exitCode := ExitCodeConfigError
		logJSON(errOut, "ERROR", err.Error(), map[string]interface{}{
			"project_iid": projectIID,
			"exit_code":   exitCode,
		})
		return exitCode, err
	}

	// 3. Credential Decryption / AES Key Validation
	aesKey := os.Getenv("AES_ENCRYPTION_KEY")
	if aesKey == "" {
		err := errors.New("AES_ENCRYPTION_KEY environment variable is not set")
		exitCode := ExitCodeConfigError
		logJSON(errOut, "ERROR", err.Error(), map[string]interface{}{
			"project_iid": projectIID,
			"exit_code":   exitCode,
		})
		return exitCode, err
	}
	if len([]byte(aesKey)) != 32 {
		err := fmt.Errorf("AES_ENCRYPTION_KEY must be exactly 32 bytes for AES-256 (got %d bytes)", len([]byte(aesKey)))
		exitCode := ExitCodeConfigError
		logJSON(errOut, "ERROR", err.Error(), map[string]interface{}{
			"project_iid": projectIID,
			"exit_code":   exitCode,
		})
		return exitCode, err
	}

	// 4. Project Configuration Retrieval
	projectGetter := opts.ProjectGetter
	if projectGetter == nil {
		if store.Pool == nil {
			if err := store.Connect(ctx); err != nil {
				exitCode := ExitCodeRuntimeError
				logJSON(errOut, "ERROR", "failed to connect to metadata database", map[string]interface{}{
					"project_iid": projectIID,
					"exit_code":   exitCode,
					"error":       err.Error(),
				})
				return exitCode, fmt.Errorf("metadata database connection failed: %w", err)
			}
		}
		projectGetter = store.GetProjectByIID
	}

	project, err := projectGetter(ctx, projectIID)
	if err != nil {
		if errors.Is(err, store.ErrProjectNotFound) || errors.Is(err, sql.ErrNoRows) || strings.Contains(strings.ToLower(err.Error()), "no rows") {
			exitCode := ExitCodeProjectNotFound
			logJSON(errOut, "ERROR", "project not found", map[string]interface{}{
				"project_iid": projectIID,
				"exit_code":   exitCode,
				"error":       err.Error(),
			})
			return exitCode, fmt.Errorf("project %s not found: %w", projectIID, err)
		}
		exitCode := ExitCodeRuntimeError
		logJSON(errOut, "ERROR", "failed to query project from metadata database", map[string]interface{}{
			"project_iid": projectIID,
			"exit_code":   exitCode,
			"error":       err.Error(),
		})
		return exitCode, fmt.Errorf("failed to query project %s: %w", projectIID, err)
	}

	if project == nil {
		exitCode := ExitCodeProjectNotFound
		logJSON(errOut, "ERROR", "project not found", map[string]interface{}{
			"project_iid": projectIID,
			"exit_code":   exitCode,
		})
		return exitCode, fmt.Errorf("project with IID %s not found", projectIID)
	}

	if project.SourceDSN == "" {
		exitCode := ExitCodeConfigError
		logJSON(errOut, "ERROR", "project source DSN is empty or failed decryption", map[string]interface{}{
			"project_iid": projectIID,
			"exit_code":   exitCode,
		})
		return exitCode, fmt.Errorf("project %s has empty source DSN", projectIID)
	}

	// 5. Pruning Configuration
	pruneEnabled := project.PruneEnabled
	if opts.ForcePrune {
		pruneEnabled = true
	} else if opts.NoPrune {
		pruneEnabled = false
	}

	// 6. Storage Driver Initialization
	var storageDriver storage.Driver
	if opts.StorageDir != "" {
		localDriver, err := storage.NewLocalDriver(storage.LocalConfig{
			BasePath: opts.StorageDir,
		})
		if err != nil {
			exitCode := ExitCodeStorageError
			logJSON(errOut, "ERROR", "failed to initialize local storage driver override", map[string]interface{}{
				"project_iid": projectIID,
				"storage_dir": opts.StorageDir,
				"exit_code":   exitCode,
				"error":       err.Error(),
			})
			return exitCode, fmt.Errorf("local storage driver initialization failed: %w", err)
		}
		storageDriver = localDriver
	} else if opts.StorageDriverFactory != nil {
		driver, err := opts.StorageDriverFactory(ctx, storage.Config{
			Type:            project.StorageType,
			Bucket:          project.StorageBucket,
			Region:          project.StorageRegion,
			Endpoint:        project.StorageEndpoint,
			AccessKeyID:     project.StorageAccessKey,
			SecretAccessKey: project.StorageSecretKey,
		})
		if err != nil {
			exitCode := ExitCodeStorageError
			logJSON(errOut, "ERROR", "storage driver factory failed", map[string]interface{}{
				"project_iid": projectIID,
				"exit_code":   exitCode,
				"error":       err.Error(),
			})
			return exitCode, fmt.Errorf("storage driver factory error: %w", err)
		}
		storageDriver = driver
	} else {
		driverType := strings.ToLower(strings.TrimSpace(project.StorageType))
		if driverType == "" {
			if project.StorageBucket != "" {
				driverType = "s3"
			} else {
				driverType = "local"
			}
		}

		storageCfg := storage.Config{
			Type:            driverType,
			Bucket:          project.StorageBucket,
			Region:          project.StorageRegion,
			Endpoint:        project.StorageEndpoint,
			AccessKeyID:     project.StorageAccessKey,
			SecretAccessKey: project.StorageSecretKey,
		}

		driver, err := storage.NewDriverFromConfig(ctx, storageCfg)
		if err != nil {
			exitCode := ExitCodeStorageError
			logJSON(errOut, "ERROR", "failed to initialize storage driver from config", map[string]interface{}{
				"project_iid":  projectIID,
				"storage_type": driverType,
				"exit_code":    exitCode,
				"error":        err.Error(),
			})
			return exitCode, fmt.Errorf("storage driver initialization failed: %w", err)
		}
		storageDriver = driver
	}

	// Ping storage driver to validate connection/bucket accessibility
	if err := storageDriver.Ping(ctx); err != nil {
		exitCode := ExitCodeStorageError
		logJSON(errOut, "ERROR", "storage driver ping failed", map[string]interface{}{
			"project_iid":  projectIID,
			"storage_type": storageDriver.Type(),
			"exit_code":    exitCode,
			"error":        err.Error(),
		})
		return exitCode, fmt.Errorf("storage driver ping failed: %w", err)
	}

	// 7. Source Database Connection & Ping
	sourceDBOpener := opts.SourceDBOpener
	if sourceDBOpener == nil {
		sourceDBOpener = collector.OpenSourceDB
	}

	sourceDB, dialect, err := sourceDBOpener(project.SourceDSN)
	if err != nil {
		exitCode := ExitCodeSourceDBError
		logJSON(errOut, "ERROR", "failed to open source database", map[string]interface{}{
			"project_iid": projectIID,
			"exit_code":   exitCode,
			"error":       err.Error(),
		})
		return exitCode, fmt.Errorf("failed to open source database: %w", err)
	}
	if opts.CollectorRunner == nil && sourceDB != nil {
		if err := sourceDB.PingContext(ctx); err != nil {
			exitCode := ExitCodeSourceDBError
			logJSON(errOut, "ERROR", "source database ping failed", map[string]interface{}{
				"project_iid": projectIID,
				"dialect":     string(dialect),
				"exit_code":   exitCode,
				"error":       err.Error(),
			})
			return exitCode, fmt.Errorf("source database ping failed: %w", err)
		}
	}

	// 8. Log sync start
	logJSON(out, "INFO", "starting periscope sync", map[string]interface{}{
		"project_iid":   projectIID,
		"project_name":  project.Name,
		"source_type":   string(dialect),
		"storage_type":  storageDriver.Type(),
		"dry_run":       opts.DryRun,
		"batch_size":    opts.BatchSize,
		"entry_types":   opts.EntryTypes,
		"prune_enabled": pruneEnabled,
	})

	// 9. Execute Collector
	var syncResult *collector.SyncResult
	if opts.CollectorRunner != nil {
		syncResult, err = opts.CollectorRunner(ctx, collector.CollectorConfig{
			ProjectID:     project.ID,
			ProjectIID:    project.IID,
			SourceDB:      sourceDB,
			Dialect:       dialect,
			StorageDriver: storageDriver,
			EntryTypes:    opts.EntryTypes,
			BatchSize:     opts.BatchSize,
			PruneEnabled:  pruneEnabled,
			DryRun:        opts.DryRun,
		})
	} else {
		col, errCol := collector.NewCollector(ctx, collector.CollectorConfig{
			ProjectID:     project.ID,
			ProjectIID:    project.IID,
			SourceDB:      sourceDB,
			Dialect:       dialect,
			StorageDriver: storageDriver,
			EntryTypes:    opts.EntryTypes,
			BatchSize:     opts.BatchSize,
			PruneEnabled:  pruneEnabled,
			DryRun:        opts.DryRun,
		})
		if errCol != nil {
			exitCode := ExitCodeRuntimeError
			logJSON(errOut, "ERROR", "failed to create collector", map[string]interface{}{
				"project_iid": projectIID,
				"exit_code":   exitCode,
				"error":       errCol.Error(),
			})
			return exitCode, fmt.Errorf("failed to create collector: %w", errCol)
		}
		defer col.Close()

		syncResult, err = col.Sync(ctx)
	}

	if err != nil {
		exitCode := ExitCodeRuntimeError
		errStr := strings.ToLower(err.Error())
		if strings.Contains(errStr, "watermark") {
			exitCode = ExitCodeWatermarkError
		} else if strings.Contains(errStr, "storage") || strings.Contains(errStr, "upload") {
			exitCode = ExitCodeStorageError
		} else if strings.Contains(errStr, "source") || strings.Contains(errStr, "poll entries") || strings.Contains(errStr, "fetch tags") {
			exitCode = ExitCodeSourceDBError
		}

		logJSON(errOut, "ERROR", "collector sync failed", map[string]interface{}{
			"project_iid": projectIID,
			"exit_code":   exitCode,
			"error":       err.Error(),
		})
		return exitCode, err
	}

	// 10. Verbose per-batch logging
	if opts.Verbose && syncResult != nil {
		for _, batch := range syncResult.FlushedBatches {
			logJSON(out, "INFO", "batch flushed", map[string]interface{}{
				"project_iid":       projectIID,
				"type":              batch.PartitionKey.Type,
				"records":           batch.RecordCount,
				"min_seq":           batch.MinSequence,
				"max_seq":           batch.MaxSequence,
				"parquet_file":      batch.StorageKey,
				"compressed_bytes":  batch.BytesCompressed,
				"flush_duration_ms": batch.Duration.Milliseconds(),
				"pruned_count":      batch.PrunedCount,
			})
		}
	}

	// 11. Completion logging
	if syncResult != nil {
		logJSON(out, "INFO", "sync completed successfully", map[string]interface{}{
			"project_iid":            projectIID,
			"entries_polled":         syncResult.EntriesPolled,
			"total_records_written":  syncResult.TotalRecordsWritten,
			"total_bytes_compressed": syncResult.TotalBytesCompressed,
			"total_pruned":           syncResult.TotalPruned,
			"flushed_batches_count":  len(syncResult.FlushedBatches),
			"watermarks":             syncResult.Watermarks,
			"duration_ms":            syncResult.Duration.Milliseconds(),
			"dry_run":                opts.DryRun,
		})
	}

	return ExitCodeSuccess, nil
}

// Command-line flag variables
var (
	flagDryRun     bool
	flagBatchSize  int
	flagEntryTypes string
	flagTimeout    time.Duration
	flagVerbose    bool
	flagForcePrune bool
	flagNoPrune    bool
	flagStorageDir string
)

var syncCmd = &cobra.Command{
	Use:     "sync <project_iid>",
	Aliases: []string{"telescope:sync"},
	Short:   "Incrementally sync Laravel Telescope telemetry data to Parquet lakehouse",
	Long:    `Drain telemetry entries from Laravel database into ZSTD-compressed Hive-partitioned Parquet files in S3/MinIO/Local storage and checkpoint watermarks.`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 1 {
			logJSON(cmd.ErrOrStderr(), "ERROR", fmt.Sprintf("accepts exactly 1 arg(s), received %d", len(args)), map[string]interface{}{
				"exit_code": ExitCodeConfigError,
				"args":      args,
			})
			osExit(ExitCodeConfigError)
			return
		}

		dryRun, _ := cmd.Flags().GetBool("dry-run")
		batchSize, _ := cmd.Flags().GetInt("batch-size")
		entryTypesStr, _ := cmd.Flags().GetString("entry-types")
		timeout, _ := cmd.Flags().GetDuration("timeout")
		verbose, _ := cmd.Flags().GetBool("verbose")
		forcePrune, _ := cmd.Flags().GetBool("force-prune")
		noPrune, _ := cmd.Flags().GetBool("no-prune")
		storageDir, _ := cmd.Flags().GetString("storage-dir")

		if timeout <= 0 {
			timeout = 10 * time.Minute
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
		defer cancel()

		var entryTypes []string
		if strings.TrimSpace(entryTypesStr) != "" {
			for _, t := range strings.Split(entryTypesStr, ",") {
				t = strings.TrimSpace(t)
				if t != "" {
					entryTypes = append(entryTypes, t)
				}
			}
		}

		opts := SyncOptions{
			ProjectIID: args[0],
			DryRun:     dryRun,
			BatchSize:  batchSize,
			EntryTypes: entryTypes,
			Timeout:    timeout,
			Verbose:    verbose,
			ForcePrune: forcePrune,
			NoPrune:    noPrune,
			StorageDir: storageDir,
			Out:        cmd.OutOrStdout(),
			ErrOut:     cmd.ErrOrStderr(),
		}

		code, _ := RunSync(ctx, opts)
		if code != ExitCodeSuccess {
			osExit(code)
		}
	},
}

func init() {
	syncCmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "Simulate draining without persisting Parquet, watermarks, or pruning")
	syncCmd.Flags().IntVar(&flagBatchSize, "batch-size", 10000, "Override batch row limit (default 10000)")
	syncCmd.Flags().StringVar(&flagEntryTypes, "entry-types", "", "Comma-separated list of entry types to sync (default all)")
	syncCmd.Flags().DurationVar(&flagTimeout, "timeout", 10*time.Minute, "Timeout for the sync operation (default 10m)")
	syncCmd.Flags().BoolVarP(&flagVerbose, "verbose", "v", false, "Enable verbose per-batch diagnostic logging")
	syncCmd.Flags().BoolVar(&flagForcePrune, "force-prune", false, "Force source table pruning even if project.prune_enabled is false")
	syncCmd.Flags().BoolVar(&flagNoPrune, "no-prune", false, "Bypass pruning even if project.prune_enabled is true")
	syncCmd.Flags().StringVar(&flagStorageDir, "storage-dir", "", "Local directory override for local storage driver")

	rootCmd.AddCommand(syncCmd)
}
