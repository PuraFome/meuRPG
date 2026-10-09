-- +goose Up
-- One upload at a time per person.
CREATE UNIQUE INDEX IF NOT EXISTS campaign_imports_user_id_idx
    ON campaign_imports (user_id);

-- The cleanup looks for the uploads that expired.
CREATE INDEX IF NOT EXISTS campaign_imports_expires_at_idx
    ON campaign_imports (expires_at);

-- +goose Down
DROP INDEX IF EXISTS campaign_imports_expires_at_idx;
DROP INDEX IF EXISTS campaign_imports_user_id_idx;
