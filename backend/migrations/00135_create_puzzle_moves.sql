-- +goose Up
-- puzzle_moves is the history of a run's moves (MR-038, ADR-0007: the session's
-- events keep only when a puzzle was shown, solved, reset and closed; the moves
-- have their own table). One row per move that changed the run, in the order the
-- server applied them: seq counts from 1 in the run.
--
-- idempotency_key is the UUID the app made for the move: a second call with the
-- same key finds the row and changes nothing, answering as the first did. move is
-- the protojson of the PuzzleMove (relative: what was done, never what the state
-- became), revision the run's revision right after it, and solved is true for the
-- move that solved the puzzle. The user and the character that moved are kept
-- while they exist; they are also what a later "attempts left of each player"
-- (slice 10.7b) counts.
CREATE TABLE IF NOT EXISTS puzzle_moves (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id UUID NOT NULL REFERENCES puzzle_runs (id) ON DELETE CASCADE,
    seq INT4 NOT NULL,
    user_id UUID NULL REFERENCES users (id) ON DELETE SET NULL,
    character_id UUID NULL REFERENCES characters (id) ON DELETE SET NULL,
    idempotency_key UUID NOT NULL,
    move JSONB NOT NULL,
    revision INT4 NOT NULL,
    solved BOOL NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT puzzle_moves_run_id_seq_key UNIQUE (run_id, seq),
    CONSTRAINT puzzle_moves_run_id_idempotency_key_key UNIQUE (run_id, idempotency_key),
    CONSTRAINT puzzle_moves_seq_valid CHECK (seq >= 1)
);

-- +goose Down
DROP TABLE IF EXISTS puzzle_moves;
