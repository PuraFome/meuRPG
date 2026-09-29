-- +goose Up
-- Lists the campaigns a user is in (CampaignService.ListMyCampaigns), and
-- finds the memberships to delete when an account is deleted.
CREATE INDEX IF NOT EXISTS campaign_members_user_id_idx ON campaign_members (user_id);

-- +goose Down
DROP INDEX IF EXISTS campaign_members_user_id_idx;
