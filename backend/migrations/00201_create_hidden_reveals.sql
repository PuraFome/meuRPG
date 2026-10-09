-- +goose Up
-- A hidden reveal is the question a player's area spell leaves the master when it
-- hit hidden creatures and the table rule asks (campaign_table_rules.
-- hidden_area_hits = 'ask'): reveal them to the players, or keep them hidden. The
-- spell already took effect on them; the question only decides whether the players
-- learn of it. While a question is 'pending' the combat's turn waits (the server
-- refuses the moves, attacks, actions, spells and the end of the turn), and the
-- players are told only that the master is being waited for.
--
--   - caster_id: the combatant that cast. Deleting it (it left the combat) deletes
--     the question, and the turn is free.
--   - combatant_ids: the hidden creatures the area hit (combatant UUIDs, as text),
--     in the combat's order. They are not foreign keys: a creature that left the
--     combat is simply not there when the master answers.
--   - origin_col, origin_row and squares (col, row, col, row...): where the area
--     landed, so the master's map can draw it again after a reload.
--   - seq: the order the questions were opened in, one count for each combat; they
--     are answered in this order.
--   - state: 'pending', 'revealed' or 'kept'. An answered question stays, so that a
--     repeated answer is told apart from the other one.
--
-- Ending the combat deletes its questions. No personal data: IDs and squares.
CREATE TABLE IF NOT EXISTS hidden_reveals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    encounter_id UUID NOT NULL REFERENCES encounters (id) ON DELETE CASCADE,
    caster_id UUID NOT NULL REFERENCES combatants (id) ON DELETE CASCADE,
    spell_key TEXT NOT NULL,
    combatant_ids TEXT[] NOT NULL,
    origin_col INT4 NOT NULL,
    origin_row INT4 NOT NULL,
    squares INT4[] NOT NULL DEFAULT '{}',
    seq INT4 NOT NULL,
    state TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL,
    answered_at TIMESTAMPTZ NULL,
    CONSTRAINT hidden_reveals_encounter_id_seq_key UNIQUE (encounter_id, seq),
    CONSTRAINT hidden_reveals_state_valid CHECK (state IN ('pending', 'revealed', 'kept')),
    CONSTRAINT hidden_reveals_square_valid CHECK (origin_col >= 0 AND origin_row >= 0),
    CONSTRAINT hidden_reveals_combatants_valid CHECK (cardinality(combatant_ids) BETWEEN 1 AND 40),
    CONSTRAINT hidden_reveals_squares_valid CHECK (cardinality(squares) % 2 = 0)
);

-- The answer of a question is an event of its own, so that a retry of the same
-- answer is the same answer.
INSERT INTO session_event_kinds (kind) VALUES ('hidden_reveal_answered') ON CONFLICT (kind) DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS hidden_reveals;
DELETE FROM session_event_kinds WHERE kind = 'hidden_reveal_answered';
