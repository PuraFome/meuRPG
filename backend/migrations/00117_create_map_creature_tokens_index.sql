-- +goose Up
-- The ON DELETE CASCADE of map_creature_tokens.creature_id, and the tokens of
-- one creature (a dismissal that takes its tokens off the maps).
CREATE INDEX IF NOT EXISTS map_creature_tokens_creature_id_idx
    ON map_creature_tokens (creature_id);

-- +goose Down
DROP INDEX IF EXISTS map_creature_tokens_creature_id_idx;
