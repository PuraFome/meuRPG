-- +goose Up
-- The doors layer of a map (MR-010, RN-26, Etapa 10): four bits a square, in the
-- byte layout of package rules/grid (layer.go): row-major, square n is the low
-- nibble of byte n/2 when n is even and the high nibble when n is odd. A nibble
-- holds a door state: 0 none, 1 open, 2 closed, 3 locked, 4 barred, 5 secret. A
-- grid of 200 x 400 squares makes at most 40 KB. Like the other layers it is
-- NULL when no door is painted, the API sizes it by the map's grid when it
-- writes it, and a new grid or a new image deletes it with the rest of the row.
ALTER TABLE map_layers
    ADD COLUMN IF NOT EXISTS doors BYTEA NULL;

-- +goose Down
ALTER TABLE map_layers
    DROP COLUMN IF EXISTS doors;
