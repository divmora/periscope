package collector

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// Dialect represents the SQL dialect of the source Laravel Telescope database.
type Dialect string

const (
	DialectMySQL    Dialect = "mysql"
	DialectPostgres Dialect = "postgres"
)

// DetectDialect inspects the DSN string to determine if it is PostgreSQL or MySQL.
func DetectDialect(dsn string) Dialect {
	trimmed := strings.TrimSpace(dsn)
	if strings.HasPrefix(trimmed, "postgres://") || strings.HasPrefix(trimmed, "postgresql://") {
		return DialectPostgres
	}
	return DialectMySQL
}

// OpenSourceDB opens a SQL connection to the source Laravel database using the appropriate driver.
func OpenSourceDB(dsn string) (*sql.DB, Dialect, error) {
	dialect := DetectDialect(dsn)
	driverName := "mysql"
	cleanDSN := dsn

	if dialect == DialectPostgres {
		driverName = "pgx"
	} else {
		// Ensure parseTime=true is present for MySQL if not already specified
		cleanDSN = strings.TrimPrefix(cleanDSN, "mysql://")
		if !strings.Contains(cleanDSN, "parseTime=") {
			if strings.Contains(cleanDSN, "?") {
				cleanDSN += "&parseTime=true"
			} else {
				cleanDSN += "?parseTime=true"
			}
		}
	}

	db, err := sql.Open(driverName, cleanDSN)
	if err != nil {
		return nil, dialect, fmt.Errorf("failed to open source database (%s): %w", dialect, err)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(10 * time.Minute)

	return db, dialect, nil
}

// PollEntries queries the source Laravel database for new Telescope entries starting after lastSequence.
func PollEntries(ctx context.Context, db *sql.DB, dialect Dialect, lastSequence int64, batchSize int, entryTypes []string) ([]TelescopeEntry, error) {
	if batchSize <= 0 {
		batchSize = 10000
	}

	var query string
	var args []interface{}

	if dialect == DialectPostgres {
		query = "SELECT sequence, uuid::text, batch_id::text, family_hash, should_display_on_index, type, content, created_at FROM telescope_entries WHERE sequence > $1"
		args = append(args, lastSequence)
		argIdx := 2

		if len(entryTypes) > 0 {
			placeholders := make([]string, len(entryTypes))
			for i, t := range entryTypes {
				placeholders[i] = fmt.Sprintf("$%d", argIdx)
				args = append(args, t)
				argIdx++
			}
			query += fmt.Sprintf(" AND type IN (%s)", strings.Join(placeholders, ", "))
		}

		query += fmt.Sprintf(" ORDER BY sequence ASC LIMIT $%d", argIdx)
		args = append(args, batchSize)
	} else {
		query = "SELECT sequence, uuid, batch_id, family_hash, should_display_on_index, type, content, created_at FROM telescope_entries WHERE sequence > ?"
		args = append(args, lastSequence)

		if len(entryTypes) > 0 {
			placeholders := make([]string, len(entryTypes))
			for i, t := range entryTypes {
				placeholders[i] = "?"
				args = append(args, t)
			}
			query += fmt.Sprintf(" AND type IN (%s)", strings.Join(placeholders, ", "))
		}

		query += " ORDER BY sequence ASC LIMIT ?"
		args = append(args, batchSize)
	}

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query telescope entries: %w", err)
	}
	defer rows.Close()

	var entries []TelescopeEntry
	for rows.Next() {
		var entry TelescopeEntry
		var rawUUID, rawBatchID interface{}
		var familyHash sql.NullString
		var shouldDisplay interface{}
		var rawCreatedAt interface{}

		err := rows.Scan(
			&entry.Sequence,
			&rawUUID,
			&rawBatchID,
			&familyHash,
			&shouldDisplay,
			&entry.Type,
			&entry.Content,
			&rawCreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan telescope entry row: %w", err)
		}

		entry.UUID = parseString(rawUUID)
		entry.BatchID = parseString(rawBatchID)
		if familyHash.Valid {
			entry.FamilyHash = &familyHash.String
		}
		entry.ShouldDisplayOnIndex = parseBool(shouldDisplay)
		entry.CreatedAt = parseTime(rawCreatedAt)
		entry.Tags = []string{}

		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating telescope entries: %w", err)
	}

	return entries, nil
}

// FetchTagsForEntries aggregates tags from telescope_entries_tags in batches and attaches them to entries.
func FetchTagsForEntries(ctx context.Context, db *sql.DB, dialect Dialect, entries []TelescopeEntry) error {
	if len(entries) == 0 {
		return nil
	}

	const chunkSize = 1000
	tagMap := make(map[string][]string, len(entries))

	for i := 0; i < len(entries); i += chunkSize {
		end := i + chunkSize
		if end > len(entries) {
			end = len(entries)
		}
		chunk := entries[i:end]

		uuids := make([]interface{}, len(chunk))
		for j, entry := range chunk {
			uuids[j] = entry.UUID
		}

		var query string
		if dialect == DialectPostgres {
			placeholders := make([]string, len(uuids))
			for j := range uuids {
				placeholders[j] = fmt.Sprintf("$%d", j+1)
			}
			query = fmt.Sprintf("SELECT entry_uuid::text, tag FROM telescope_entries_tags WHERE entry_uuid::text IN (%s)", strings.Join(placeholders, ", "))
		} else {
			placeholders := make([]string, len(uuids))
			for j := range uuids {
				placeholders[j] = "?"
			}
			query = fmt.Sprintf("SELECT entry_uuid, tag FROM telescope_entries_tags WHERE entry_uuid IN (%s)", strings.Join(placeholders, ", "))
		}

		rows, err := db.QueryContext(ctx, query, uuids...)
		if err != nil {
			return fmt.Errorf("failed to fetch tags chunk: %w", err)
		}

		for rows.Next() {
			var rawUUID interface{}
			var tag string
			if err := rows.Scan(&rawUUID, &tag); err != nil {
				rows.Close()
				return fmt.Errorf("failed to scan tag row: %w", err)
			}
			uuidStr := parseString(rawUUID)
			tagMap[uuidStr] = append(tagMap[uuidStr], tag)
		}
		rows.Close()

		if err := rows.Err(); err != nil {
			return fmt.Errorf("error iterating tag rows: %w", err)
		}
	}

	for i := range entries {
		if tags, exists := tagMap[entries[i].UUID]; exists {
			entries[i].Tags = tags
		} else {
			entries[i].Tags = []string{}
		}
	}

	return nil
}

// Helpers for scanning various database driver formats
func parseString(val interface{}) string {
	switch v := val.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	case fmt.Stringer:
		return v.String()
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", v)
	}
}

func parseBool(val interface{}) bool {
	switch v := val.(type) {
	case bool:
		return v
	case int64:
		return v != 0
	case int32:
		return v != 0
	case int:
		return v != 0
	case []byte:
		str := string(v)
		b, _ := strconv.ParseBool(str)
		return b
	case string:
		b, _ := strconv.ParseBool(v)
		return b
	default:
		return false
	}
}

func parseTime(val interface{}) time.Time {
	switch v := val.(type) {
	case time.Time:
		return v.UTC()
	case string:
		// Attempt parsing common SQL datetime layouts
		layouts := []string{
			time.RFC3339Nano,
			time.RFC3339,
			"2006-01-02 15:04:05.999999",
			"2006-01-02 15:04:05",
			"2006-01-02T15:04:05Z",
		}
		for _, layout := range layouts {
			if t, err := time.Parse(layout, v); err == nil {
				return t.UTC()
			}
		}
		return time.Now().UTC()
	case []byte:
		return parseTime(string(v))
	default:
		return time.Now().UTC()
	}
}
