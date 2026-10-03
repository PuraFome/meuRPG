-- +goose Up
-- scene_clues holds the clues the master prepared on an RP scene point
-- (MR-029, Etapa 8, D5): one short text each, at most 30 per point (the API
-- checks, inside the transaction that inserts one). Only a point of kind
-- 'scene' has any (the API checks that too, and deletes them when the point
-- changes kind). The text is free text the master writes, 1 to 500
-- characters. A player never receives a clue from here: what a player gets is
-- a copy made when the master reveals it (scene_clue_reveals, 00070).
--
-- position orders the clues of a point, from 0. It is not unique: a move
-- renumbers the point's clues in one transaction. Deleting the point deletes
-- its clues (CASCADE).
CREATE TABLE IF NOT EXISTS scene_clues (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    point_id UUID NOT NULL REFERENCES map_points (id) ON DELETE CASCADE,
    position INT4 NOT NULL,
    text TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT scene_clues_position_valid CHECK (position >= 0),
    CONSTRAINT scene_clues_text_length CHECK (char_length(text) BETWEEN 1 AND 500)
);

-- +goose Down
DROP TABLE IF EXISTS scene_clues;
