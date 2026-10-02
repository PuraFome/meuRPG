-- +goose Up
-- dice_preference is how this member prefers to roll (RN-18, MR-014):
-- 'app' (the default) or 'physical' (their own dice, typing the sum). It
-- only counts while the campaign's dice_mode is 'players_choose'
-- (migration 00036); with another mode the choice is kept, but ignored.
-- Every member has one, the master included, who may roll their own
-- dice too. It belongs to the membership, so it is per campaign.
--
-- One statement with several parts, so re-running it is safe (see 00022).
ALTER TABLE campaign_members
    ADD COLUMN IF NOT EXISTS dice_preference TEXT NOT NULL DEFAULT 'app',
    DROP CONSTRAINT IF EXISTS campaign_members_dice_preference_valid,
    ADD CONSTRAINT campaign_members_dice_preference_valid CHECK (dice_preference IN ('app', 'physical'));

-- +goose Down
ALTER TABLE campaign_members
    DROP CONSTRAINT IF EXISTS campaign_members_dice_preference_valid,
    DROP COLUMN IF EXISTS dice_preference;
