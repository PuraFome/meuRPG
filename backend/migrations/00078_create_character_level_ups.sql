-- +goose Up
-- character_level_ups is the record of a guided level-up (MR-040, RN-01): what
-- the owning player chose when a character that could level up went up a
-- level on its locked sheet. The master reads it as "O que mudou" (RN-12);
-- the sheet itself already holds the result.
--
-- from_level and to_level are the character's total levels (class_key is the
-- class that gained the level). choices is the protojson of
-- characters.v1.LevelUpChoices: only what was new (the ability increase, the
-- subclass, the new cantrips, spells and prepared spells, the new feature
-- options, skills and expertise) and the hit points of the level, as content
-- keys and numbers. hp_method ('average', 'rolled_in_app' or
-- 'rolled_physical') and hp_value repeat the hit points so the history can be
-- read without decoding the document.
--
-- Rows are written once and never changed (like session_events). Deleting
-- the character or the campaign deletes them. No personal data: the player
-- is the character's own player_user_id, and nothing is free text.
CREATE TABLE IF NOT EXISTS character_level_ups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id UUID NOT NULL REFERENCES campaigns (id) ON DELETE CASCADE,
    character_id UUID NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    class_key TEXT NOT NULL,
    from_level INT4 NOT NULL,
    to_level INT4 NOT NULL,
    hp_method TEXT NOT NULL,
    hp_value INT4 NOT NULL,
    choices JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT character_level_ups_levels_valid CHECK (from_level >= 1 AND to_level = from_level + 1 AND to_level <= 20),
    CONSTRAINT character_level_ups_hp_method_valid CHECK (hp_method IN ('average', 'rolled_in_app', 'rolled_physical')),
    CONSTRAINT character_level_ups_hp_value_valid CHECK (hp_value >= 1 AND hp_value <= 12)
);

-- +goose Down
DROP TABLE IF EXISTS character_level_ups;
