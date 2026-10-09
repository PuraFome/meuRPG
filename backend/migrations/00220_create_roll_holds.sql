-- +goose Up
-- roll_holds are the attack rolls that wait for a player's answer about a Bardic
-- Inspiration die (SRD 5.1, Bard: the creature "can wait until after it rolls the d20
-- before deciding to use the Bardic Inspiration die, but must decide before the GM says
-- whether the roll succeeds or fails"). A character that holds a die rolls its attack: the
-- d20 is rolled (or typed) and kept here with the request, and nothing is resolved, spent
-- or written to the history until the answer, which resolves the attack with that d20 and
-- the die added, or without it. request is the RollAttackRequest as it came (IDs, keys and
-- the way the d20 was given: no personal data); face is the d20 and modifier the attack's bonus
-- to it; round is the combat's
-- round, so that a hold the combatant never answered goes away when its round does;
-- answer_key is the idempotency key of the answer, set when it is given, so that a retry of
-- the answer finds the hold answered.
--
-- Deleting the encounter or the combatant deletes the hold. One statement per object, so
-- re-running this migration is safe.
CREATE TABLE IF NOT EXISTS roll_holds (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    encounter_id UUID NOT NULL REFERENCES encounters (id) ON DELETE CASCADE,
    combatant_id UUID NOT NULL REFERENCES combatants (id) ON DELETE CASCADE,
    idempotency_key TEXT NOT NULL,
    request BYTEA NOT NULL,
    face INT4 NOT NULL,
    modifier INT4 NOT NULL DEFAULT 0,
    round INT4 NOT NULL,
    answer_key TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT roll_holds_face_valid CHECK (face >= 1 AND face <= 20),
    CONSTRAINT roll_holds_round_valid CHECK (round >= 0),
    CONSTRAINT roll_holds_key_unique UNIQUE (encounter_id, idempotency_key)
);

-- +goose Down
DROP TABLE IF EXISTS roll_holds;
