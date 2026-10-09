-- +goose Up
-- A combat effect is something that lasts on one or more combatants of an
-- encounter: a spell (Bênção, Imobilizar Pessoa, Velocidade, Teia), an action
-- (Esquivar), what a spell leaves when it ends (the lethargy of Velocidade) or
-- something the master put there (RN-22). It has a source, targets, a duration
-- and an end, and the conditions it gives, the modifiers it makes and the saving
-- throws and damage it brings with the turns.
--
-- group_id is the casting: every effect of one concentration shares it, and when
-- the concentration ends they all end. caster_id is who cast it (deleting the
-- caster deletes the effects that came from it: the master's own effects have no
-- caster). source_kind is 'spell', 'feature' or 'master' and source_key the spell's
-- or the catalog's key. condition_keys and modifiers are what it does (modifiers
-- is the list of rules.EffectModifier, at most 8 entries).
--
-- The clock: started_round is the round it began in. duration_kind says how long
-- it lasts ('rounds', 'until_start_of_turn_of', 'until_end_of_turn_of',
-- 'concentration', 'until_dismissed', 'long_rest'). ends_round, ends_combatant_id
-- and ends_phase ('start' or 'end') are the moment it ends, which the server
-- works out when it is made or when the master changes the duration: the turn of
-- ends_combatant_id in ends_round, at its start or its end (no combatant: at the
-- start of the round). All three are NULL for an effect with no end on the clock.
--
-- end_save_ability is the saving throw at the end of a target's turn, and
-- start_save_ability the one at its start; save_dc is the caster's DC, kept when
-- it is cast (NULL when the master gave none), and on_fail_effect the catalog
-- effect a failed start-of-turn save adds. trigger_dice, trigger_damage_type and
-- trigger_max_triggers are the damage dealt to who starts the turn under it, at
-- most trigger_max_triggers times for each target.
--
-- follows_key is the spell an effect follows (the lethargy a Velocidade leaves
-- says which one: "spell:haste").
--
-- Who reads it: player_visible is the master's switch (off, no player but the
-- target's own reads it), audience says 'all' (the table sees its label) or
-- 'owner' (only the target's player), player_label is the master's free label,
-- at most 30 characters.
--
-- Deleting the encounter deletes the effects. No personal data: IDs, keys,
-- numbers and the master's label.
CREATE TABLE IF NOT EXISTS combat_effects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    encounter_id UUID NOT NULL REFERENCES encounters (id) ON DELETE CASCADE,
    group_id UUID NOT NULL,
    source_kind TEXT NOT NULL,
    source_key TEXT NOT NULL,
    caster_id UUID NULL REFERENCES combatants (id) ON DELETE CASCADE,
    concentration BOOL NOT NULL DEFAULT false,
    condition_keys TEXT[] NOT NULL DEFAULT '{}',
    modifiers JSONB NOT NULL DEFAULT '[]',
    duration_kind TEXT NOT NULL,
    started_round INT4 NOT NULL,
    ends_round INT4 NULL,
    ends_combatant_id UUID NULL REFERENCES combatants (id) ON DELETE SET NULL,
    ends_phase TEXT NULL,
    end_save_ability TEXT NULL,
    start_save_ability TEXT NULL,
    save_dc INT4 NULL,
    on_fail_effect TEXT NULL,
    follows_key TEXT NULL,
    trigger_dice TEXT NULL,
    trigger_damage_type TEXT NULL,
    trigger_max_triggers INT4 NULL,
    player_visible BOOL NOT NULL DEFAULT true,
    audience TEXT NOT NULL DEFAULT 'all',
    player_label TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT combat_effects_source_kind_valid CHECK (source_kind IN ('spell', 'feature', 'master')),
    CONSTRAINT combat_effects_duration_kind_valid CHECK (duration_kind IN ('rounds', 'until_start_of_turn_of', 'until_end_of_turn_of', 'concentration', 'until_dismissed', 'long_rest')),
    CONSTRAINT combat_effects_ends_valid CHECK (
        (ends_round IS NULL AND ends_phase IS NULL AND ends_combatant_id IS NULL)
        OR (ends_round IS NOT NULL AND ends_round >= 1 AND ends_phase IN ('start', 'end'))
    ),
    CONSTRAINT combat_effects_audience_valid CHECK (audience IN ('all', 'owner')),
    CONSTRAINT combat_effects_label_valid CHECK (player_label IS NULL OR char_length(player_label) BETWEEN 1 AND 30),
    CONSTRAINT combat_effects_save_dc_valid CHECK (save_dc IS NULL OR save_dc BETWEEN 1 AND 40),
    CONSTRAINT combat_effects_modifiers_valid CHECK (jsonb_typeof(modifiers) = 'array' AND jsonb_array_length(modifiers) <= 8),
    CONSTRAINT combat_effects_conditions_valid CHECK (array_length(condition_keys, 1) IS NULL OR array_length(condition_keys, 1) <= 4)
);

-- The targets of an effect (Bênção has up to three) and how many times its damage
-- hit each one. Deleting the combatant takes it off the effect; an effect with no
-- target left is deleted by the service.
CREATE TABLE IF NOT EXISTS combat_effect_targets (
    effect_id UUID NOT NULL REFERENCES combat_effects (id) ON DELETE CASCADE,
    combatant_id UUID NOT NULL REFERENCES combatants (id) ON DELETE CASCADE,
    triggers_fired INT4 NOT NULL DEFAULT 0,
    PRIMARY KEY (effect_id, combatant_id),
    CONSTRAINT combat_effect_targets_triggers_valid CHECK (triggers_fired >= 0)
);

-- A saving throw an effect asks of a target at the start or the end of its turn:
-- the window the target's player (or the master, for an NPC) answers. state is
-- 'open' until it is answered, then 'answered'; the master ending the effect, or
-- the caster losing the concentration, closes it with a closed_reason
-- ('EFFECT_ENDED' or 'CASTER_LOST_CONCENTRATION') and state 'closed'. d20,
-- bonus, total and saved are the answer. The unique index makes a repeated end
-- of turn open it once.
CREATE TABLE IF NOT EXISTS combat_effect_windows (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    encounter_id UUID NOT NULL REFERENCES encounters (id) ON DELETE CASCADE,
    effect_id UUID NULL REFERENCES combat_effects (id) ON DELETE SET NULL,
    combatant_id UUID NOT NULL REFERENCES combatants (id) ON DELETE CASCADE,
    phase TEXT NOT NULL,
    round INT4 NOT NULL,
    state TEXT NOT NULL DEFAULT 'open',
    closed_reason TEXT NULL,
    d20 INT4 NULL,
    bonus INT4 NULL,
    total INT4 NULL,
    saved BOOL NULL,
    created_at TIMESTAMPTZ NOT NULL,
    answered_at TIMESTAMPTZ NULL,
    CONSTRAINT combat_effect_windows_phase_valid CHECK (phase IN ('start', 'end')),
    CONSTRAINT combat_effect_windows_state_valid CHECK (state IN ('open', 'answered', 'closed')),
    CONSTRAINT combat_effect_windows_reason_valid CHECK (closed_reason IS NULL OR closed_reason IN ('EFFECT_ENDED', 'CASTER_LOST_CONCENTRATION'))
);

-- +goose Down
DROP TABLE IF EXISTS combat_effect_windows;
DROP TABLE IF EXISTS combat_effect_targets;
DROP TABLE IF EXISTS combat_effects;
