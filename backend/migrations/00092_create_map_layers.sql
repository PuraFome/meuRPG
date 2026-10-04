-- +goose Up
-- map_layers holds the squares the master painted on a map's grid (MR-034,
-- MR-036, Etapa 9, D2): difficult terrain, walls, cover and light, each one a
-- packed bitmap in the byte layout of package rules/grid (layer.go): row-major,
-- one bit a square for terrain and walls, two bits for cover (0 none, 1 half, 2
-- three-quarters) and light (0 not painted, 1 dark, 2 dim, 3 bright). A grid of
-- 200 x 400 squares makes at most 10 KB for a one-bit layer and 20 KB for a
-- two-bit one.
--
-- They live in a table of their own, not in four columns of maps, because
-- maps is read whole (SELECT *) by almost every map call, and 60 KB would go
-- along each time. A map with nothing painted has no row, and a layer with
-- nothing painted is NULL: the API reads both as "nothing painted". The API
-- sizes every layer by the map's grid when it writes, and refuses (as a
-- wrong length) a layer that does not fit when it reads one, so changing the
-- grid's columns or the map's image deletes the row.
--
-- Deleting the map deletes its layers. There is no personal data here: squares.
CREATE TABLE IF NOT EXISTS map_layers (
    map_id UUID PRIMARY KEY REFERENCES maps (id) ON DELETE CASCADE,
    difficult_terrain BYTEA NULL,
    walls BYTEA NULL,
    cover BYTEA NULL,
    light BYTEA NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS map_layers;
