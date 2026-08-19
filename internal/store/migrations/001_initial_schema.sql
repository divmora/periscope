-- 001_initial_schema.sql
-- +goose Up
-- Enable uuid-ossp if gen_random_uuid() is not available, though PG13+ has it built-in.
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    iid UUID NOT NULL DEFAULT gen_random_uuid() UNIQUE,
    username VARCHAR(255) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    global_role VARCHAR(50) NOT NULL DEFAULT 'member',
    status VARCHAR(50) NOT NULL DEFAULT 'active',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE projects (
    id SERIAL PRIMARY KEY,
    iid UUID UNIQUE NOT NULL DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    -- AES Encrypted DSN of the source Laravel database
    encrypted_source_dsn TEXT NOT NULL,
    -- Configuration for where Parquet files are stored
    storage_type VARCHAR(50) NOT NULL DEFAULT 's3',
    storage_bucket VARCHAR(255) NOT NULL,
    storage_region VARCHAR(255),
    storage_endpoint VARCHAR(255),
    encrypted_storage_access_key TEXT,
    encrypted_storage_secret_key TEXT,
    prune_enabled BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TYPE project_role AS ENUM ('viewer', 'member', 'admin');

CREATE TABLE project_memberships (
    project_id INTEGER REFERENCES projects(id) ON DELETE CASCADE,
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    role project_role NOT NULL DEFAULT 'viewer',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (project_id, user_id)
);

CREATE TABLE telescope_watermarks (
    project_id INTEGER REFERENCES projects(id) ON DELETE CASCADE,
    entry_type VARCHAR(50) NOT NULL,
    last_sequence BIGINT NOT NULL DEFAULT 0,
    last_synced_at TIMESTAMP WITH TIME ZONE,
    PRIMARY KEY (project_id, entry_type)
);

-- Indexes
CREATE INDEX idx_project_memberships_user_id ON project_memberships(user_id);

-- +goose Down
DROP TABLE IF EXISTS telescope_watermarks;
DROP TABLE IF EXISTS project_memberships;
DROP TYPE IF EXISTS project_role;
DROP TABLE IF EXISTS projects;
DROP TABLE IF EXISTS users;
DROP EXTENSION IF EXISTS "pgcrypto";
