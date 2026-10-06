-- +goose Up
-- The mode of a combat (MR-025, RN-25, ADR-0017): 'grid' is the combat on a map
-- with a grid, as every combat has been, and 'theatre' is the combat without a
-- grid, the "teatro da mente". The master chooses it when the combat starts and
-- it never changes. Every combat that exists is a 'grid' one, so the default is
-- the backfill. In 'theatre' map_id and map_point_id are NULL, grid_columns and
-- grid_rows are 0, and no combatant has a square. The CHECKs are in the next
-- migration, which never writes the column it adds a CHECK to.
ALTER TABLE encounters
    ADD COLUMN IF NOT EXISTS mode TEXT NOT NULL DEFAULT 'grid';

-- +goose Down
ALTER TABLE encounters
    DROP COLUMN IF EXISTS mode;
