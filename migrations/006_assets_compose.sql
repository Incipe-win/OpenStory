-- +goose Up

ALTER TABLE assets
    ADD COLUMN duration_ms INT,
    ADD COLUMN width       INT,
    ADD COLUMN height      INT,
    ADD COLUMN checksum    VARCHAR(128) NOT NULL DEFAULT '';

CREATE INDEX idx_assets_checksum ON assets (checksum) WHERE checksum <> '';

-- +goose Down
DROP INDEX IF EXISTS idx_assets_checksum;
ALTER TABLE assets
    DROP COLUMN IF EXISTS checksum,
    DROP COLUMN IF EXISTS height,
    DROP COLUMN IF EXISTS width,
    DROP COLUMN IF EXISTS duration_ms;
