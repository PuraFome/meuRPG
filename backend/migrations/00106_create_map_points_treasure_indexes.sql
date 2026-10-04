-- +goose Up
-- The indexes of the treasure columns of map_points (00102), one statement each,
-- all safe to run again. Both are partial: most points are not treasures.
--
-- The treasures found in a game session: the session summary's "Mais tesouro
-- encontrado" (slice 9.11).
CREATE INDEX IF NOT EXISTS map_points_treasure_session_id_idx
    ON map_points (treasure_session_id) WHERE treasure_session_id IS NOT NULL;

-- The treasures an XP award converted: undoing the award (slice 9.11).
CREATE INDEX IF NOT EXISTS map_points_treasure_converted_award_id_idx
    ON map_points (treasure_converted_award_id) WHERE treasure_converted_award_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS map_points_treasure_converted_award_id_idx;
DROP INDEX IF EXISTS map_points_treasure_session_id_idx;
