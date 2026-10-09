-- +goose Up
-- Effects that last (RN-22): a combat effect is a row of combatant_states with the
-- kind 'effect', one for each target. It is something that stays on a combatant for a
-- duration: a spell (Bênção, Perdição, Velocidade, Imobilizar Pessoa, Teia, Fogo das
-- Fadas), what a spell leaves when it ends (the lethargy of Velocidade), an action
-- the app keeps for a while, or a condition the master put there with an end. The
-- rest of the row is the state's own (source_id: who cast it, ends_*: when it ends).
--
-- group_id is the casting: the rows of one casting (one for each target) share it,
-- and, with source_key (the spell's or the catalog's key), they are what the master
-- reads as one effect with several targets. concentration says it ends with its
-- caster's concentration. source_kind is 'spell', 'feature' or 'master'.
-- condition_keys are the conditions it gives (they also go to combatants.conditions)
-- and modifiers the dice, the armor class, the speed and the actions it changes (a
-- JSON list of at most 8 modifiers of rules.EffectModifier).
--
-- duration_kind says how it ends by itself: 'rounds', 'until_start_of_turn_of',
-- 'until_end_of_turn_of', 'concentration', 'until_dismissed' or 'long_rest'; the
-- moment on the clock is ends_round, ends_combatant_id and ends_phase of the state
-- (a start or an end of a turn; no combatant: the start of the round).
--
-- end_save_ability is the saving throw the target makes at the end of its turn,
-- start_save_ability the one at the start of it, save_dc the caster's DC kept when it
-- was cast (NULL: none), on_fail_effect the catalog effect a failed start-of-turn
-- save adds, follows_key the spell a lethargy follows, and trigger_dice,
-- trigger_damage_type and trigger_max_triggers the damage dealt to who starts a turn
-- under it, with triggers_fired how many times it did to this target.
--
-- player_visible is the master's switch (off: only the master reads the effect), audience 'all' or 'owner', player_label the master's free label (at
-- most 30 characters).
--
-- One statement with several parts, so re-running it is safe (see 00036).
ALTER TABLE combatant_states
    ADD COLUMN IF NOT EXISTS group_id UUID NULL,
    ADD COLUMN IF NOT EXISTS source_key TEXT NULL,
    ADD COLUMN IF NOT EXISTS source_kind TEXT NULL,
    ADD COLUMN IF NOT EXISTS concentration BOOL NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS condition_keys TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS modifiers JSONB NOT NULL DEFAULT '[]',
    ADD COLUMN IF NOT EXISTS duration_kind TEXT NULL,
    ADD COLUMN IF NOT EXISTS end_save_ability TEXT NULL,
    ADD COLUMN IF NOT EXISTS start_save_ability TEXT NULL,
    ADD COLUMN IF NOT EXISTS save_dc INT4 NULL,
    ADD COLUMN IF NOT EXISTS on_fail_effect TEXT NULL,
    ADD COLUMN IF NOT EXISTS follows_key TEXT NULL,
    ADD COLUMN IF NOT EXISTS trigger_dice TEXT NULL,
    ADD COLUMN IF NOT EXISTS trigger_damage_type TEXT NULL,
    ADD COLUMN IF NOT EXISTS trigger_max_triggers INT4 NULL,
    ADD COLUMN IF NOT EXISTS triggers_fired INT4 NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS player_visible BOOL NOT NULL DEFAULT true,
    ADD COLUMN IF NOT EXISTS audience TEXT NOT NULL DEFAULT 'all',
    ADD COLUMN IF NOT EXISTS player_label TEXT NULL,
    DROP CONSTRAINT IF EXISTS combatant_states_kind_valid,
    ADD CONSTRAINT combatant_states_kind_valid CHECK (kind IN ('rage', 'hunters_mark_target', 'dodging', 'reckless', 'effect')),
    DROP CONSTRAINT IF EXISTS combatant_states_effect_valid,
    ADD CONSTRAINT combatant_states_effect_valid CHECK (
        kind <> 'effect'
        OR (group_id IS NOT NULL AND source_key IS NOT NULL AND source_kind IN ('spell', 'feature', 'master')
            AND duration_kind IN ('rounds', 'until_start_of_turn_of', 'until_end_of_turn_of', 'concentration', 'until_dismissed', 'long_rest'))
    ),
    DROP CONSTRAINT IF EXISTS combatant_states_audience_valid,
    ADD CONSTRAINT combatant_states_audience_valid CHECK (audience IN ('all', 'owner')),
    DROP CONSTRAINT IF EXISTS combatant_states_label_valid,
    ADD CONSTRAINT combatant_states_label_valid CHECK (player_label IS NULL OR char_length(player_label) BETWEEN 1 AND 30),
    DROP CONSTRAINT IF EXISTS combatant_states_save_dc_valid,
    ADD CONSTRAINT combatant_states_save_dc_valid CHECK (save_dc IS NULL OR save_dc BETWEEN 1 AND 40),
    DROP CONSTRAINT IF EXISTS combatant_states_modifiers_valid,
    ADD CONSTRAINT combatant_states_modifiers_valid CHECK (jsonb_typeof(modifiers) = 'array' AND jsonb_array_length(modifiers) <= 8);

-- The same effect on a character that is not in a running combat: the record changes home
-- with the character, whole and with the same id, when a combat takes it in or lets it go,
-- so there is never a copy of it. Out of combat there are no turns and no running clock:
-- seconds_left is the game time the effect has left (a round is 6 seconds, in a combat and
-- out of it), and it only moves when the master moves game time (a cast that took time, a
-- rest) or ends the effect. NULL means it lasts until it is dismissed or its caster stops
-- concentrating. source_character_id is the caster (a character), NULL for the master.
-- An effect of a monster (no character) lives in a combat alone and ends with it.
CREATE TABLE IF NOT EXISTS character_effects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id UUID NOT NULL REFERENCES campaigns (id) ON DELETE CASCADE,
    character_id UUID NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    source_character_id UUID NULL REFERENCES characters (id) ON DELETE SET NULL,
    group_id UUID NOT NULL,
    source_key TEXT NOT NULL,
    source_kind TEXT NOT NULL CHECK (source_kind IN ('spell', 'feature', 'master')),
    concentration BOOL NOT NULL DEFAULT false,
    condition_keys TEXT[] NOT NULL DEFAULT '{}',
    modifiers JSONB NOT NULL DEFAULT '[]' CHECK (jsonb_typeof(modifiers) = 'array' AND jsonb_array_length(modifiers) <= 8),
    duration_kind TEXT NOT NULL CHECK (duration_kind IN ('rounds', 'until_start_of_turn_of', 'until_end_of_turn_of', 'concentration', 'until_dismissed', 'long_rest')),
    seconds_left INT4 NULL CHECK (seconds_left IS NULL OR seconds_left >= 0),
    end_save_ability TEXT NULL,
    start_save_ability TEXT NULL,
    save_dc INT4 NULL CHECK (save_dc IS NULL OR save_dc BETWEEN 1 AND 40),
    on_fail_effect TEXT NULL,
    follows_key TEXT NULL,
    trigger_dice TEXT NULL,
    trigger_damage_type TEXT NULL,
    trigger_max_triggers INT4 NULL,
    triggers_fired INT4 NOT NULL DEFAULT 0,
    player_visible BOOL NOT NULL DEFAULT true,
    audience TEXT NOT NULL DEFAULT 'all' CHECK (audience IN ('all', 'owner')),
    player_label TEXT NULL CHECK (player_label IS NULL OR char_length(player_label) BETWEEN 1 AND 30),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS character_effects_character_idx ON character_effects (character_id);
CREATE INDEX IF NOT EXISTS character_effects_campaign_group_idx ON character_effects (campaign_id, group_id);

-- What the effects leave on a combatant, worked out again whenever one starts or
-- ends so that every read of the combatant sees it: effect_conditions are the
-- conditions the effects give (they are also in conditions; the service tells them
-- from the ones set by hand), effect_ac_bonus the armor class they add (Velocidade),
-- effect_speed_pct the speed in percent of the normal one (Velocidade doubles it,
-- exhaustion halves it or takes it to 0), effect_no_action and effect_no_move the
-- lethargy of a Velocidade that ended, and extra_action_used whether the extra
-- action of this turn was spent.
--
-- exhaustion_level is the creature's level of exhaustion, 0 to 6 (a player's
-- character keeps its own in character_vitals and the combat copies it);
-- hp_max_base is the NPC's hit point maximum before the exhaustion of level 4 halved
-- it, NULL while it is not.
ALTER TABLE combatants
    ADD COLUMN IF NOT EXISTS effect_conditions TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS effect_ac_bonus INT4 NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS effect_speed_pct INT4 NOT NULL DEFAULT 100,
    ADD COLUMN IF NOT EXISTS effect_speed_add_ft INT4 NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS effect_no_action BOOL NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS effect_no_move BOOL NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS extra_action_used BOOL NOT NULL DEFAULT false,
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

-- The damage an effect deals when a turn starts under it (the web that burns) has no
-- attacker and no trap: effect_source_key is the effect's key.
ALTER TABLE pending_damages
    ADD COLUMN IF NOT EXISTS effect_source_key TEXT NULL;

-- +goose Down
DROP TABLE IF EXISTS character_effects;
ALTER TABLE pending_damages DROP COLUMN IF EXISTS effect_source_key;
ALTER TABLE character_vitals
    DROP CONSTRAINT IF EXISTS character_vitals_exhaustion_level_valid,
    DROP COLUMN IF EXISTS exhaustion_level;
ALTER TABLE combatants
    DROP CONSTRAINT IF EXISTS combatants_effect_speed_pct_valid,
    DROP CONSTRAINT IF EXISTS combatants_exhaustion_level_valid,
    DROP COLUMN IF EXISTS hp_max_base,
    DROP COLUMN IF EXISTS exhaustion_level,
    DROP COLUMN IF EXISTS extra_action_used,
    DROP COLUMN IF EXISTS effect_no_move,
    DROP COLUMN IF EXISTS effect_no_action,
    DROP COLUMN IF EXISTS effect_speed_pct,
    DROP COLUMN IF EXISTS effect_speed_add_ft,
    DROP COLUMN IF EXISTS effect_ac_bonus,
    DROP COLUMN IF EXISTS effect_conditions;
DELETE FROM combatant_states WHERE kind = 'effect';
ALTER TABLE combatant_states
    DROP CONSTRAINT IF EXISTS combatant_states_modifiers_valid,
    DROP CONSTRAINT IF EXISTS combatant_states_save_dc_valid,
    DROP CONSTRAINT IF EXISTS combatant_states_label_valid,
    DROP CONSTRAINT IF EXISTS combatant_states_audience_valid,
    DROP CONSTRAINT IF EXISTS combatant_states_effect_valid,
    DROP CONSTRAINT IF EXISTS combatant_states_kind_valid,
    ADD CONSTRAINT combatant_states_kind_valid CHECK (kind IN ('rage', 'hunters_mark_target', 'dodging', 'reckless')),
    DROP COLUMN IF EXISTS player_label,
    DROP COLUMN IF EXISTS audience,
    DROP COLUMN IF EXISTS player_visible,
    DROP COLUMN IF EXISTS triggers_fired,
    DROP COLUMN IF EXISTS trigger_max_triggers,
    DROP COLUMN IF EXISTS trigger_damage_type,
    DROP COLUMN IF EXISTS trigger_dice,
    DROP COLUMN IF EXISTS follows_key,
    DROP COLUMN IF EXISTS on_fail_effect,
    DROP COLUMN IF EXISTS save_dc,
    DROP COLUMN IF EXISTS start_save_ability,
    DROP COLUMN IF EXISTS end_save_ability,
    DROP COLUMN IF EXISTS duration_kind,
    DROP COLUMN IF EXISTS modifiers,
    DROP COLUMN IF EXISTS condition_keys,
    DROP COLUMN IF EXISTS concentration,
    DROP COLUMN IF EXISTS source_kind,
    DROP COLUMN IF EXISTS source_key,
    DROP COLUMN IF EXISTS group_id;
