package collector

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/parquet-go/parquet-go"
)

// EncodeParquet serializes a slice of TelemetryRecord structs into a Parquet byte buffer
// with ZSTD compression (Default compression level) and dictionary encoding.
func EncodeParquet(records []TelemetryRecord) ([]byte, error) {
	if len(records) == 0 {
		return nil, errors.New("cannot encode empty records slice to parquet")
	}

	var buf bytes.Buffer
	writer := parquet.NewGenericWriter[TelemetryRecord](
		&buf,
		parquet.Compression(&parquet.Zstd),
	)

	if _, err := writer.Write(records); err != nil {
		_ = writer.Close()
		return nil, fmt.Errorf("failed to write parquet records: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close parquet writer: %w", err)
	}

	return buf.Bytes(), nil
}

// DecodeParquet reads and deserializes Parquet byte data back into a slice of TelemetryRecord structs.
// Useful for verification, testing, and retrieval.
func DecodeParquet(data []byte) ([]TelemetryRecord, error) {
	if len(data) == 0 {
		return nil, errors.New("cannot decode empty parquet data")
	}

	reader := parquet.NewGenericReader[TelemetryRecord](bytes.NewReader(data))
	defer reader.Close()

	numRows := reader.NumRows()
	records := make([]TelemetryRecord, numRows)

	n, err := reader.Read(records)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("failed to read parquet records: %w", err)
	}

	return records[:n], nil
}
