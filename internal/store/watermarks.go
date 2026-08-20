package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Watermark represents a checkpoint record for a specific project and entry type.
type Watermark struct {
	ProjectID    int       `json:"project_id"`
	EntryType    string    `json:"entry_type"`
	LastSequence int64     `json:"last_sequence"`
	LastSyncedAt time.Time `json:"last_synced_at"`
}

// GetWatermark queries the last_sequence for the specified project and entry type.
// If no watermark record exists, it returns 0 and a nil error.
func GetWatermark(ctx context.Context, projectID int, entryType string) (int64, error) {
	if Pool == nil {
		return 0, fmt.Errorf("database pool is not initialized")
	}

	var lastSequence int64
	err := Pool.QueryRow(ctx, `
		SELECT last_sequence 
		FROM telescope_watermarks 
		WHERE project_id = $1 AND entry_type = $2
	`, projectID, entryType).Scan(&lastSequence)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to query watermark for project %d, type %s: %w", projectID, entryType, err)
	}

	return lastSequence, nil
}

// UpdateWatermark upserts the watermark for a given project and entry type.
// It uses GREATEST to guarantee monotonic progression of last_sequence.
func UpdateWatermark(ctx context.Context, projectID int, entryType string, sequence int64) error {
	if Pool == nil {
		return fmt.Errorf("database pool is not initialized")
	}

	_, err := Pool.Exec(ctx, `
		INSERT INTO telescope_watermarks (project_id, entry_type, last_sequence, last_synced_at)
		VALUES ($1, $2, $3, CURRENT_TIMESTAMP)
		ON CONFLICT (project_id, entry_type)
		DO UPDATE SET 
			last_sequence = GREATEST(telescope_watermarks.last_sequence, EXCLUDED.last_sequence),
			last_synced_at = CURRENT_TIMESTAMP
	`, projectID, entryType, sequence)

	if err != nil {
		return fmt.Errorf("failed to update watermark for project %d, type %s to sequence %d: %w", projectID, entryType, sequence, err)
	}

	return nil
}

// GetAllWatermarks returns all watermarks for a given project mapped by entry type.
func GetAllWatermarks(ctx context.Context, projectID int) (map[string]int64, error) {
	if Pool == nil {
		return nil, fmt.Errorf("database pool is not initialized")
	}

	rows, err := Pool.Query(ctx, `
		SELECT entry_type, last_sequence 
		FROM telescope_watermarks 
		WHERE project_id = $1
	`, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to query watermarks for project %d: %w", projectID, err)
	}
	defer rows.Close()

	watermarks := make(map[string]int64)
	for rows.Next() {
		var entryType string
		var lastSeq int64
		if err := rows.Scan(&entryType, &lastSeq); err != nil {
			return nil, fmt.Errorf("failed to scan watermark row: %w", err)
		}
		watermarks[entryType] = lastSeq
	}

	return watermarks, nil
}

// GetMaxWatermark returns the highest last_sequence across all entry types for a given project.
// If no watermarks exist, it returns 0 and a nil error.
func GetMaxWatermark(ctx context.Context, projectID int) (int64, error) {
	if Pool == nil {
		return 0, fmt.Errorf("database pool is not initialized")
	}

	var maxSeq *int64
	err := Pool.QueryRow(ctx, `
		SELECT MAX(last_sequence) 
		FROM telescope_watermarks 
		WHERE project_id = $1
	`, projectID).Scan(&maxSeq)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to query max watermark for project %d: %w", projectID, err)
	}

	if maxSeq == nil {
		return 0, nil
	}

	return *maxSeq, nil
}
