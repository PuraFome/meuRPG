-- +goose Up
-- Deleting a character sets puzzle_hint_tries.character_id to NULL: the index keeps
-- that from scanning every try (MR-038).
CREATE INDEX IF NOT EXISTS puzzle_hint_tries_character_id_idx
    ON puzzle_hint_tries (character_id);

-- +goose Down
DROP INDEX IF EXISTS puzzle_hint_tries_character_id_idx;
