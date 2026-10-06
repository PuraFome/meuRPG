-- +goose Up
-- One character per (campaign, create_key): the unique index is what makes the
-- idempotency hold when two calls with the same key race. Rows without a key
-- are left out of the index.
CREATE UNIQUE INDEX IF NOT EXISTS characters_campaign_id_create_key_idx
    ON characters (campaign_id, create_key)
    WHERE create_key IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS characters_campaign_id_create_key_idx;
