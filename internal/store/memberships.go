package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

var (
	ErrUserNotFound    = errors.New("user not found")
	ErrProjectNotFound = errors.New("project not found")
	ErrMemberNotFound  = errors.New("member not found")
	ErrInvalidRole     = errors.New("invalid role: must be viewer, member, or admin")
)

type ProjectMember struct {
	UserIID   string    `json:"user_iid"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

func isValidProjectRole(role string) bool {
	return role == "viewer" || role == "member" || role == "admin"
}

// GetProjectRole returns the role of the user for a project, or empty string if not a member.
func GetProjectRole(ctx context.Context, userID int, projectIID string) (string, error) {
	var role string
	err := Pool.QueryRow(ctx, `
		SELECT pm.role::text
		FROM project_memberships pm
		JOIN projects p ON p.id = pm.project_id
		WHERE pm.user_id = $1 AND p.iid = $2
	`, userID, projectIID).Scan(&role)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return role, nil
}

// ListProjectMembers lists all members of a project.
func ListProjectMembers(ctx context.Context, projectIID string) ([]ProjectMember, error) {
	rows, err := Pool.Query(ctx, `
		SELECT u.iid, u.username, u.email, pm.role::text, pm.created_at
		FROM project_memberships pm
		JOIN users u ON u.id = pm.user_id
		JOIN projects p ON p.id = pm.project_id
		WHERE p.iid = $1
		ORDER BY u.username ASC
	`, projectIID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []ProjectMember
	for rows.Next() {
		var m ProjectMember
		if err := rows.Scan(&m.UserIID, &m.Username, &m.Email, &m.Role, &m.CreatedAt); err != nil {
			return nil, err
		}
		members = append(members, m)
	}

	if members == nil {
		members = []ProjectMember{}
	}
	return members, nil
}

// AddProjectMember adds or updates a user membership in a project.
// userIdentifier can be an email, username, or user IID UUID.
func AddProjectMember(ctx context.Context, projectIID string, userIdentifier string, role string) (*ProjectMember, error) {
	if !isValidProjectRole(role) {
		return nil, ErrInvalidRole
	}

	// 1. Find user by email, username, or iid
	var internalUserID int
	var member ProjectMember
	err := Pool.QueryRow(ctx, `
		SELECT id, iid, username, email 
		FROM users 
		WHERE email = $1 OR username = $1 OR iid::text = $1
	`, userIdentifier).Scan(&internalUserID, &member.UserIID, &member.Username, &member.Email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to find user: %w", err)
	}

	// 2. Find internal project ID
	var internalProjectID int
	err = Pool.QueryRow(ctx, `
		SELECT id FROM projects WHERE iid = $1
	`, projectIID).Scan(&internalProjectID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrProjectNotFound
		}
		return nil, fmt.Errorf("failed to find project: %w", err)
	}

	// 3. Upsert membership
	var createdAt time.Time
	var assignedRole string
	err = Pool.QueryRow(ctx, `
		INSERT INTO project_memberships (project_id, user_id, role)
		VALUES ($1, $2, $3::project_role)
		ON CONFLICT (project_id, user_id)
		DO UPDATE SET role = EXCLUDED.role
		RETURNING role::text, created_at
	`, internalProjectID, internalUserID, role).Scan(&assignedRole, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("failed to assign project membership: %w", err)
	}

	member.Role = assignedRole
	member.CreatedAt = createdAt
	return &member, nil
}

// UpdateProjectMemberRole updates the role of an existing project member.
func UpdateProjectMemberRole(ctx context.Context, projectIID string, userIID string, role string) error {
	if !isValidProjectRole(role) {
		return ErrInvalidRole
	}

	res, err := Pool.Exec(ctx, `
		UPDATE project_memberships pm
		SET role = $3::project_role
		FROM projects p, users u
		WHERE pm.project_id = p.id
		  AND pm.user_id = u.id
		  AND p.iid = $1
		  AND u.iid = $2
	`, projectIID, userIID, role)
	if err != nil {
		return err
	}

	if res.RowsAffected() == 0 {
		return ErrMemberNotFound
	}
	return nil
}

// RemoveProjectMember removes a member from a project.
func RemoveProjectMember(ctx context.Context, projectIID string, userIID string) error {
	res, err := Pool.Exec(ctx, `
		DELETE FROM project_memberships pm
		USING projects p, users u
		WHERE pm.project_id = p.id
		  AND pm.user_id = u.id
		  AND p.iid = $1
		  AND u.iid = $2
	`, projectIID, userIID)
	if err != nil {
		return err
	}

	if res.RowsAffected() == 0 {
		return ErrMemberNotFound
	}
	return nil
}
