-- +goose Up

CREATE TABLE provider_configs (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_name   VARCHAR(100) NOT NULL,
    config_key      VARCHAR(100) NOT NULL,
    encrypted_value TEXT         NOT NULL,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_provider_config UNIQUE (provider_name, config_key)
);
CREATE TRIGGER trg_provider_configs_updated_at BEFORE UPDATE ON provider_configs
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE provider_call_logs (
    id                UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id           UUID         REFERENCES generation_tasks(id) ON DELETE SET NULL,
    provider          VARCHAR(100) NOT NULL,
    capability        VARCHAR(50)  NOT NULL,
    model             VARCHAR(255) NOT NULL DEFAULT '',
    status            VARCHAR(50)  NOT NULL,
    duration_ms       BIGINT       NOT NULL,
    prompt_tokens     INT          NOT NULL DEFAULT 0,
    completion_tokens INT          NOT NULL DEFAULT 0,
    total_tokens      INT          NOT NULL DEFAULT 0,
    cost_credits      INT          NOT NULL DEFAULT 0,
    error_message     TEXT,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_provider_call_logs_task     ON provider_call_logs (task_id);
CREATE INDEX idx_provider_call_logs_provider ON provider_call_logs (provider, capability, created_at DESC);
CREATE INDEX idx_provider_call_logs_status   ON provider_call_logs (status);

-- +goose Down
DROP TABLE IF EXISTS provider_call_logs;
DROP TABLE IF EXISTS provider_configs;
