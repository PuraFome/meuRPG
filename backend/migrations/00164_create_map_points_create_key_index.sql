-- +goose Up
-- One point per create_key (which carries the campaign's ID): the unique index is what
-- makes the idempotency hold when two calls with the same key race. Points without a
-- key are left out of the index.
CREATE UNIQUE INDEX IF NOT EXISTS map_points_create_key_idx
    ON map_points (create_key)
    WHERE create_key IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS map_points_create_key_idx;
