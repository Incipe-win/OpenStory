-- +goose Up

-- ============================================================
-- Utility: auto-update updated_at on row modification
-- ============================================================
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- ============================================================
-- 1. users
-- ============================================================
CREATE TABLE users (
    id                UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    email             VARCHAR(255) NOT NULL,
    username          VARCHAR(100) NOT NULL,
    password_hash     VARCHAR(255) NOT NULL,
    display_name      VARCHAR(255) NOT NULL DEFAULT '',
    avatar_url        TEXT         NOT NULL DEFAULT '',
    role              VARCHAR(50)  NOT NULL DEFAULT 'user',
    status            VARCHAR(50)  NOT NULL DEFAULT 'active',
    email_verified_at TIMESTAMPTZ,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_users_email    UNIQUE (email),
    CONSTRAINT uq_users_username UNIQUE (username)
);
CREATE TRIGGER trg_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================
-- 2. refresh_tokens
-- ============================================================
CREATE TABLE refresh_tokens (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash  VARCHAR(255) NOT NULL,
    device_info TEXT         NOT NULL DEFAULT '',
    ip_address  VARCHAR(45)  NOT NULL DEFAULT '',
    expires_at  TIMESTAMPTZ  NOT NULL,
    revoked_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_refresh_tokens_hash UNIQUE (token_hash)
);
CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens (user_id);
CREATE INDEX idx_refresh_tokens_active  ON refresh_tokens (expires_at) WHERE revoked_at IS NULL;

-- ============================================================
-- 3. Recreate projects with FK to users
-- ============================================================
DROP TABLE IF EXISTS projects CASCADE;

CREATE TABLE projects (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name          VARCHAR(255) NOT NULL,
    description   TEXT         NOT NULL DEFAULT '',
    cover_url     TEXT         NOT NULL DEFAULT '',
    status        VARCHAR(50)  NOT NULL DEFAULT 'draft',
    settings_json JSONB        NOT NULL DEFAULT '{}',
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_projects_user_id ON projects (user_id);
CREATE INDEX idx_projects_status  ON projects (status);
CREATE TRIGGER trg_projects_updated_at BEFORE UPDATE ON projects
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================
-- 4. works
-- ============================================================
CREATE TABLE works (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id    UUID         NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id       UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title         VARCHAR(255) NOT NULL,
    description   TEXT         NOT NULL DEFAULT '',
    duration_ms   INT,
    resolution    VARCHAR(20),
    format        VARCHAR(20),
    file_url      TEXT         NOT NULL DEFAULT '',
    thumbnail_url TEXT         NOT NULL DEFAULT '',
    status        VARCHAR(50)  NOT NULL DEFAULT 'draft',
    metadata_json JSONB        NOT NULL DEFAULT '{}',
    published_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_works_project_id ON works (project_id);
CREATE INDEX idx_works_user_id    ON works (user_id);
CREATE INDEX idx_works_status     ON works (status);
CREATE TRIGGER trg_works_updated_at BEFORE UPDATE ON works
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================
-- 5. assets
-- ============================================================
CREATE TABLE assets (
    id             UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id        UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    project_id     UUID         REFERENCES projects(id) ON DELETE SET NULL,
    type           VARCHAR(50)  NOT NULL,
    name           VARCHAR(255) NOT NULL,
    mime_type      VARCHAR(100) NOT NULL DEFAULT '',
    size_bytes     BIGINT       NOT NULL DEFAULT 0,
    storage_key    VARCHAR(500) NOT NULL,
    storage_bucket VARCHAR(100) NOT NULL DEFAULT 'openstory',
    url            TEXT         NOT NULL DEFAULT '',
    thumbnail_url  TEXT         NOT NULL DEFAULT '',
    metadata_json  JSONB        NOT NULL DEFAULT '{}',
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_assets_user_id    ON assets (user_id);
CREATE INDEX idx_assets_project_id ON assets (project_id);
CREATE INDEX idx_assets_type       ON assets (type);
CREATE UNIQUE INDEX uq_assets_storage ON assets (storage_bucket, storage_key);

-- ============================================================
-- 6. workflows
-- ============================================================
CREATE TABLE workflows (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id      UUID         NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id         UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name            VARCHAR(255) NOT NULL,
    description     TEXT         NOT NULL DEFAULT '',
    status          VARCHAR(50)  NOT NULL DEFAULT 'draft',
    current_version INT          NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_workflows_project_id ON workflows (project_id);
CREATE INDEX idx_workflows_user_id    ON workflows (user_id);
CREATE TRIGGER trg_workflows_updated_at BEFORE UPDATE ON workflows
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================
-- 7. workflow_versions
-- ============================================================
CREATE TABLE workflow_versions (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id   UUID        NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
    version       INT         NOT NULL,
    snapshot_json JSONB       NOT NULL DEFAULT '{}',
    created_by    UUID        NOT NULL REFERENCES users(id),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_workflow_version UNIQUE (workflow_id, version)
);
CREATE INDEX idx_wf_versions_workflow ON workflow_versions (workflow_id);

-- ============================================================
-- 8. workflow_nodes
-- ============================================================
CREATE TABLE workflow_nodes (
    id          UUID             PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id UUID             NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
    type        VARCHAR(50)      NOT NULL,
    name        VARCHAR(255)     NOT NULL DEFAULT '',
    config_json JSONB            NOT NULL DEFAULT '{}',
    position_x  DOUBLE PRECISION NOT NULL DEFAULT 0,
    position_y  DOUBLE PRECISION NOT NULL DEFAULT 0,
    status      VARCHAR(50)      NOT NULL DEFAULT 'idle',
    created_at  TIMESTAMPTZ      NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ      NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_wf_nodes_workflow ON workflow_nodes (workflow_id);
CREATE TRIGGER trg_wf_nodes_updated_at BEFORE UPDATE ON workflow_nodes
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================
-- 9. workflow_edges
-- ============================================================
CREATE TABLE workflow_edges (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id    UUID        NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
    source_node_id UUID       NOT NULL REFERENCES workflow_nodes(id) ON DELETE CASCADE,
    target_node_id UUID       NOT NULL REFERENCES workflow_nodes(id) ON DELETE CASCADE,
    source_handle  VARCHAR(100) NOT NULL DEFAULT 'default',
    target_handle  VARCHAR(100) NOT NULL DEFAULT 'default',
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_workflow_edge UNIQUE (workflow_id, source_node_id, target_node_id, source_handle, target_handle)
);
CREATE INDEX idx_wf_edges_workflow ON workflow_edges (workflow_id);
CREATE INDEX idx_wf_edges_source   ON workflow_edges (source_node_id);
CREATE INDEX idx_wf_edges_target   ON workflow_edges (target_node_id);

-- ============================================================
-- 10. generation_tasks
-- ============================================================
CREATE TABLE generation_tasks (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID         NOT NULL REFERENCES users(id),
    project_id      UUID         NOT NULL REFERENCES projects(id),
    workflow_id     UUID         REFERENCES workflows(id),
    node_id         UUID         REFERENCES workflow_nodes(id),
    type            VARCHAR(50)  NOT NULL,
    provider        VARCHAR(100) NOT NULL DEFAULT '',
    status          VARCHAR(50)  NOT NULL DEFAULT 'pending',
    idempotency_key VARCHAR(255) NOT NULL,
    input_json      JSONB        NOT NULL DEFAULT '{}',
    output_json     JSONB,
    error_message   TEXT,
    cost_credits    INT          NOT NULL DEFAULT 0,
    retry_count     INT          NOT NULL DEFAULT 0,
    started_at      TIMESTAMPTZ,
    finished_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_tasks_idempotency UNIQUE (idempotency_key)
);
CREATE INDEX idx_gen_tasks_user_id    ON generation_tasks (user_id);
CREATE INDEX idx_gen_tasks_project_id ON generation_tasks (project_id);
CREATE INDEX idx_gen_tasks_workflow   ON generation_tasks (workflow_id);
CREATE INDEX idx_gen_tasks_node       ON generation_tasks (node_id);
CREATE INDEX idx_gen_tasks_status     ON generation_tasks (status);
CREATE INDEX idx_gen_tasks_type       ON generation_tasks (type);
CREATE TRIGGER trg_gen_tasks_updated_at BEFORE UPDATE ON generation_tasks
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================
-- 11. task_events
-- ============================================================
CREATE TABLE task_events (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id      UUID        NOT NULL REFERENCES generation_tasks(id) ON DELETE CASCADE,
    type         VARCHAR(50) NOT NULL,
    payload_json JSONB       NOT NULL DEFAULT '{}',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_task_events_task_id ON task_events (task_id);
CREATE INDEX idx_task_events_created ON task_events (created_at);

-- ============================================================
-- 12. outbox_events (BIGSERIAL id for strict ordering)
-- ============================================================
CREATE TABLE outbox_events (
    id             BIGSERIAL    PRIMARY KEY,
    event_type     VARCHAR(100) NOT NULL,
    aggregate_type VARCHAR(100) NOT NULL,
    aggregate_id   UUID         NOT NULL,
    payload_json   JSONB        NOT NULL DEFAULT '{}',
    schema_version INT          NOT NULL DEFAULT 1,
    trace_id       VARCHAR(100) NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    published_at   TIMESTAMPTZ,
    retry_count    INT          NOT NULL DEFAULT 0,
    last_error     TEXT
);
CREATE INDEX idx_outbox_unpublished ON outbox_events (id) WHERE published_at IS NULL;
CREATE INDEX idx_outbox_aggregate   ON outbox_events (aggregate_type, aggregate_id);
CREATE INDEX idx_outbox_type        ON outbox_events (event_type);

-- ============================================================
-- 13. credit_accounts
-- ============================================================
CREATE TABLE credit_accounts (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    balance     INT         NOT NULL DEFAULT 0,
    total_earned INT        NOT NULL DEFAULT 0,
    total_spent  INT        NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_credit_accounts_user UNIQUE (user_id),
    CONSTRAINT chk_credit_balance CHECK (balance >= 0)
);
CREATE TRIGGER trg_credit_accounts_updated_at BEFORE UPDATE ON credit_accounts
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================
-- 14. credit_ledger
-- ============================================================
CREATE TABLE credit_ledger (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id     UUID        NOT NULL REFERENCES credit_accounts(id),
    user_id        UUID        NOT NULL REFERENCES users(id),
    type           VARCHAR(50) NOT NULL,
    amount         INT         NOT NULL,
    balance_after  INT         NOT NULL,
    reference_type VARCHAR(50) NOT NULL DEFAULT '',
    reference_id   UUID,
    description    TEXT        NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_credit_ledger_account ON credit_ledger (account_id);
CREATE INDEX idx_credit_ledger_user    ON credit_ledger (user_id);
CREATE INDEX idx_credit_ledger_ref     ON credit_ledger (reference_type, reference_id);

-- ============================================================
-- 15. moderation_records
-- ============================================================
CREATE TABLE moderation_records (
    id              UUID             PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID             NOT NULL REFERENCES users(id),
    target_type     VARCHAR(50)      NOT NULL,
    target_id       UUID             NOT NULL,
    provider        VARCHAR(50)      NOT NULL DEFAULT '',
    status          VARCHAR(50)      NOT NULL DEFAULT 'pending',
    categories_json JSONB            NOT NULL DEFAULT '{}',
    score           DOUBLE PRECISION,
    reason          TEXT,
    reviewed_by     UUID             REFERENCES users(id),
    reviewed_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ      NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_moderation_target ON moderation_records (target_type, target_id);
CREATE INDEX idx_moderation_user   ON moderation_records (user_id);
CREATE INDEX idx_moderation_status ON moderation_records (status);

-- ============================================================
-- 16. audit_logs
-- ============================================================
CREATE TABLE audit_logs (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID         REFERENCES users(id) ON DELETE SET NULL,
    action          VARCHAR(100) NOT NULL,
    resource_type   VARCHAR(100) NOT NULL,
    resource_id     UUID,
    ip_address      VARCHAR(45)  NOT NULL DEFAULT '',
    user_agent      TEXT         NOT NULL DEFAULT '',
    old_values_json JSONB,
    new_values_json JSONB,
    trace_id        VARCHAR(100) NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_audit_logs_user     ON audit_logs (user_id);
CREATE INDEX idx_audit_logs_resource ON audit_logs (resource_type, resource_id);
CREATE INDEX idx_audit_logs_action   ON audit_logs (action);
CREATE INDEX idx_audit_logs_created  ON audit_logs (created_at);


-- +goose Down
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS moderation_records;
DROP TABLE IF EXISTS credit_ledger;
DROP TABLE IF EXISTS credit_accounts;
DROP TABLE IF EXISTS outbox_events;
DROP TABLE IF EXISTS task_events;
DROP TABLE IF EXISTS generation_tasks;
DROP TABLE IF EXISTS workflow_edges;
DROP TABLE IF EXISTS workflow_nodes;
DROP TABLE IF EXISTS workflow_versions;
DROP TABLE IF EXISTS workflows;
DROP TABLE IF EXISTS assets;
DROP TABLE IF EXISTS works;
DROP TABLE IF EXISTS projects;
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS users;
DROP FUNCTION IF EXISTS set_updated_at();
