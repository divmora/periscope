-- +goose Up
-- Granular Source-Level Access Control per project membership
ALTER TABLE project_memberships ADD COLUMN IF NOT EXISTS allowed_sources TEXT[] DEFAULT NULL;

-- Project Teams (Grafana / Datadog RBAC model)
CREATE TABLE IF NOT EXISTS project_teams (
    id SERIAL PRIMARY KEY,
    iid UUID NOT NULL DEFAULT gen_random_uuid() UNIQUE,
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    role project_role NOT NULL DEFAULT 'viewer',
    allowed_sources TEXT[] DEFAULT NULL, -- NULL = all sources in project; or array of source IIDs
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(project_id, name)
);

CREATE INDEX IF NOT EXISTS idx_project_teams_project_id ON project_teams(project_id);

-- Team memberships linking users to teams within a project
CREATE TABLE IF NOT EXISTS project_team_members (
    team_id INTEGER NOT NULL REFERENCES project_teams(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (team_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_project_team_members_user_id ON project_team_members(user_id);

-- +goose Down
DROP TABLE IF EXISTS project_team_members;
DROP TABLE IF EXISTS project_teams;
ALTER TABLE project_memberships DROP COLUMN IF EXISTS allowed_sources;
