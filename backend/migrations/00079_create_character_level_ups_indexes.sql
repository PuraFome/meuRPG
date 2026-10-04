-- +goose Up
-- A campaign's level-ups, newest first: the master's "O que mudou". It also
-- serves the ON DELETE CASCADE when the campaign is deleted.
CREATE INDEX IF NOT EXISTS character_level_ups_campaign_id_created_at_idx
    ON character_level_ups (campaign_id, created_at DESC, id DESC);

-- One character's level-ups, newest first, and the ON DELETE CASCADE when
-- the character is deleted.
CREATE INDEX IF NOT EXISTS character_level_ups_character_id_created_at_idx
    ON character_level_ups (character_id, created_at DESC, id DESC);

-- +goose Down
DROP INDEX IF EXISTS character_level_ups_character_id_created_at_idx;
DROP INDEX IF EXISTS character_level_ups_campaign_id_created_at_idx;
