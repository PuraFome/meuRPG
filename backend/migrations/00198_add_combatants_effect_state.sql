-- +goose Up
-- What the effects that last do to a combatant, worked out again whenever one
-- starts or ends so that every read of the combatant sees it (RN-22):
-- effect_conditions are the conditions the effects give, which are also in
-- conditions (the service tells them apart from the ones set by hand);
-- effect_ac_bonus is the armor class the effects add (Velocidade), effect_speed_pct
-- the speed in percent of the normal one (Velocidade doubles it, the exhaustion
-- halves it or takes it to 0), and effect_no_action and effect_no_move are the
-- lethargy of a Velocidade that ended.
--
-- exhaustion_level is the creature's level of exhaustion, 0 to 6 (a player's
-- character keeps its own in character_vitals); hp_max_base is the NPC's hit
-- point maximum before the exhaustion of level 4 halved it, NULL while it is not.
--
-- One statement with several parts, so re-running it is safe (see 00036).
ALTER TABLE combatants
    ADD COLUMN IF NOT EXISTS effect_conditions TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS effect_ac_bonus INT4 NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS effect_speed_pct INT4 NOT NULL DEFAULT 100,
    ADD COLUMN IF NOT EXISTS effect_no_action BOOL NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS effect_no_move BOOL NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS exhaustion_level INT4 NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS hp_max_base INT4 NULL,
    DROP CONSTRAINT IF EXISTS combatants_exhaustion_level_valid,
    ADD CONSTRAINT combatants_exhaustion_level_valid CHECK (exhaustion_level BETWEEN 0 AND 6),
    DROP CONSTRAINT IF EXISTS combatants_effect_speed_pct_valid,
    ADD CONSTRAINT combatants_effect_speed_pct_valid CHECK (effect_speed_pct BETWEEN 0 AND 400);

-- A player's character keeps its exhaustion level between fights, with its other
-- live numbers.
ALTER TABLE character_vitals
    ADD COLUMN IF NOT EXISTS exhaustion_level INT4 NOT NULL DEFAULT 0,
    DROP CONSTRAINT IF EXISTS character_vitals_exhaustion_level_valid,
    ADD CONSTRAINT character_vitals_exhaustion_level_valid CHECK (exhaustion_level BETWEEN 0 AND 6);

-- The damage an effect deals when a turn starts under it (the web that burns) has
-- no attacker and no trap: effect_source_key is the effect's key.
ALTER TABLE pending_damages
    ADD COLUMN IF NOT EXISTS effect_source_key TEXT NULL;

-- The kinds of session event the effects write. A new kind is one INSERT (00168).
INSERT INTO session_event_kinds (kind) VALUES
    ('combat_effect_added'),
    ('combat_effect_changed'),
    ('combat_effect_ended'),
    ('combat_effect_visibility_changed'),
    ('combat_effect_saved'),
    ('combat_effect_triggered'),
    ('exhaustion_changed')
ON CONFLICT (kind) DO NOTHING;

-- +goose Down
DELETE FROM session_events WHERE kind IN ('combat_effect_added', 'combat_effect_changed', 'combat_effect_ended', 'combat_effect_visibility_changed', 'combat_effect_saved', 'combat_effect_triggered', 'exhaustion_changed');
DELETE FROM session_event_kinds WHERE kind IN ('combat_effect_added', 'combat_effect_changed', 'combat_effect_ended', 'combat_effect_visibility_changed', 'combat_effect_saved', 'combat_effect_triggered', 'exhaustion_changed');
ALTER TABLE pending_damages DROP COLUMN IF EXISTS effect_source_key;
ALTER TABLE character_vitals
    DROP CONSTRAINT IF EXISTS character_vitals_exhaustion_level_valid,
    DROP COLUMN IF EXISTS exhaustion_level;
ALTER TABLE combatants
    DROP CONSTRAINT IF EXISTS combatants_effect_speed_pct_valid,
    DROP CONSTRAINT IF EXISTS combatants_exhaustion_level_valid,
    DROP COLUMN IF EXISTS hp_max_base,
    DROP COLUMN IF EXISTS exhaustion_level,
    DROP COLUMN IF EXISTS effect_no_move,
    DROP COLUMN IF EXISTS effect_no_action,
    DROP COLUMN IF EXISTS effect_speed_pct,
    DROP COLUMN IF EXISTS effect_ac_bonus,
    DROP COLUMN IF EXISTS effect_conditions;
