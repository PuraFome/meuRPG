-- +goose Up
-- Deleting a character sets puzzle_moves.character_id to NULL: the index keeps that
-- from scanning every move (MR-038).
CREATE INDEX IF NOT EXISTS puzzle_moves_character_id_idx
    ON puzzle_moves (character_id);

-- +goose Down
DROP INDEX IF EXISTS puzzle_moves_character_id_idx;
