-- +goose Up
-- A character's creatures that are still with it, oldest first
-- (ListCharacterCreatures, a combat's start, a casting that replaces the
-- familiar). The partial index holds only the live ones.
CREATE INDEX IF NOT EXISTS character_creatures_character_id_live_idx
    ON character_creatures (character_id, created_at)
    WHERE dismissed_at IS NULL;

-- The creatures of one casting (concentration ending, an undo).
CREATE INDEX IF NOT EXISTS character_creatures_summon_group_id_idx
    ON character_creatures (summon_group_id);

-- A campaign's creatures: the ON DELETE CASCADE when a campaign is deleted.
CREATE INDEX IF NOT EXISTS character_creatures_campaign_id_idx
    ON character_creatures (campaign_id);

-- +goose Down
DROP INDEX IF EXISTS character_creatures_campaign_id_idx;
DROP INDEX IF EXISTS character_creatures_summon_group_id_idx;
DROP INDEX IF EXISTS character_creatures_character_id_live_idx;
