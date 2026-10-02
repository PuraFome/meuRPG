-- +goose Up
-- dice_mode is how the campaign's players roll dice (RN-18, MR-013):
--   - 'players_choose': each player picks "No app" or "Meus próprios dados"
--     (campaign_members.dice_preference, migration 00037). The default.
--   - 'app': everyone rolls in the app; nobody types a result.
--   - 'physical': everyone rolls real dice and types the sum.
-- The master changes it at any time; it applies from the next roll. NPCs
-- always roll in the app, whatever the mode.
--
-- DEFAULT 'players_choose' gives every existing campaign the mode Samuel
-- chose as the default (Q38).
--
-- One statement with several parts, so re-running it is safe (see 00022).
ALTER TABLE campaigns
    ADD COLUMN IF NOT EXISTS dice_mode TEXT NOT NULL DEFAULT 'players_choose',
    DROP CONSTRAINT IF EXISTS campaigns_dice_mode_valid,
    ADD CONSTRAINT campaigns_dice_mode_valid CHECK (dice_mode IN ('players_choose', 'app', 'physical'));

-- +goose Down
ALTER TABLE campaigns
    DROP CONSTRAINT IF EXISTS campaigns_dice_mode_valid,
    DROP COLUMN IF EXISTS dice_mode;
