-- +goose Up
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE projects (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(255) NOT NULL,
    description TEXT         NOT NULL DEFAULT '',
    owner_id    UUID         NOT NULL,
    status      VARCHAR(50)  NOT NULL DEFAULT 'draft',
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_projects_owner_id ON projects (owner_id);
CREATE INDEX idx_projects_status   ON projects (status);

-- +goose Down
DROP TABLE IF EXISTS projects;
