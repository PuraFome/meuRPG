-- +goose Up
-- puzzles are the campaign's puzzles (MR-038, RN-27, Etapa 10): what the master
-- makes, and the master only: no player ever reads this table (RN-10), the
-- solution least of all. A puzzle is played in a session through puzzle_runs.
--
-- kind is 'lights', 'lock' or 'pillars' (slice 10.7b adds the riddle, the
-- sequence and the cipher). It has no CHECK: the code is the list of kinds, so a
-- new kind needs no migration. config is the protojson of the kind's public
-- configuration (PuzzleConfig: the board's size, the lock's wheels and alphabet,
-- the pillars and their links), solution that of its answer (PuzzleSolution: the
-- lock's wheels, the pillars' mural; '{}' for the lights, whose goal is every
-- light off), and start that of the state it starts at (PuzzleState). The start
-- of the lights and of the pillars is generated from seed by package
-- rules/puzzle, so the same seed and config give the same start; a lock's start is
-- the master's choice and its seed is 0. minimum_moves is the fewest moves from
-- the start (NULL when the start cannot be solved, which the service refuses).
--
-- hints is a JSON array of texts, in the order the master releases them. The
-- "Ao resolver" action is solve_action ('notify', 'open_door', 'reveal_point' or
-- 'reveal_clue') and its target solve_target (protojson of the door, point or
-- clue; NULL for 'notify'). Ids inside solve_target have no foreign key: the
-- target can be deleted after, and the solve then says so (target_gone).
--
-- A puzzle is archived, never deleted. No personal data: fiction and numbers.
CREATE TABLE IF NOT EXISTS puzzles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id UUID NOT NULL REFERENCES campaigns (id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    name TEXT NOT NULL,
    config JSONB NOT NULL,
    solution JSONB NOT NULL DEFAULT '{}',
    seed INT8 NOT NULL DEFAULT 0,
    start JSONB NOT NULL,
    minimum_moves INT4 NULL,
    clue TEXT NOT NULL DEFAULT '',
    hints JSONB NOT NULL DEFAULT '[]',
    solve_action TEXT NOT NULL DEFAULT 'notify',
    solve_target JSONB NULL,
    archived_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT puzzles_name_valid CHECK (char_length(name) BETWEEN 1 AND 80),
    CONSTRAINT puzzles_clue_valid CHECK (char_length(clue) <= 500),
    CONSTRAINT puzzles_hints_valid CHECK (jsonb_typeof(hints) = 'array' AND jsonb_array_length(hints) <= 10),
    CONSTRAINT puzzles_solve_action_valid CHECK (solve_action IN ('notify', 'open_door', 'reveal_point', 'reveal_clue')),
    CONSTRAINT puzzles_minimum_moves_valid CHECK (minimum_moves IS NULL OR minimum_moves >= 0)
);

-- +goose Down
DROP TABLE IF EXISTS puzzles;
