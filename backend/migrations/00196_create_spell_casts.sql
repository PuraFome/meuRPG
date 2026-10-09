-- +goose Up
-- spell_casts are the spells cast outside a combat (SRD 5.1, "Spellcasting"):
-- healing between fights, Mage Armor before one, rituals, and every spell that
-- takes minutes or hours to cast. One row is one casting, from its start to its
-- end, and the rows of a session are also its log of casts.
--
-- status is 'casting' (the spell takes time and the time has not passed: no slot is
-- spent yet, and the caster is concentrating on the casting), 'active' (it took
-- effect and lasts, or the caster concentrates on it), 'ended' (it took effect and is
-- over) or 'failed' (the casting did not finish; no slot was spent). end_reason says
-- why a cast is over: 'instant', 'dismissed', 'concentration', 'rest', 'interrupted',
-- 'combat_started' or 'caster_gone'. The app does not count a spell's duration: a
-- lasting spell ends by the master, the caster's concentration or a rest.
--
-- caster_id is a character, a player's or an NPC (a cast by an NPC spends nothing).
-- slot_level and slot_pact record the slot (0 for a cantrip or a ritual); the slot
-- itself is spent in character_vitals when the casting finishes. concentrating says
-- the caster concentrates on this cast now; at most one cast of a caster does
-- (spell_casts_one_concentration_idx). duration_seconds is the spell's timed
-- duration (NULL when it has none to count) and rest_ends the rest that is long
-- enough to end it ('short', 'long' or NULL).
--
-- targets lists what the cast did to each target, ids and numbers only: the character,
-- the effect, the amount and the hit points before and after, the armor class it gave.
-- roll_faces are the faces of the dice a healing spell rolled. creature_ids are the
-- creatures a summoning spell made. secret says an NPC that is not on the stage cast
-- it: the master alone reads it (RN-10). carried_encounter_id is the combat that
-- holds the caster's concentration now (the combatant carries it, and gives it back
-- when the combat ends).
--
-- No personal data: ids, spell keys and numbers. Deleting the campaign, the session
-- or the caster deletes its casts.
CREATE TABLE IF NOT EXISTS spell_casts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id UUID NOT NULL REFERENCES campaigns (id) ON DELETE CASCADE,
    game_session_id UUID NOT NULL REFERENCES game_sessions (id) ON DELETE CASCADE,
    caster_id UUID NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    spell_key TEXT NOT NULL,
    ritual BOOL NOT NULL DEFAULT false,
    slot_level INT4 NOT NULL DEFAULT 0,
    slot_pact BOOL NOT NULL DEFAULT false,
    status TEXT NOT NULL,
    end_reason TEXT NULL,
    concentrating BOOL NOT NULL DEFAULT false,
    casting_minutes INT4 NOT NULL DEFAULT 0,
    lasts BOOL NOT NULL DEFAULT false,
    duration_seconds INT4 NULL,
    rest_ends TEXT NULL,
    secret BOOL NOT NULL DEFAULT false,
    targets JSONB NOT NULL DEFAULT '[]',
    dice_count INT4 NOT NULL DEFAULT 0,
    dice_sides INT4 NOT NULL DEFAULT 0,
    roll_faces INT4[] NOT NULL DEFAULT '{}',
    roll_total INT4 NOT NULL DEFAULT 0,
    physical BOOL NOT NULL DEFAULT false,
    creature_ids TEXT[] NOT NULL DEFAULT '{}',
    carried_encounter_id UUID NULL REFERENCES encounters (id) ON DELETE SET NULL,
    started_at TIMESTAMPTZ NOT NULL,
    cast_at TIMESTAMPTZ NULL,
    ended_at TIMESTAMPTZ NULL,
    CONSTRAINT spell_casts_spell_key_length CHECK (char_length(spell_key) BETWEEN 1 AND 100),
    CONSTRAINT spell_casts_status_valid CHECK (status IN ('casting', 'active', 'ended', 'failed')),
    CONSTRAINT spell_casts_end_reason_valid CHECK (
        end_reason IS NULL OR end_reason IN ('instant', 'dismissed', 'concentration', 'rest', 'interrupted', 'combat_started', 'caster_gone')
    ),
    CONSTRAINT spell_casts_over_valid CHECK ((status IN ('ended', 'failed')) = (ended_at IS NOT NULL AND end_reason IS NOT NULL)),
    CONSTRAINT spell_casts_slot_valid CHECK (slot_level BETWEEN 0 AND 9),
    CONSTRAINT spell_casts_minutes_valid CHECK (casting_minutes >= 0),
    CONSTRAINT spell_casts_duration_valid CHECK (duration_seconds IS NULL OR duration_seconds > 0),
    CONSTRAINT spell_casts_rest_valid CHECK (rest_ends IS NULL OR rest_ends IN ('short', 'long')),
    CONSTRAINT spell_casts_dice_valid CHECK (dice_count BETWEEN 0 AND 100 AND dice_sides BETWEEN 0 AND 100)
);

-- The kinds of session event the casts write (ADR-0007). Their payloads hold ids and
-- numbers only.
INSERT INTO session_event_kinds (kind) VALUES
    ('spell_cast_outside'),
    ('spell_cast_started'),
    ('spell_cast_finished'),
    ('spell_cast_interrupted'),
    ('spell_cast_ended')
ON CONFLICT (kind) DO NOTHING;

-- +goose Down
DELETE FROM session_event_kinds WHERE kind IN (
    'spell_cast_outside', 'spell_cast_started', 'spell_cast_finished', 'spell_cast_interrupted', 'spell_cast_ended'
);

DROP TABLE IF EXISTS spell_casts;
