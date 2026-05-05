-- +goose Up

ALTER TABLE credit_ledger
    ADD COLUMN IF NOT EXISTS metadata_json JSONB NOT NULL DEFAULT '{}';

CREATE UNIQUE INDEX IF NOT EXISTS uq_credit_ledger_ref_operation
    ON credit_ledger (reference_type, reference_id, type)
    WHERE reference_id IS NOT NULL
      AND type IN ('reserve', 'confirm', 'refund');

ALTER TABLE moderation_records
    ADD CONSTRAINT chk_moderation_status
    CHECK (status IN ('pending', 'approved', 'rejected')) NOT VALID;
ALTER TABLE moderation_records VALIDATE CONSTRAINT chk_moderation_status;

-- +goose Down

ALTER TABLE moderation_records
    DROP CONSTRAINT IF EXISTS chk_moderation_status;

DROP INDEX IF EXISTS uq_credit_ledger_ref_operation;

ALTER TABLE credit_ledger
    DROP COLUMN IF EXISTS metadata_json;
