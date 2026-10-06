-- +goose Up
-- Deleting a campaign deletes its saved encounters: the index keeps that from scanning the table.
CREATE INDEX IF NOT EXISTS battle_encounters_campaign_id_idx
    ON battle_encounters (campaign_id);

-- +goose Down
DROP INDEX IF EXISTS battle_encounters_campaign_id_idx;
