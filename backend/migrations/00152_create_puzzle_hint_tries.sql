-- +goose Up
-- puzzle_hint_tries is every try for a hint by a skill check (MR-038, RN-27, RN-18): a
-- player rolls their character's skill against the DC the master set (puzzles.hint_dc),
-- and a pass gives that player, and only them, the next hint they do not read yet.
-- One row per try, in a run.
--
-- hint_index is the hint the try was for (from 0: the first of puzzles.hints): the
-- first one the player does not read, that is the one after the master's released
-- ones (puzzle_runs.released_hints) and after the ones the player won. A player tries
-- once for each hint: the unique (run_id, user_id, hint_index) is the limit. granted_count
-- is how many hints the player reads after a pass (hint_index + 1), NULL for a fail: the
-- hints a player reads are the master's released ones, or this number, whichever is
-- larger. d20, modifier and total are the roll (physical says the player typed the
-- face of a real die, RN-18); the DC is not kept here, only whether it was reached.
-- idempotency_key is the UUID the app made for the try: the same key twice returns
-- the first answer.
--
-- The user and the character are kept while they exist (SET NULL, RN-16). No personal
-- data: fiction, numbers and IDs.
CREATE TABLE IF NOT EXISTS puzzle_hint_tries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id UUID NOT NULL REFERENCES puzzle_runs (id) ON DELETE CASCADE,
    user_id UUID NULL REFERENCES users (id) ON DELETE SET NULL,
    character_id UUID NULL REFERENCES characters (id) ON DELETE SET NULL,
    idempotency_key UUID NOT NULL,
    hint_index INT4 NOT NULL,
    passed BOOL NOT NULL,
    granted_count INT4 NULL,
    d20 INT4 NOT NULL,
    modifier INT4 NOT NULL,
    total INT4 NOT NULL,
    physical BOOL NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT puzzle_hint_tries_run_id_idempotency_key_key UNIQUE (run_id, idempotency_key),
    CONSTRAINT puzzle_hint_tries_run_id_user_id_hint_index_key UNIQUE (run_id, user_id, hint_index),
    CONSTRAINT puzzle_hint_tries_hint_index_valid CHECK (hint_index BETWEEN 0 AND 9),
    CONSTRAINT puzzle_hint_tries_d20_valid CHECK (d20 BETWEEN 1 AND 20),
    CONSTRAINT puzzle_hint_tries_granted_valid CHECK ((passed AND granted_count = hint_index + 1) OR (NOT passed AND granted_count IS NULL))
);

-- +goose Down
DROP TABLE IF EXISTS puzzle_hint_tries;
