-- +goose Up
-- Which way a generated dungeon's stair goes (MR-010, slice 10.14b): 'up' or 'down'. NULL for every
-- other point. A stair is its own thing on the map, not a submap (the app draws it with its arrow).
ALTER TABLE map_points ADD COLUMN IF NOT EXISTS stairs TEXT NULL;

-- +goose Down
ALTER TABLE map_points DROP COLUMN IF EXISTS stairs;
