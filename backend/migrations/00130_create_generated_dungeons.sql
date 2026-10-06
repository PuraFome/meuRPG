-- +goose Up
-- generated_dungeons is the record of a map made by DungeonService
-- (MR-010, RN-26, Etapa 10): one row per generated map, the master's alone (a
-- player never reads it, RN-10: the rooms list and the seed give away the
-- dungeon). Deleting the map deletes it.
--
-- options is the DungeonOptions message as used (defaults filled in, sizes made
-- odd) in its JSON form, and rooms is a GetDungeonRoomsResponse holding only its
-- rooms (floor, middle square and exits, with the doors' true kinds and the
-- trap flags), so the stored copy and the message are the same thing. The seed is
-- the generator's uint64 stored as its two's-complement INT8 (the same 64 bits).
-- generator_version says which algorithm made the cells: the cells are kept, so a
-- later generator version never changes an old map.
--
-- cells is the final grid of base kinds, two bits a square, row-major, square n
-- in bits 2 * (n % 4) of byte n / 4 from the low bit: 0 rock, 1 room floor, 2
-- corridor, 3 door. The image of the map is drawn from it (with the walls and the
-- doors layers) when the master asks to redraw it. 199 x 399 squares take at most
-- 20 KB.
--
-- No personal data: generated content, IDs and numbers.
CREATE TABLE IF NOT EXISTS generated_dungeons (
    map_id UUID NOT NULL REFERENCES maps (id) ON DELETE CASCADE,
    generator_version INT4 NOT NULL,
    seed INT8 NOT NULL,
    width INT4 NOT NULL,
    height INT4 NOT NULL,
    options JSONB NOT NULL,
    cells BYTEA NOT NULL,
    rooms JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT generated_dungeons_pkey PRIMARY KEY (map_id),
    CONSTRAINT generated_dungeons_size_valid CHECK (width BETWEEN 15 AND 199 AND height BETWEEN 15 AND 399),
    CONSTRAINT generated_dungeons_cells_size CHECK (length(cells) * 4 >= width * height AND length(cells) * 4 < width * height + 4)
);

-- +goose Down
DROP TABLE IF EXISTS generated_dungeons;
