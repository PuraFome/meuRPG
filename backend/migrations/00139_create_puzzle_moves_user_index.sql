-- +goose Up
-- Deleting an account sets puzzle_moves.user_id to NULL (RN-16): the index keeps that
-- from scanning every move (MR-038).
CREATE INDEX IF NOT EXISTS puzzle_moves_user_id_idx
    ON puzzle_moves (user_id);

-- +goose Down
DROP INDEX IF EXISTS puzzle_moves_user_id_idx;
