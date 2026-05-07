-- +goose Up

ALTER TABLE assets
    ADD COLUMN IF NOT EXISTS status       VARCHAR(50) NOT NULL DEFAULT 'draft',
    ADD COLUMN IF NOT EXISTS published_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW();

CREATE INDEX IF NOT EXISTS idx_assets_status
    ON assets (status);
CREATE INDEX IF NOT EXISTS idx_assets_published
    ON assets (published_at DESC)
    WHERE status = 'published';

ALTER TABLE assets
    DROP CONSTRAINT IF EXISTS chk_assets_status;
ALTER TABLE assets
    ADD CONSTRAINT chk_assets_status
    CHECK (status IN ('draft', 'pending_review', 'published', 'rejected')) NOT VALID;
ALTER TABLE assets VALIDATE CONSTRAINT chk_assets_status;

-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_trigger
        WHERE tgname = 'trg_assets_updated_at'
    ) THEN
        CREATE TRIGGER trg_assets_updated_at
            BEFORE UPDATE ON assets
            FOR EACH ROW EXECUTE FUNCTION set_updated_at();
    END IF;
END;
$$;
-- +goose StatementEnd

-- +goose Down

DROP TRIGGER IF EXISTS trg_assets_updated_at ON assets;
ALTER TABLE assets
    DROP CONSTRAINT IF EXISTS chk_assets_status;
DROP INDEX IF EXISTS idx_assets_published;
DROP INDEX IF EXISTS idx_assets_status;
ALTER TABLE assets
    DROP COLUMN IF EXISTS updated_at,
    DROP COLUMN IF EXISTS published_at,
    DROP COLUMN IF EXISTS status;
