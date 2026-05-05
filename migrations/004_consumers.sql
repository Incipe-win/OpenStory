-- +goose Up

-- ============================================================
-- Consumer idempotency and failure tracking
-- ============================================================
CREATE TABLE consumer_processed_events (
    consumer_name VARCHAR(100) NOT NULL,
    event_id      VARCHAR(100) NOT NULL,
    topic         VARCHAR(100) NOT NULL,
    partition_id  INT          NOT NULL,
    offset_id     BIGINT       NOT NULL,
    event_type    VARCHAR(100) NOT NULL,
    processed_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    PRIMARY KEY (consumer_name, event_id)
);
CREATE INDEX idx_consumer_processed_topic_offset
    ON consumer_processed_events (topic, partition_id, offset_id);

CREATE TABLE consumer_failures (
    id             UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    consumer_name  VARCHAR(100) NOT NULL,
    topic          VARCHAR(100) NOT NULL,
    partition_id   INT          NOT NULL,
    offset_id      BIGINT       NOT NULL,
    event_id       VARCHAR(100) NOT NULL DEFAULT '',
    event_type     VARCHAR(100) NOT NULL DEFAULT '',
    attempts       INT          NOT NULL DEFAULT 1,
    last_error     TEXT         NOT NULL DEFAULT '',
    first_failed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_failed_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_consumer_failure_message UNIQUE (consumer_name, topic, partition_id, offset_id)
);
CREATE INDEX idx_consumer_failures_consumer ON consumer_failures (consumer_name);
CREATE INDEX idx_consumer_failures_event    ON consumer_failures (event_id);

-- ============================================================
-- Consumer materialized views / side effects
-- ============================================================
CREATE TABLE analytics_task_metrics (
    bucket_date        DATE         NOT NULL,
    provider           VARCHAR(100) NOT NULL,
    task_type          VARCHAR(50)  NOT NULL,
    total_tasks        BIGINT       NOT NULL DEFAULT 0,
    succeeded_tasks    BIGINT       NOT NULL DEFAULT 0,
    failed_tasks       BIGINT       NOT NULL DEFAULT 0,
    canceled_tasks     BIGINT       NOT NULL DEFAULT 0,
    total_duration_ms  BIGINT       NOT NULL DEFAULT 0,
    total_cost_credits BIGINT       NOT NULL DEFAULT 0,
    updated_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    PRIMARY KEY (bucket_date, provider, task_type)
);

CREATE TABLE notifications (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id    VARCHAR(100) NOT NULL,
    user_id     UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type        VARCHAR(100) NOT NULL,
    title       VARCHAR(255) NOT NULL,
    body        TEXT         NOT NULL DEFAULT '',
    read_at     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_notifications_event UNIQUE (event_id)
);
CREATE INDEX idx_notifications_user_created ON notifications (user_id, created_at DESC);

CREATE TABLE feed_items (
    work_id       UUID        PRIMARY KEY REFERENCES works(id) ON DELETE CASCADE,
    event_id      VARCHAR(100) NOT NULL,
    user_id       UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    project_id    UUID        NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    title         VARCHAR(255) NOT NULL,
    metadata_json JSONB       NOT NULL DEFAULT '{}',
    rank_score    DOUBLE PRECISION NOT NULL DEFAULT 0,
    published_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_feed_items_event UNIQUE (event_id)
);
CREATE INDEX idx_feed_items_published ON feed_items (published_at DESC);

CREATE UNIQUE INDEX uq_moderation_target
    ON moderation_records (target_type, target_id);

-- +goose Down
DROP INDEX IF EXISTS uq_moderation_target;
DROP TABLE IF EXISTS feed_items;
DROP TABLE IF EXISTS notifications;
DROP TABLE IF EXISTS analytics_task_metrics;
DROP TABLE IF EXISTS consumer_failures;
DROP TABLE IF EXISTS consumer_processed_events;
