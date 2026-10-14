package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

var (
	ErrTeamNotFound = errors.New("team not found")
)

type ProjectTeam struct {
	ID             int       `json:"-"`
	IID            string    `json:"iid"`
	ProjectIID     string    `json:"project_iid"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	Role           string    `json:"role"`            // 'viewer', 'member', 'admin'
	AllowedSources []string  `json:"allowed_sources"` // nil/empty = all sources in project
	MemberCount    int       `json:"member_count"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type TeamMember struct {
	UserIID   string    `json:"user_iid"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

// ListProjectTeams returns all teams defined in a project with their member counts.
func ListProjectTeams(ctx context.Context, projectIID string) ([]ProjectTeam, error) {
	if Pool == nil {
		return nil, fmt.Errorf("database connection pool not initialized")
	}

	rows, err := Pool.Query(ctx, `
		SELECT pt.id, pt.iid, p.iid, pt.name, COALESCE(pt.description, ''), pt.role::text, 
		       COALESCE(pt.allowed_sources, '{}'),
		       (SELECT COUNT(*) FROM project_team_members ptm WHERE ptm.team_id = pt.id) as member_count,
		       pt.created_at, pt.updated_at
		FROM project_teams pt
		JOIN projects p ON p.id = pt.project_id
		WHERE p.iid = $1
		ORDER BY pt.name ASC
	`, projectIID)
	if err != nil {
		return nil, fmt.Errorf("failed to query project teams: %w", err)
	}
	defer rows.Close()

	var teams []ProjectTeam
	for rows.Next() {
		var t ProjectTeam
		if err := rows.Scan(
			&t.ID, &t.IID, &t.ProjectIID, &t.Name, &t.Description, &t.Role,
			&t.AllowedSources, &t.MemberCount, &t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan project team: %w", err)
		}
		teams = append(teams, t)
	}

	if teams == nil {
		teams = []ProjectTeam{}
	}
	return teams, nil
}

// CreateProjectTeam creates a new team with specified role and source access.
func CreateProjectTeam(ctx context.Context, projectIID string, team *ProjectTeam) error {
	if !isValidProjectRole(team.Role) {
		return ErrInvalidRole
	}
	if Pool == nil {
		return fmt.Errorf("database connection pool not initialized")
	}

	var internalProjectID int
	err := Pool.QueryRow(ctx, `SELECT id FROM projects WHERE iid = $1`, projectIID).Scan(&internalProjectID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrProjectNotFound
		}
		return fmt.Errorf("failed to find project: %w", err)
	}

	err = Pool.QueryRow(ctx, `
		INSERT INTO project_teams (project_id, name, description, role, allowed_sources)
		VALUES ($1, $2, $3, $4::project_role, $5)
		RETURNING id, iid, created_at, updated_at
	`, internalProjectID, team.Name, team.Description, team.Role, team.AllowedSources).Scan(
		&team.ID, &team.IID, &team.CreatedAt, &team.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create team: %w", err)
	}

	team.ProjectIID = projectIID
	return nil
}

// UpdateProjectTeam updates team attributes, role, and allowed sources.
func UpdateProjectTeam(ctx context.Context, projectIID string, teamIID string, name, description, role string, allowedSources []string) error {
	if !isValidProjectRole(role) {
		return ErrInvalidRole
	}
	if Pool == nil {
		return fmt.Errorf("database connection pool not initialized")
	}

	res, err := Pool.Exec(ctx, `
		UPDATE project_teams pt
		SET name = $3,
		    description = $4,
		    role = $5::project_role,
		    allowed_sources = $6,
		    updated_at = CURRENT_TIMESTAMP
		FROM projects p
		WHERE pt.project_id = p.id
		  AND p.iid = $1
		  AND pt.iid = $2
	`, projectIID, teamIID, name, description, role, allowedSources)
	if err != nil {
		return err
	}

	if res.RowsAffected() == 0 {
		return ErrTeamNotFound
	}
	return nil
}

// DeleteProjectTeam deletes a team from a project.
func DeleteProjectTeam(ctx context.Context, projectIID string, teamIID string) error {
	if Pool == nil {
		return fmt.Errorf("database connection pool not initialized")
	}

	res, err := Pool.Exec(ctx, `
		DELETE FROM project_teams pt
		USING projects p
		WHERE pt.project_id = p.id
		  AND p.iid = $1
		  AND pt.iid = $2
	`, projectIID, teamIID)
	if err != nil {
		return err
	}

	if res.RowsAffected() == 0 {
		return ErrTeamNotFound
	}
	return nil
}

// ListTeamMembers returns all users belonging to a specific team.
func ListTeamMembers(ctx context.Context, teamIID string) ([]TeamMember, error) {
	if Pool == nil {
		return nil, fmt.Errorf("database connection pool not initialized")
	}

	rows, err := Pool.Query(ctx, `
		SELECT u.iid, u.username, u.email, ptm.created_at
		FROM project_team_members ptm
		JOIN users u ON u.id = ptm.user_id
		JOIN project_teams pt ON pt.id = ptm.team_id
		WHERE pt.iid = $1
		ORDER BY u.username ASC
	`, teamIID)
	if err != nil {
		return nil, fmt.Errorf("failed to query team members: %w", err)
	}
	defer rows.Close()

	var members []TeamMember
	for rows.Next() {
		var m TeamMember
		if err := rows.Scan(&m.UserIID, &m.Username, &m.Email, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan team member: %w", err)
		}
		members = append(members, m)
	}

	if members == nil {
		members = []TeamMember{}
	}
	return members, nil
}

// AddTeamMember adds a user (by username, email, or iid) to a team.
func AddTeamMember(ctx context.Context, teamIID string, userIdentifier string) error {
	if Pool == nil {
		return fmt.Errorf("database connection pool not initialized")
	}

	var internalUserID int
	err := Pool.QueryRow(ctx, `
		SELECT id FROM users WHERE email = $1 OR username = $1 OR iid::text = $1
	`, userIdentifier).Scan(&internalUserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrUserNotFound
		}
		return fmt.Errorf("failed to find user: %w", err)
	}

	var internalTeamID int
	err = Pool.QueryRow(ctx, `SELECT id FROM project_teams WHERE iid = $1`, teamIID).Scan(&internalTeamID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrTeamNotFound
		}
		return fmt.Errorf("failed to find team: %w", err)
	}

	_, err = Pool.Exec(ctx, `
		INSERT INTO project_team_members (team_id, user_id)
		VALUES ($1, $2)
		ON CONFLICT (team_id, user_id) DO NOTHING
	`, internalTeamID, internalUserID)
	if err != nil {
		return fmt.Errorf("failed to add member to team: %w", err)
	}

	return nil
}

// RemoveTeamMember removes a user from a team.
func RemoveTeamMember(ctx context.Context, teamIID string, userIID string) error {
	if Pool == nil {
		return fmt.Errorf("database connection pool not initialized")
	}

	res, err := Pool.Exec(ctx, `
		DELETE FROM project_team_members ptm
		USING project_teams pt, users u
		WHERE ptm.team_id = pt.id
		  AND ptm.user_id = u.id
		  AND pt.iid = $1
		  AND u.iid = $2
	`, teamIID, userIID)
	if err != nil {
		return err
	}

	if res.RowsAffected() == 0 {
		return ErrMemberNotFound
	}
	return nil
}

// GetUserProjectPermissions evaluates effective role and accessible sources
// across both direct project membership and team memberships (Grafana/Datadog model).
func GetUserProjectPermissions(ctx context.Context, userID int, projectIID string) (string, []string, error) {
	if Pool == nil {
		return "", nil, fmt.Errorf("database connection pool not initialized")
	}

	// 1. Check direct membership
	directRole, directSources, _ := GetProjectMembership(ctx, userID, projectIID)

	// 2. Query all teams for this project that the user belongs to
	rows, err := Pool.Query(ctx, `
		SELECT pt.role::text, pt.allowed_sources
		FROM project_teams pt
		JOIN project_team_members ptm ON pt.id = ptm.team_id
		JOIN projects p ON p.id = pt.project_id
		WHERE ptm.user_id = $1 AND p.iid = $2
	`, userID, projectIID)
	if err != nil {
		return directRole, directSources, err
	}
	defer rows.Close()

	effectiveRole := directRole
	roleWeights := map[string]int{"viewer": 1, "member": 2, "admin": 3}

	var teamSourcesSets [][]string
	hasUnrestrictedAccess := (directRole != "" && len(directSources) == 0)
	userHasAnyMembership := (directRole != "")

	for rows.Next() {
		userHasAnyMembership = true
		var tRole string
		var tSources []string
		if err := rows.Scan(&tRole, &tSources); err == nil {
			if roleWeights[tRole] > roleWeights[effectiveRole] {
				effectiveRole = tRole
			}
			if len(tSources) == 0 {
				hasUnrestrictedAccess = true
			} else {
				teamSourcesSets = append(teamSourcesSets, tSources)
			}
		}
	}

	if !userHasAnyMembership {
		return "", nil, nil
	}

	if effectiveRole == "admin" || hasUnrestrictedAccess {
		return effectiveRole, nil, nil // Nil = access to all sources
	}

	// Compute union of allowed sources
	sourceSet := make(map[string]bool)
	for _, id := range directSources {
		sourceSet[id] = true
	}
	for _, set := range teamSourcesSets {
		for _, id := range set {
			sourceSet[id] = true
		}
	}

	var combined []string
	for id := range sourceSet {
		combined = append(combined, id)
	}
	return effectiveRole, combined, nil
}
