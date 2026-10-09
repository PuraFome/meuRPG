-- +goose Up
-- The export screen reads a campaign's latest export.
CREATE INDEX IF NOT EXISTS campaign_exports_campaign_id_created_at_idx
    ON campaign_exports (campaign_id, created_at DESC);

-- The cleanup looks for the rows that expired.
CREATE INDEX IF NOT EXISTS campaign_exports_expires_at_idx
    ON campaign_exports (expires_at);

-- A retry of StartCampaignExport with the same key finds the first export.
CREATE UNIQUE INDEX IF NOT EXISTS campaign_exports_create_key_idx
    ON campaign_exports (create_key) WHERE create_key IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS campaign_exports_create_key_idx;
DROP INDEX IF EXISTS campaign_exports_expires_at_idx;
DROP INDEX IF EXISTS campaign_exports_campaign_id_created_at_idx;
