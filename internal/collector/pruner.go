package collector

import (
	"context"
	"database/sql"
	"fmt"
)

// PruneSource deletes records and associated tags from the source Laravel Telescope database
// for records with sequence <= maxSequence.
// Only called after Parquet upload is confirmed and watermark is successfully checkpointed.
func PruneSource(ctx context.Context, db *sql.DB, dialect Dialect, entryType string, maxSequence int64) (int64, error) {
	if maxSequence <= 0 {
		return 0, nil
	}
	if db == nil {
		return 0, fmt.Errorf("source database connection is nil")
	}

	if dialect == DialectPostgres {
		return prunePostgres(ctx, db, entryType, maxSequence)
	}
	return pruneMySQL(ctx, db, entryType, maxSequence)
}

func prunePostgres(ctx context.Context, db *sql.DB, entryType string, maxSequence int64) (int64, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction for postgres pruning: %w", err)
	}
	defer tx.Rollback()

	if entryType != "" {
		// Delete tags first
		_, err = tx.ExecContext(ctx, `
			DELETE FROM telescope_entries_tags 
			WHERE entry_uuid IN (
				SELECT uuid FROM telescope_entries WHERE type = $1 AND sequence <= $2
			)
		`, entryType, maxSequence)
		if err != nil {
			return 0, fmt.Errorf("failed to delete tags in postgres source: %w", err)
		}

		// Delete entries
		res, err := tx.ExecContext(ctx, `
			DELETE FROM telescope_entries 
			WHERE type = $1 AND sequence <= $2
		`, entryType, maxSequence)
		if err != nil {
			return 0, fmt.Errorf("failed to delete entries in postgres source: %w", err)
		}

		if err := tx.Commit(); err != nil {
			return 0, fmt.Errorf("failed to commit postgres prune transaction: %w", err)
		}

		rowsDeleted, _ := res.RowsAffected()
		return rowsDeleted, nil
	}

	// If entryType is empty, prune all types up to maxSequence
	_, err = tx.ExecContext(ctx, `
		DELETE FROM telescope_entries_tags 
		WHERE entry_uuid IN (
			SELECT uuid FROM telescope_entries WHERE sequence <= $1
		)
	`, maxSequence)
	if err != nil {
		return 0, fmt.Errorf("failed to delete tags in postgres source: %w", err)
	}

	res, err := tx.ExecContext(ctx, `
		DELETE FROM telescope_entries 
		WHERE sequence <= $1
	`, maxSequence)
	if err != nil {
		return 0, fmt.Errorf("failed to delete entries in postgres source: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit postgres prune transaction: %w", err)
	}

	rowsDeleted, _ := res.RowsAffected()
	return rowsDeleted, nil
}

func pruneMySQL(ctx context.Context, db *sql.DB, entryType string, maxSequence int64) (int64, error) {
	var query string
	var args []interface{}

	if entryType != "" {
		query = `
			DELETE t, e
			FROM telescope_entries e
			LEFT JOIN telescope_entries_tags t ON e.uuid = t.entry_uuid
			WHERE e.type = ? AND e.sequence <= ?
		`
		args = []interface{}{entryType, maxSequence}
	} else {
		query = `
			DELETE t, e
			FROM telescope_entries e
			LEFT JOIN telescope_entries_tags t ON e.uuid = t.entry_uuid
			WHERE e.sequence <= ?
		`
		args = []interface{}{maxSequence}
	}

	res, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to execute mysql multi-table delete: %w", err)
	}

	rowsDeleted, _ := res.RowsAffected()
	return rowsDeleted, nil
}
