-- +goose Up
-- The list of a map's points that keep an encounter (MR-043), and the map's own cascade.
CREATE INDEX IF NOT EXISTS battle_encounters_map_id_idx
    ON battle_encounters (map_id);

-- +goose Down
DROP INDEX IF EXISTS battle_encounters_map_id_idx;
