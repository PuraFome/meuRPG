-- +goose Up
-- grid_columns is the battle grid of a map (MR-013, RN-21): how many squares
-- of 1.5 m (5 ft) fit across the image's width. NULL means the map has no
-- grid yet, and a combat cannot start on it: the master sets one first
-- (MapService.SetMapGrid). The rows follow the image's proportions
-- (round(columns * height / width)), so they are never stored.
--
-- The range is checked in the next migration (00041), one change each.
ALTER TABLE maps
    ADD COLUMN IF NOT EXISTS grid_columns INT4 NULL;

-- +goose Down
ALTER TABLE maps
    DROP COLUMN IF EXISTS grid_columns;
