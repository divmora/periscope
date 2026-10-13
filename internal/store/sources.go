package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/divmora/periscope/internal/crypto"
	"github.com/jackc/pgx/v5"
)

type ProjectSource struct {
	ID         int            `json:"-"`
	IID        string         `json:"iid"`
	ProjectIID string         `json:"project_iid"`
	Name       string         `json:"name"`
	SourceType string         `json:"source_type"` // 'laravel_telescope', 'aurora_das', 'postgres_audit', 'opentelemetry', 'generic_db'
	Status     string         `json:"status"`      // 'active', 'paused', 'error'
	DSN        string         `json:"dsn,omitempty"`
	Config     map[string]any `json:"config"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

func ListProjectSources(ctx context.Context, projectIID string) ([]ProjectSource, error) {
	rows, err := Pool.Query(ctx, `
		SELECT ps.id, ps.iid, p.iid, ps.name, ps.source_type, ps.status, ps.config, ps.created_at, ps.updated_at
		FROM project_sources ps
		JOIN projects p ON p.id = ps.project_id
		WHERE p.iid = $1
		ORDER BY ps.created_at ASC
	`, projectIID)
	if err != nil {
		return nil, fmt.Errorf("failed to query project sources: %w", err)
	}
	defer rows.Close()

	var sources []ProjectSource
	for rows.Next() {
		var s ProjectSource
		var rawConfig []byte
		if err := rows.Scan(&s.ID, &s.IID, &s.ProjectIID, &s.Name, &s.SourceType, &s.Status, &rawConfig, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan project source: %w", err)
		}
		if len(rawConfig) > 0 {
			_ = json.Unmarshal(rawConfig, &s.Config)
		}
		if s.Config == nil {
			s.Config = make(map[string]any)
		}
		sources = append(sources, s)
	}
	return sources, nil
}

func CreateProjectSource(ctx context.Context, projectIID string, s *ProjectSource) error {
	var encDSN string
	if s.DSN != "" {
		var err error
		encDSN, err = crypto.Encrypt(s.DSN)
		if err != nil {
			return fmt.Errorf("failed to encrypt source DSN: %w", err)
		}
	}

	configBytes, err := json.Marshal(s.Config)
	if err != nil {
		configBytes = []byte("{}")
	}

	if s.Status == "" {
		s.Status = "active"
	}

	err = Pool.QueryRow(ctx, `
		INSERT INTO project_sources (project_id, name, source_type, status, encrypted_dsn, config)
		SELECT p.id, $2, $3, $4, NULLIF($5, ''), $6::jsonb
		FROM projects p
		WHERE p.iid = $1
		RETURNING iid, created_at, updated_at
	`, projectIID, s.Name, s.SourceType, s.Status, encDSN, string(configBytes)).Scan(&s.IID, &s.CreatedAt, &s.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to insert project source: %w", err)
	}

	s.ProjectIID = projectIID
	return nil
}

func DeleteProjectSource(ctx context.Context, projectIID string, sourceIID string) error {
	res, err := Pool.Exec(ctx, `
		DELETE FROM project_sources ps
		USING projects p
		WHERE ps.project_id = p.id AND p.iid = $1 AND ps.iid = $2
	`, projectIID, sourceIID)
	if err != nil {
		return fmt.Errorf("failed to delete project source: %w", err)
	}
	if res.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
