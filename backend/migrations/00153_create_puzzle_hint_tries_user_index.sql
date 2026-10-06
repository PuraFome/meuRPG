-- +goose Up
-- Deleting an account sets puzzle_hint_tries.user_id to NULL (RN-16): the index keeps
-- that from scanning every try (MR-038).
CREATE INDEX IF NOT EXISTS puzzle_hint_tries_user_id_idx
    ON puzzle_hint_tries (user_id);

-- +goose Down
DROP INDEX IF EXISTS puzzle_hint_tries_user_id_idx;
