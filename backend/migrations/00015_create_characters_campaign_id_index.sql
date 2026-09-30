-- +goose Up
-- Finds a campaign's characters: the master's list (every character of the
-- campaign), a player's list (campaign_id and player_user_id), the sheet lock
-- when a game session starts (RN-01), and the ON DELETE SET NULL when a
-- campaign is deleted.
--
-- There is no index on player_user_id or master_user_id alone. Only an
-- account deletion looks characters up by them (ON DELETE SET NULL and
-- CASCADE), and scanning the table then is fine, as for
-- campaigns.created_by.
CREATE INDEX IF NOT EXISTS characters_campaign_id_player_user_id_idx ON characters (campaign_id, player_user_id);

-- +goose Down
DROP INDEX IF EXISTS characters_campaign_id_player_user_id_idx;
