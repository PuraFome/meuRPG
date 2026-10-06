-- +goose Up
-- What the players read when a puzzle is solved (MR-038, RN-27, "Ao resolver"): the
-- master's own text, or a generic line for what the action did ("Uma porta se
-- abriu."), worked out and kept in the winning move's transaction, so the point's
-- name it may carry is the one it had then. NULL until solved, and when there is
-- nothing to say. Fiction only.
ALTER TABLE puzzle_runs
    ADD COLUMN IF NOT EXISTS solve_message TEXT NULL;

-- +goose Down
ALTER TABLE puzzle_runs
    DROP COLUMN IF EXISTS solve_message;
