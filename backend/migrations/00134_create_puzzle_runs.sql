-- +goose Up
-- puzzle_runs is a puzzle played in a session (MR-038, RN-27): one row per
-- (session, puzzle), made when the master first shows the puzzle, or prepares a
-- new start for it, in the session. It is the live state everybody reads, so
-- every change to it goes through one transaction that locks the session first.
--
-- seed and start are where the run started: the puzzle's own, or a new one the
-- master drew ("Gerar outro começo"); "Recomeçar" puts state back to start. state
-- is the protojson of PuzzleState, per kind, and changes only by relative moves
-- (press, turn), so two players' moves made at the same time both count.
-- released_hints is how many of the puzzle's hints the players may read.
--
-- shown_at is when the master showed it (NULL: prepared, not shown) and closed_at
-- when the master hid it again. Once solved_at is set the state is frozen: it
-- stops moving, and only the master's reset, new start or close change it again.
-- solved_by_character_id is the character whose move solved it,
-- last_mover_character_id the one that moved last, and last_move the protojson of
-- that move with what changed (for the players' "Lia girou o pilar 1"). solve_outcome
-- is what the "Ao resolver" action did (door_opened, ...). moves_made counts every
-- move of the run, resets included: the history stays in puzzle_moves. revision goes
-- up with every change; the app applies a read only if it is larger.
--
-- The characters have no cascade: deleting one only forgets who it was. No
-- personal data: fiction, numbers and IDs.
CREATE TABLE IF NOT EXISTS puzzle_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    game_session_id UUID NOT NULL REFERENCES game_sessions (id) ON DELETE CASCADE,
    puzzle_id UUID NOT NULL REFERENCES puzzles (id) ON DELETE CASCADE,
    seed INT8 NOT NULL DEFAULT 0,
    start JSONB NOT NULL,
    state JSONB NOT NULL,
    released_hints INT4 NOT NULL DEFAULT 0,
    shown_at TIMESTAMPTZ NULL,
    closed_at TIMESTAMPTZ NULL,
    solved_at TIMESTAMPTZ NULL,
    solved_by_character_id UUID NULL REFERENCES characters (id) ON DELETE SET NULL,
    solve_outcome TEXT NULL,
    last_mover_character_id UUID NULL REFERENCES characters (id) ON DELETE SET NULL,
    last_move JSONB NULL,
    last_moved_at TIMESTAMPTZ NULL,
    moves_made INT4 NOT NULL DEFAULT 0,
    revision INT4 NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT puzzle_runs_game_session_id_puzzle_id_key UNIQUE (game_session_id, puzzle_id),
    CONSTRAINT puzzle_runs_released_hints_valid CHECK (released_hints BETWEEN 0 AND 10),
    CONSTRAINT puzzle_runs_moves_made_valid CHECK (moves_made >= 0),
    CONSTRAINT puzzle_runs_revision_valid CHECK (revision >= 1)
);

-- +goose Down
DROP TABLE IF EXISTS puzzle_runs;
