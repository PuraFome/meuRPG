-- +goose Up
-- A point's actions in order: what every map read and every scene read asks.
CREATE INDEX IF NOT EXISTS scene_actions_point_id_position_idx
    ON scene_actions (point_id, position);

-- +goose Down
DROP INDEX IF EXISTS scene_actions_point_id_position_idx;
