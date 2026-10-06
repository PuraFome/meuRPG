-- +goose Up
-- xp_mode_changed_at is when the master last changed the XP mode after creating
-- the campaign (RN-09, SetCampaignXpMode): NULL until it changes. It lets the app
-- say "Modo de XP mudado em ..." again after a reload, and lets a repeated call
-- that changes nothing answer with the time of the real change.
--
-- One statement, so re-running it is safe (see 00022).
ALTER TABLE campaigns
    ADD COLUMN IF NOT EXISTS xp_mode_changed_at TIMESTAMPTZ NULL;

-- +goose Down
ALTER TABLE campaigns
    DROP COLUMN IF EXISTS xp_mode_changed_at;
