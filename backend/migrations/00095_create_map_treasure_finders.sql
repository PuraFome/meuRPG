-- +goose Up
-- map_treasure_finders records who found a treasure (MR-041, Etapa 9, D8): the
-- characters the master picked when it marked the treasure found. Everyone who
-- sees the map reads them ("Encontrado por Brisa"), and the session summary's
-- "Mais tesouro encontrado" splits the treasure's value among them (slice 9.11).
-- Unmarking the treasure deletes its rows.
--
-- Deleting the treasure or the character deletes the row; there is no index by
-- character_id, as for map_tokens (00030). No personal data: IDs.
CREATE TABLE IF NOT EXISTS map_treasure_finders (
    point_id UUID NOT NULL REFERENCES map_points (id) ON DELETE CASCADE,
    character_id UUID NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    CONSTRAINT map_treasure_finders_pkey PRIMARY KEY (point_id, character_id)
);

-- +goose Down
DROP TABLE IF EXISTS map_treasure_finders;
