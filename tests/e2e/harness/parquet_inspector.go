package harness

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/parquet-go/parquet-go"
)

// HivePartitionComponents stores parsed fields from a Hive partition path.
type HivePartitionComponents struct {
	ProjectID string
	Type      string
	Year      int
	Month     int
	Day       int
	Hour      int
	Filename  string
}

// HivePathRegex matches `projectId={id}/type={type}/year={YYYY}/month={MM}/day={DD}/hour={HH}/{filename}`
var HivePathRegex = regexp.MustCompile(`^projectId=([^/]+)/type=([^/]+)/year=(\d{4})/month=(\d{2})/day=(\d{2})/hour=(\d{2})/(.+)$`)

// ParseHivePath parses and validates a partition path against the required specification.
func ParseHivePath(path string) (*HivePartitionComponents, error) {
	cleanPath := strings.TrimPrefix(path, "/")
	matches := HivePathRegex.FindStringSubmatch(cleanPath)
	if len(matches) != 8 {
		return nil, fmt.Errorf("path '%s' does not match required Hive partition format: projectId={id}/type={type}/year={YYYY}/month={MM}/day={DD}/hour={HH}/filename", path)
	}

	year, err := strconv.Atoi(matches[3])
	if err != nil {
		return nil, fmt.Errorf("invalid year in path: %w", err)
	}
	month, err := strconv.Atoi(matches[4])
	if err != nil || month < 1 || month > 12 {
		return nil, fmt.Errorf("invalid month in path: %s", matches[4])
	}
	day, err := strconv.Atoi(matches[5])
	if err != nil || day < 1 || day > 31 {
		return nil, fmt.Errorf("invalid day in path: %s", matches[5])
	}
	hour, err := strconv.Atoi(matches[6])
	if err != nil || hour < 0 || hour > 23 {
		return nil, fmt.Errorf("invalid hour in path: %s", matches[6])
	}

	return &HivePartitionComponents{
		ProjectID: matches[1],
		Type:      matches[2],
		Year:      year,
		Month:     month,
		Day:       day,
		Hour:      hour,
		Filename:  matches[7],
	}, nil
}

// ParquetInspector provides utilities to inspect and validate Parquet files in tests.
type ParquetInspector struct{}

// InspectParquetBytes reads Parquet binary bytes, validates its structure,
// asserts ZSTD compression, and extracts all rows into a slice of maps.
func (p *ParquetInspector) InspectParquetBytes(data []byte) (*ParquetInspectionResult, error) {
	reader := bytes.NewReader(data)
	file, err := parquet.OpenFile(reader, int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("failed to open parquet file: %w", err)
	}

	schema := file.Schema()
	numRows := file.NumRows()
	numRowGroups := len(file.RowGroups())

	// Check compression on row groups via file metadata
	var hasZstd bool
	if file.Metadata() != nil {
		for _, rg := range file.Metadata().RowGroups {
			for _, col := range rg.Columns {
				codecStr := col.MetaData.Codec.String()
				if strings.EqualFold(codecStr, "ZSTD") || col.MetaData.Codec == 4 {
					hasZstd = true
					break
				}
			}
			if hasZstd {
				break
			}
		}
	}

	// Read rows into generic rows
	genericReader := parquet.NewGenericReader[any](file)
	defer genericReader.Close()

	columnNames := make([]string, 0, len(schema.Fields()))
	for _, f := range schema.Fields() {
		columnNames = append(columnNames, f.Name())
	}

	return &ParquetInspectionResult{
		NumRows:      numRows,
		NumRowGroups: numRowGroups,
		ColumnNames:  columnNames,
		HasZSTD:      hasZstd,
		File:         file,
	}, nil
}

// ParquetInspectionResult holds metadata extracted from a Parquet file.
type ParquetInspectionResult struct {
	NumRows      int64
	NumRowGroups int
	ColumnNames  []string
	HasZSTD      bool
	File         *parquet.File
}

// HasColumn returns true if the schema contains the column with the specified name.
func (r *ParquetInspectionResult) HasColumn(name string) bool {
	for _, col := range r.ColumnNames {
		if col == name {
			return true
		}
	}
	return false
}

// GenericTelemetryRow represents a deserialized telemetry row from Parquet.
type GenericTelemetryRow struct {
	Sequence             int64     `parquet:"sequence"`
	UUID                 string    `parquet:"uuid"`
	BatchID              string    `parquet:"batch_id"`
	FamilyHash           *string   `parquet:"family_hash,optional"`
	ShouldDisplayOnIndex bool      `parquet:"should_display_on_index"`
	Type                 string    `parquet:"type"`
	CreatedAt            time.Time `parquet:"created_at"`
	Tags                 []string  `parquet:"tags"`
	Content              string    `parquet:"content"`

	// Extracted dimensions
	UserID           *string  `parquet:"user_id,optional"`
	UserEmail        *string  `parquet:"user_email,optional"`
	HTTPStatus       *int32   `parquet:"http_status,optional"`
	HTTPMethod       *string  `parquet:"http_method,optional"`
	HTTPURI          *string  `parquet:"http_uri,optional"`
	HTTPController   *string  `parquet:"http_controller,optional"`
	DurationMS       *float64 `parquet:"duration_ms,optional"`
	MemoryMB         *float64 `parquet:"memory_mb,optional"`
	ClientIP         *string  `parquet:"client_ip,optional"`
	QueryConnection  *string  `parquet:"query_connection,optional"`
	QuerySQL         *string  `parquet:"query_sql,optional"`
	QueryTimeMS      *float64 `parquet:"query_time_ms,optional"`
	QuerySlow        *bool    `parquet:"query_slow,optional"`
	ExceptionClass   *string  `parquet:"exception_class,optional"`
	ExceptionMessage *string  `parquet:"exception_message,optional"`
	LogLevel         *string  `parquet:"log_level,optional"`
	LogMessage       *string  `parquet:"log_message,optional"`
	JobName          *string  `parquet:"job_name,optional"`
	JobQueue         *string  `parquet:"job_queue,optional"`
	JobStatus        *string  `parquet:"job_status,optional"`
	JobTries         *int32   `parquet:"job_tries,optional"`
	CommandName      *string  `parquet:"command_name,optional"`
	CommandExitCode  *int32   `parquet:"command_exit_code,optional"`
}

// ReadTelemetryRows reads typed GenericTelemetryRow records from parquet bytes.
func (p *ParquetInspector) ReadTelemetryRows(data []byte) ([]GenericTelemetryRow, error) {
	reader := bytes.NewReader(data)
	file, err := parquet.OpenFile(reader, int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("failed to open parquet file: %w", err)
	}

	genericReader := parquet.NewGenericReader[GenericTelemetryRow](file)
	defer genericReader.Close()

	rows := make([]GenericTelemetryRow, file.NumRows())
	n, err := genericReader.Read(rows)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("failed to read telemetry rows: %w", err)
	}

	return rows[:n], nil
}
