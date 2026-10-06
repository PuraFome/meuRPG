-- +goose Up
-- The mode is 'grid' or 'theatre', and a combat without a grid has none: it has
-- no map, no map point and a grid of 0 by 0 (a combat on a grid keeps the limits
-- it always had). The named CHECKs are dropped, if present, and added again, so
-- re-running this migration is safe.
ALTER TABLE encounters
    DROP CONSTRAINT IF EXISTS encounters_mode_valid,
    ADD CONSTRAINT encounters_mode_valid CHECK (mode IN ('grid', 'theatre')),
    DROP CONSTRAINT IF EXISTS encounters_grid_valid,
    ADD CONSTRAINT encounters_grid_valid CHECK (
        (mode = 'grid' AND grid_columns BETWEEN 4 AND 200 AND grid_rows BETWEEN 1 AND 400)
        OR (mode = 'theatre' AND grid_columns = 0 AND grid_rows = 0 AND map_id IS NULL AND map_point_id IS NULL)
    );

-- +goose Down
-- Down is a development tool: a combat without a grid has no map and a grid of
-- 0 by 0, which the old CHECK refuses, so those combats are deleted (their
-- combatants, offers and events go with them).
DELETE FROM encounters WHERE mode = 'theatre';

ALTER TABLE encounters
    DROP CONSTRAINT IF EXISTS encounters_mode_valid,
    DROP CONSTRAINT IF EXISTS encounters_grid_valid,
    ADD CONSTRAINT encounters_grid_valid CHECK (grid_columns BETWEEN 4 AND 200 AND grid_rows BETWEEN 1 AND 400);
