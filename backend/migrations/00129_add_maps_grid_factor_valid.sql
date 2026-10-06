-- +goose Up
-- The factor is 1 to 20 (package rules/grid, MaxFactor), and a map with a grid
-- has a whole number of drawn columns: grid_columns is a multiple of it. The
-- named CHECKs are dropped, if present, and added again, so re-running this
-- migration is safe.
ALTER TABLE maps
    DROP CONSTRAINT IF EXISTS maps_grid_factor_valid,
    ADD CONSTRAINT maps_grid_factor_valid CHECK (grid_factor BETWEEN 1 AND 20 AND (grid_columns IS NULL OR grid_columns % grid_factor = 0));

-- +goose Down
ALTER TABLE maps
    DROP CONSTRAINT IF EXISTS maps_grid_factor_valid;
