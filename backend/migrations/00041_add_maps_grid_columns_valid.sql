-- +goose Up
-- A grid has 4 to 200 columns: fewer is not a grid, more is not readable
-- on a phone. NULL (no grid) passes. The named CHECK is dropped, if
-- present, and added again, so re-running this migration is safe.
ALTER TABLE maps
    DROP CONSTRAINT IF EXISTS maps_grid_columns_valid,
    ADD CONSTRAINT maps_grid_columns_valid CHECK (grid_columns IS NULL OR grid_columns BETWEEN 4 AND 200);

-- +goose Down
ALTER TABLE maps
    DROP CONSTRAINT IF EXISTS maps_grid_columns_valid;
