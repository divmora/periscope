package harness

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/divmora/periscope/internal/storage"
	"github.com/google/uuid"
	"github.com/parquet-go/parquet-go"
)

// TelemetryRecord defines the complete Parquet schema layout with ZSTD compression.
type TelemetryRecord struct {
	// Core Telescope Envelope
	Sequence             int64     `parquet:"sequence,delta_binary_packed"`
	UUID                 string    `parquet:"uuid,zstd"`
	BatchID              string    `parquet:"batch_id,zstd"`
	FamilyHash           *string   `parquet:"family_hash,zstd,optional"`
	ShouldDisplayOnIndex bool      `parquet:"should_display_on_index,boolean"`
	Type                 string    `parquet:"type,dict,zstd"`
	CreatedAt            time.Time `parquet:"created_at,timestamp(microsecond)"`
	Tags                 []string  `parquet:"tags,list,zstd"`

	// Common User Dimensions
	UserID    *string `parquet:"user_id,dict,zstd,optional"`
	UserEmail *string `parquet:"user_email,zstd,optional"`
	UserName  *string `parquet:"user_name,zstd,optional"`

	// HTTP Request Dimensions
	HTTPStatus     *int32   `parquet:"http_status,plain,optional"`
	HTTPMethod     *string  `parquet:"http_method,dict,zstd,optional"`
	HTTPURI        *string  `parquet:"http_uri,zstd,optional"`
	HTTPController *string  `parquet:"http_controller,zstd,optional"`
	DurationMS     *float64 `parquet:"duration_ms,plain,optional"`
	MemoryMB       *float64 `parquet:"memory_mb,plain,optional"`
	ClientIP       *string  `parquet:"client_ip,dict,zstd,optional"`

	// SQL Query Dimensions
	QueryConnection *string  `parquet:"query_connection,dict,zstd,optional"`
	QuerySQL        *string  `parquet:"query_sql,zstd,optional"`
	QueryTimeMS     *float64 `parquet:"query_time_ms,plain,optional"`
	QuerySlow       *bool    `parquet:"query_slow,boolean,optional"`
	QueryFile       *string  `parquet:"query_file,zstd,optional"`
	QueryLine       *int32   `parquet:"query_line,plain,optional"`

	// Exception Dimensions
	ExceptionClass   *string `parquet:"exception_class,dict,zstd,optional"`
	ExceptionMessage *string `parquet:"exception_message,zstd,optional"`
	ExceptionFile    *string `parquet:"exception_file,zstd,optional"`
	ExceptionLine    *int32  `parquet:"exception_line,plain,optional"`

	// Job Dimensions
	JobName       *string `parquet:"job_name,dict,zstd,optional"`
	JobQueue      *string `parquet:"job_queue,dict,zstd,optional"`
	JobConnection *string `parquet:"job_connection,dict,zstd,optional"`
	JobStatus     *string `parquet:"job_status,dict,zstd,optional"`
	JobTries      *int32  `parquet:"job_tries,plain,optional"`

	// Command Dimensions
	CommandName     *string `parquet:"command_name,dict,zstd,optional"`
	CommandExitCode *int32  `parquet:"command_exit_code,plain,optional"`

	// Log Dimensions
	LogLevel   *string `parquet:"log_level,dict,zstd,optional"`
	LogMessage *string `parquet:"log_message,zstd,optional"`

	// Full Raw Payload
	Content string `parquet:"content,zstd"`
}

// ExtractDimensions parses an entry's raw JSON and returns a typed TelemetryRecord.
func ExtractDimensions(entry RawTelescopeEntry) TelemetryRecord {
	rec := TelemetryRecord{
		Sequence:             entry.Sequence,
		UUID:                 entry.UUID,
		BatchID:              entry.BatchID,
		FamilyHash:           entry.FamilyHash,
		ShouldDisplayOnIndex: entry.ShouldDisplayOnIndex,
		Type:                 entry.Type,
		CreatedAt:            entry.CreatedAt.UTC(),
		Tags:                 entry.Tags,
		Content:              entry.Content,
	}

	if rec.Tags == nil {
		rec.Tags = []string{}
	}

	var rawMap map[string]any
	if err := json.Unmarshal([]byte(entry.Content), &rawMap); err != nil {
		// Malformed JSON: retain raw Content, leave dimensions nil
		return rec
	}

	// Extract User info if present
	if userVal, ok := rawMap["user"].(map[string]any); ok {
		if id, ok := userVal["id"]; ok && id != nil {
			idStr := fmt.Sprintf("%v", id)
			rec.UserID = &idStr
		}
		if email, ok := userVal["email"].(string); ok && email != "" {
			rec.UserEmail = &email
		}
		if name, ok := userVal["name"].(string); ok && name != "" {
			rec.UserName = &name
		}
	}

	switch entry.Type {
	case "request":
		if st, ok := rawMap["response_status"].(float64); ok {
			stInt := int32(st)
			rec.HTTPStatus = &stInt
		}
		if m, ok := rawMap["method"].(string); ok && m != "" {
			rec.HTTPMethod = &m
		}
		if u, ok := rawMap["uri"].(string); ok && u != "" {
			rec.HTTPURI = &u
		}
		if c, ok := rawMap["controller_action"].(string); ok && c != "" {
			rec.HTTPController = &c
		}
		if d, ok := rawMap["duration"].(float64); ok {
			rec.DurationMS = &d
		}
		if mem, ok := rawMap["memory"].(float64); ok {
			rec.MemoryMB = &mem
		}
		if headers, ok := rawMap["headers"].(map[string]any); ok {
			if ip, ok := headers["x-forwarded-for"].(string); ok && ip != "" {
				rec.ClientIP = &ip
			} else if ip, ok := headers["host"].(string); ok && ip != "" {
				rec.ClientIP = &ip
			}
		}

	case "query":
		if conn, ok := rawMap["connection"].(string); ok && conn != "" {
			rec.QueryConnection = &conn
		}
		if sql, ok := rawMap["sql"].(string); ok && sql != "" {
			rec.QuerySQL = &sql
		}
		if t, ok := rawMap["time"].(float64); ok {
			rec.QueryTimeMS = &t
		}
		if slow, ok := rawMap["slow"].(bool); ok {
			rec.QuerySlow = &slow
		}
		if f, ok := rawMap["file"].(string); ok && f != "" {
			rec.QueryFile = &f
		}
		if l, ok := rawMap["line"].(float64); ok {
			lineInt := int32(l)
			rec.QueryLine = &lineInt
		}

	case "exception":
		if cls, ok := rawMap["class"].(string); ok && cls != "" {
			rec.ExceptionClass = &cls
		}
		if msg, ok := rawMap["message"].(string); ok && msg != "" {
			rec.ExceptionMessage = &msg
		}
		if f, ok := rawMap["file"].(string); ok && f != "" {
			rec.ExceptionFile = &f
		}
		if l, ok := rawMap["line"].(float64); ok {
			lineInt := int32(l)
			rec.ExceptionLine = &lineInt
		}

	case "log":
		if lvl, ok := rawMap["level"].(string); ok && lvl != "" {
			rec.LogLevel = &lvl
		}
		if msg, ok := rawMap["message"].(string); ok && msg != "" {
			rec.LogMessage = &msg
		}

	case "job":
		if n, ok := rawMap["name"].(string); ok && n != "" {
			rec.JobName = &n
		}
		if q, ok := rawMap["queue"].(string); ok && q != "" {
			rec.JobQueue = &q
		}
		if conn, ok := rawMap["connection"].(string); ok && conn != "" {
			rec.JobConnection = &conn
		}
		if st, ok := rawMap["status"].(string); ok && st != "" {
			rec.JobStatus = &st
		}
		if tr, ok := rawMap["tries"].(float64); ok {
			triesInt := int32(tr)
			rec.JobTries = &triesInt
		}

	case "command":
		if cmd, ok := rawMap["command"].(string); ok && cmd != "" {
			rec.CommandName = &cmd
		}
		if ec, ok := rawMap["exit_code"].(float64); ok {
			ecInt := int32(ec)
			rec.CommandExitCode = &ecInt
		}
	}

	return rec
}

// EncodeParquet writes TelemetryRecord records into a ZSTD-compressed Parquet byte stream.
func EncodeParquet(records []TelemetryRecord) ([]byte, error) {
	if len(records) == 0 {
		return nil, fmt.Errorf("cannot encode empty records slice to parquet")
	}

	var buf bytes.Buffer
	writer := parquet.NewGenericWriter[TelemetryRecord](
		&buf,
		parquet.Compression(&parquet.Zstd),
	)

	if _, err := writer.Write(records); err != nil {
		_ = writer.Close()
		return nil, fmt.Errorf("failed writing parquet records: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed closing parquet writer: %w", err)
	}

	return buf.Bytes(), nil
}

// GenerateHivePartitionPath builds the exact Hive partition path and filename.
// Template: `projectId={id}/type={type}/year={YYYY}/month={MM}/day={DD}/hour={HH}/part-{minSeq:012d}-{maxSeq:012d}-{batchUUID}.parquet`
func GenerateHivePartitionPath(projectID, entryType string, t time.Time, minSeq, maxSeq int64, batchUUID string) string {
	utc := t.UTC()
	return fmt.Sprintf("projectId=%s/type=%s/year=%04d/month=%02d/day=%02d/hour=%02d/part-%012d-%012d-%s.parquet",
		projectID,
		entryType,
		utc.Year(),
		int(utc.Month()),
		utc.Day(),
		utc.Hour(),
		minSeq,
		maxSeq,
		batchUUID,
	)
}

// PartitionGroupKey uniquely identifies an hourly partition.
type PartitionGroupKey struct {
	Type  string
	Year  int
	Month int
	Day   int
	Hour  int
}

// SyncResult records statistics for a completed synchronization run.
type SyncResult struct {
	RecordsDrained        int
	ParquetFilesUploaded  int
	UploadedKeys          []string
	WatermarksUpdated     map[string]int64
	RowsPruned            int64
	PartitionPathsCreated []string
}

// RunSyncCycle executes an opaque-box end-to-end synchronization cycle:
// 1. Drains entries where sequence > last_sequence per entry type.
// 2. Extracts dimensions and aggregates tags.
// 3. Partitions into hourly Hive slices.
// 4. Encodes to ZSTD Parquet.
// 5. Uploads to storage driver.
// 6. Confirms upload -> Checkpoints watermark in metadata DB -> Safely prunes source DB.
func RunSyncCycle(
	ctx context.Context,
	sourceDB *MockTelescopeDatabase,
	metaDB *MockMetadataDatabase,
	storageDriver storage.Driver,
	projectID int,
	projectIID string,
	entryTypes []string,
	batchSize int,
	pruneEnabled bool,
) (*SyncResult, error) {
	if len(entryTypes) == 0 {
		entryTypes = AllKnownEntryTypes
	}
	if batchSize <= 0 {
		batchSize = 10000
	}

	result := &SyncResult{
		WatermarksUpdated: make(map[string]int64),
	}

	for _, entryType := range entryTypes {
		if err := ctx.Err(); err != nil {
			return result, err
		}

		lastWatermark, err := metaDB.GetWatermark(projectID, entryType)
		if err != nil {
			return result, fmt.Errorf("failed fetching watermark for type %s: %w", entryType, err)
		}

		rawEntries, err := sourceDB.DrainEntries(entryType, lastWatermark, batchSize)
		if err != nil {
			return result, fmt.Errorf("failed draining source entries for type %s: %w", entryType, err)
		}

		if len(rawEntries) == 0 {
			continue
		}

		result.RecordsDrained += len(rawEntries)

		// Group records into hourly partition buckets
		partitionBuckets := make(map[PartitionGroupKey][]TelemetryRecord)
		for _, raw := range rawEntries {
			rec := ExtractDimensions(raw)
			key := PartitionGroupKey{
				Type:  rec.Type,
				Year:  rec.CreatedAt.Year(),
				Month: int(rec.CreatedAt.Month()),
				Day:   rec.CreatedAt.Day(),
				Hour:  rec.CreatedAt.Hour(),
			}
			partitionBuckets[key] = append(partitionBuckets[key], rec)
		}

		var maxProcessedSequence int64 = lastWatermark

		// Process and upload each partition bucket
		for key, records := range partitionBuckets {
			if len(records) == 0 {
				continue
			}

			// Sort records by sequence within partition
			sort.Slice(records, func(i, j int) bool {
				return records[i].Sequence < records[j].Sequence
			})

			minSeq := records[0].Sequence
			maxSeq := records[len(records)-1].Sequence
			if maxSeq > maxProcessedSequence {
				maxProcessedSequence = maxSeq
			}

			batchID := uuid.NewString()
			partitionTime := time.Date(key.Year, time.Month(key.Month), key.Day, key.Hour, 0, 0, 0, time.UTC)
			s3Key := GenerateHivePartitionPath(projectIID, key.Type, partitionTime, minSeq, maxSeq, batchID)

			parquetData, err := EncodeParquet(records)
			if err != nil {
				return result, fmt.Errorf("failed encoding parquet for %s: %w", s3Key, err)
			}

			// Upload to storage driver
			uploadErr := storageDriver.Upload(ctx, s3Key, bytes.NewReader(parquetData), int64(len(parquetData)), "application/vnd.apache.parquet")
			if uploadErr != nil {
				return result, fmt.Errorf("failed uploading parquet batch to %s: %w", s3Key, uploadErr)
			}

			result.ParquetFilesUploaded++
			result.UploadedKeys = append(result.UploadedKeys, s3Key)
			result.PartitionPathsCreated = append(result.PartitionPathsCreated, s3Key)
		}

		// Two-phase commit: update watermark only after Parquet confirmation
		if maxProcessedSequence > lastWatermark {
			if err := metaDB.UpsertWatermark(projectID, entryType, maxProcessedSequence, time.Now().UTC()); err != nil {
				return result, fmt.Errorf("failed updating watermark for type %s: %w", entryType, err)
			}
			result.WatermarksUpdated[entryType] = maxProcessedSequence

			// If pruning enabled, delete source rows up to maxProcessedSequence
			if pruneEnabled {
				pruned, err := sourceDB.Prune(entryType, maxProcessedSequence)
				if err != nil {
					return result, fmt.Errorf("failed pruning source db for type %s: %w", entryType, err)
				}
				result.RowsPruned += pruned
			}
		}
	}

	return result, nil
}
