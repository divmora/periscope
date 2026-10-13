-- +goose Up
-- Allow projects to be created without requiring an immediate Laravel DSN
ALTER TABLE projects ALTER COLUMN encrypted_source_dsn DROP NOT NULL;

-- Advanced Data Retention & Lifecycle Tiering
ALTER TABLE projects ADD COLUMN IF NOT EXISTS source_retention_days INTEGER NOT NULL DEFAULT 0;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS lakehouse_retention_days INTEGER NOT NULL DEFAULT 365;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS compaction_enabled BOOLEAN NOT NULL DEFAULT TRUE;

-- Multi-Source Architecture: project_sources table
CREATE TABLE IF NOT EXISTS project_sources (
    id SERIAL PRIMARY KEY,
    iid UUID NOT NULL DEFAULT gen_random_uuid() UNIQUE,
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    source_type VARCHAR(50) NOT NULL, -- 'laravel_telescope', 'aurora_das', 'postgres_audit', 'opentelemetry', 'generic_db'
    status VARCHAR(50) NOT NULL DEFAULT 'active', -- 'active', 'paused', 'error'
    encrypted_dsn TEXT,
    config JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_project_sources_project_id ON project_sources(project_id);
CREATE INDEX IF NOT EXISTS idx_project_sources_source_type ON project_sources(source_type);

-- +goose Down
DROP TABLE IF EXISTS project_sources;
ALTER TABLE projects DROP COLUMN IF EXISTS compaction_enabled;
ALTER TABLE projects DROP COLUMN IF EXISTS lakehouse_retention_days;
ALTER TABLE projects DROP COLUMN IF EXISTS source_retention_days;
ALTER TABLE projects ALTER COLUMN encrypted_source_dsn SET NOT NULL;
