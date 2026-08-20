package store

import (
	"context"
	"fmt"
	"time"

	"github.com/divmora/periscope/internal/crypto"
	"github.com/jackc/pgx/v5"
)

type Project struct {
	ID              int       `json:"-"`   // Internal DB primary key for FK relations & watermarks
	IID             string    `json:"iid"` // We expose iid as iid to the API
	Name            string    `json:"name"`
	StorageType     string    `json:"storage_type"`
	StorageBucket   string    `json:"storage_bucket"`
	StorageRegion   string    `json:"storage_region"`
	StorageEndpoint string    `json:"storage_endpoint"`
	PruneEnabled    bool      `json:"prune_enabled"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	// These fields are internal and should only be returned when explicitly requesting config
	SourceDSN        string `json:"source_dsn,omitempty"`
	StorageAccessKey string `json:"storage_access_key,omitempty"`
	StorageSecretKey string `json:"storage_secret_key,omitempty"`
}

func ListProjects(ctx context.Context, userID int, globalRole string) ([]Project, error) {
	var rows pgx.Rows
	var err error

	if globalRole == "admin" {
		rows, err = Pool.Query(ctx, `
			SELECT id, iid, name, storage_type, storage_bucket, storage_region, storage_endpoint, prune_enabled, created_at, updated_at
			FROM projects
			ORDER BY name ASC
		`)
	} else {
		rows, err = Pool.Query(ctx, `
			SELECT p.id, p.iid, p.name, p.storage_type, p.storage_bucket, p.storage_region, p.storage_endpoint, p.prune_enabled, p.created_at, p.updated_at
			FROM projects p
			JOIN project_memberships pm ON p.id = pm.project_id
			WHERE pm.user_id = $1
			ORDER BY p.name ASC
		`, userID)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.IID, &p.Name, &p.StorageType, &p.StorageBucket, &p.StorageRegion, &p.StorageEndpoint, &p.PruneEnabled, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, nil
}

func CheckProjectAccess(ctx context.Context, userID int, projectIID string) (bool, error) {
	var exists bool
	err := Pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM project_memberships pm
			JOIN projects p ON p.id = pm.project_id
			WHERE pm.user_id = $1 AND p.iid = $2
		)
	`, userID, projectIID).Scan(&exists)
	return exists, err
}

func GetProject(ctx context.Context, iid string) (*Project, error) {
	var p Project
	var encDSN, encAccess, encSecret string

	err := Pool.QueryRow(ctx, `
		SELECT id, iid, name, encrypted_source_dsn, storage_type, storage_bucket, storage_region, storage_endpoint, 
		       encrypted_storage_access_key, encrypted_storage_secret_key, prune_enabled, created_at, updated_at
		FROM projects
		WHERE iid = $1
	`, iid).Scan(&p.ID, &p.IID, &p.Name, &encDSN, &p.StorageType, &p.StorageBucket, &p.StorageRegion, &p.StorageEndpoint,
		&encAccess, &encSecret, &p.PruneEnabled, &p.CreatedAt, &p.UpdatedAt)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	// Decrypt sensitive fields
	p.SourceDSN, _ = crypto.Decrypt(encDSN)
	p.StorageAccessKey, _ = crypto.Decrypt(encAccess)
	p.StorageSecretKey, _ = crypto.Decrypt(encSecret)

	return &p, nil
}

// GetProjectByIID is an alias for GetProject.
func GetProjectByIID(ctx context.Context, iid string) (*Project, error) {
	return GetProject(ctx, iid)
}

func CreateProject(ctx context.Context, userID int, p *Project) error {
	encDSN, err := crypto.Encrypt(p.SourceDSN)
	if err != nil {
		return fmt.Errorf("failed to encrypt DSN: %w", err)
	}
	encAccess, _ := crypto.Encrypt(p.StorageAccessKey)
	encSecret, _ := crypto.Encrypt(p.StorageSecretKey)

	tx, err := Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var projectID int
	err = tx.QueryRow(ctx, `
		INSERT INTO projects (name, encrypted_source_dsn, storage_type, storage_bucket, storage_region, storage_endpoint, encrypted_storage_access_key, encrypted_storage_secret_key, prune_enabled)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, iid, created_at, updated_at
	`, p.Name, encDSN, p.StorageType, p.StorageBucket, p.StorageRegion, p.StorageEndpoint, encAccess, encSecret, p.PruneEnabled).Scan(&projectID, &p.IID, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return err
	}
	p.ID = projectID

	if userID > 0 {
		_, err = tx.Exec(ctx, `
			INSERT INTO project_memberships (project_id, user_id, role)
			VALUES ($1, $2, 'admin'::project_role)
			ON CONFLICT (project_id, user_id) DO NOTHING
		`, projectID, userID)
		if err != nil {
			return fmt.Errorf("failed to add creator as admin: %w", err)
		}
	}

	return tx.Commit(ctx)
}

func UpdateProject(ctx context.Context, p *Project) error {
	encDSN, err := crypto.Encrypt(p.SourceDSN)
	if err != nil {
		return fmt.Errorf("failed to encrypt DSN: %w", err)
	}
	encAccess, _ := crypto.Encrypt(p.StorageAccessKey)
	encSecret, _ := crypto.Encrypt(p.StorageSecretKey)

	_, err = Pool.Exec(ctx, `
		UPDATE projects SET 
			name = $1, 
			encrypted_source_dsn = $2, 
			storage_type = $3, 
			storage_bucket = $4, 
			storage_region = $5, 
			storage_endpoint = $6, 
			encrypted_storage_access_key = $7, 
			encrypted_storage_secret_key = $8, 
			prune_enabled = $9,
			updated_at = CURRENT_TIMESTAMP
		WHERE iid = $10
	`, p.Name, encDSN, p.StorageType, p.StorageBucket, p.StorageRegion, p.StorageEndpoint, encAccess, encSecret, p.PruneEnabled, p.IID)

	return err
}

func DeleteProject(ctx context.Context, iid string) error {
	res, err := Pool.Exec(ctx, `DELETE FROM projects WHERE iid = $1`, iid)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return ErrProjectNotFound
	}
	return nil
}
