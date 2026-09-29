-- +goose Up
-- Lists a campaign's invites for its master (CampaignService.ListInvites).
CREATE INDEX IF NOT EXISTS campaign_invites_campaign_id_idx ON campaign_invites (campaign_id);

-- +goose Down
DROP INDEX IF EXISTS campaign_invites_campaign_id_idx;
