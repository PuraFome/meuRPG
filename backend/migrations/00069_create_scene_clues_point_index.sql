-- +goose Up
-- A point's clues in order: what every map read and every scene read asks.
CREATE INDEX IF NOT EXISTS scene_clues_point_id_position_idx
    ON scene_clues (point_id, position);

-- +goose Down
DROP INDEX IF EXISTS scene_clues_point_id_position_idx;
