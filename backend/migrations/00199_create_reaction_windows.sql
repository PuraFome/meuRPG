-- +goose Up
-- The reaction window (PM-04): a question to a reactor, "may you react?", that
-- holds the action that triggered it until it is answered. One table serves the
-- seven reactions (Shield, Uncanny Dodge, Hellish Rebuke, Counterspell, Cutting
-- Words, Deflect Missiles, Feather Fall), the concentration save and the master's
-- one-tap check of the table rule "Reações dos inimigos".
--
--   - group_id is the trigger the window belongs to: the windows of one action
--     (two players that could counter the same spell) share it, and the action
--     goes on when none of the group is open.
--   - seq is the order they are answered in (the order they were opened, which
--     is the reactors' initiative order).
--   - kind is the reaction; status is 'open', 'answered' or 'closed' (closed by
--     itself, with closed_reason: the reactor spent its reaction, was
--     incapacitated, or the trigger is gone).
--   - reactor_id is the combatant that reacts (null for the master's check).
--   - pending_damage_id is the hit or damage the window holds (Shield, Uncanny
--     Dodge, Deflect Missiles); hold_id is the held action (a cast waiting for a
--     Counterspell, a roll waiting for Cutting Words). A window with neither only
--     holds the turn (Hellish Rebuke, the concentration save).
--   - step is 1 for the first question, 2 for the one an answer asks (the
--     aggressor's saving throw, the monk's throw back).
--   - trigger and outcome are JSON of IDs and numbers only, never a name.
--
-- Deleting the encounter, the combatant or the pending damage deletes the row.
CREATE TABLE IF NOT EXISTS reaction_holds (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    encounter_id UUID NOT NULL REFERENCES encounters (id) ON DELETE CASCADE,
    group_id UUID NOT NULL,
    kind TEXT NOT NULL,
    actor_id UUID NOT NULL REFERENCES combatants (id) ON DELETE CASCADE,
    actor_user_id UUID NOT NULL,
    actor_is_master BOOL NOT NULL,
    request BYTEA NULL,
    data JSONB NOT NULL DEFAULT '{}',
    state TEXT NOT NULL DEFAULT 'held',
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT reaction_holds_kind_valid CHECK (kind IN ('cast', 'attack', 'damage')),
    CONSTRAINT reaction_holds_state_valid CHECK (state IN ('held', 'released', 'dropped'))
);

CREATE INDEX IF NOT EXISTS reaction_holds_encounter_idx ON reaction_holds (encounter_id);

CREATE TABLE IF NOT EXISTS reaction_windows (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    encounter_id UUID NOT NULL REFERENCES encounters (id) ON DELETE CASCADE,
    seq INT8 NOT NULL DEFAULT unique_rowid(),
    group_id UUID NOT NULL,
    kind TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'open',
    closed_reason TEXT NULL,
    reactor_id UUID NULL REFERENCES combatants (id) ON DELETE CASCADE,
    pending_damage_id UUID NULL REFERENCES pending_damages (id) ON DELETE CASCADE,
    hold_id UUID NULL REFERENCES reaction_holds (id) ON DELETE CASCADE,
    step INT4 NOT NULL DEFAULT 1,
    trigger JSONB NOT NULL DEFAULT '{}',
    outcome JSONB NULL,
    created_at TIMESTAMPTZ NOT NULL,
    answered_at TIMESTAMPTZ NULL,
    CONSTRAINT reaction_windows_kind_valid CHECK (kind IN (
        'shield', 'uncanny_dodge', 'hellish_rebuke', 'counterspell', 'cutting_words',
        'deflect_missiles', 'feather_fall', 'concentration_save', 'master_check'
    )),
    CONSTRAINT reaction_windows_status_valid CHECK (status IN ('open', 'answered', 'closed')),
    CONSTRAINT reaction_windows_closed_reason_valid CHECK (
        closed_reason IS NULL OR closed_reason IN ('reaction_spent', 'reactor_incapacitated', 'trigger_gone')
    ),
    CONSTRAINT reaction_windows_step_valid CHECK (step IN (1, 2))
);

CREATE INDEX IF NOT EXISTS reaction_windows_encounter_idx ON reaction_windows (encounter_id, status, seq);

-- A new kind of session event for the answer of a window and for a concentration
-- save: IDs and numbers only (the combat log is built from them).
INSERT INTO session_event_kinds (kind) VALUES
    ('reaction_answered'),
    ('concentration_save_rolled')
ON CONFLICT (kind) DO NOTHING;

-- "Reações dos inimigos" (RN-24): when a player's action against an enemy waits for
-- the master. 'only_when_possible' (the default) waits only when an enemy has a
-- reaction; 'always' waits for the master's one tap on every such action.
ALTER TABLE campaign_table_rules
    ADD COLUMN IF NOT EXISTS enemy_reactions TEXT NOT NULL DEFAULT 'only_when_possible'
        CHECK (enemy_reactions IN ('only_when_possible', 'always'));

-- The spell slots a monster's stat block spent in a combat, by slot level
-- ({"1": 2}): a monster has no sheet with slots, so Shield and Counterspell of a
-- stat block (the Mage) are counted here. A player's character and an NPC with a
-- full sheet spend from character_vitals, as always.
ALTER TABLE combatants
    ADD COLUMN IF NOT EXISTS slots_used JSONB NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE combatants DROP COLUMN IF EXISTS slots_used;
ALTER TABLE campaign_table_rules DROP COLUMN IF EXISTS enemy_reactions;
DELETE FROM session_events WHERE kind IN ('reaction_answered', 'concentration_save_rolled');
DELETE FROM session_event_kinds WHERE kind IN ('reaction_answered', 'concentration_save_rolled');
DROP TABLE IF EXISTS reaction_windows;
DROP TABLE IF EXISTS reaction_holds;
