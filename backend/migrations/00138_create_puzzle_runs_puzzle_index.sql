-- +goose Up
-- Which puzzles were ever shown (ListShownPuzzleIDs reads puzzle_id and shown_at
-- only, so the index covers it) and the cascade of a deleted puzzle (MR-038).
CREATE INDEX IF NOT EXISTS puzzle_runs_puzzle_id_idx
    ON puzzle_runs (puzzle_id) INCLUDE (shown_at);

-- +goose Down
DROP INDEX IF EXISTS puzzle_runs_puzzle_id_idx;
