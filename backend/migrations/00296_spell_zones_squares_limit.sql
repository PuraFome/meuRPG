-- +goose Up
-- A zone keeps the squares it covers, at most 4,096. Fog Cloud cast with a 9th-level slot is a sphere of 180 feet
-- of radius (20 feet, and 20 more for each slot above the 1st): 4,053 squares of 5 feet. The check is dropped when present and added again, so running it
-- twice is safe on PostgreSQL and on CockroachDB.
ALTER TABLE spell_zones DROP CONSTRAINT IF EXISTS spell_zones_squares_valid;
ALTER TABLE spell_zones ADD CONSTRAINT spell_zones_squares_valid
    CHECK (jsonb_typeof(squares) = 'array' AND jsonb_array_length(squares) <= 4096);

-- +goose Down
-- A zone of more than 1,024 squares does not fit the 1,024 limit: those rows go first.
DELETE FROM spell_zones WHERE jsonb_array_length(squares) > 1024;
ALTER TABLE spell_zones DROP CONSTRAINT IF EXISTS spell_zones_squares_valid;
ALTER TABLE spell_zones ADD CONSTRAINT spell_zones_squares_valid
    CHECK (jsonb_typeof(squares) = 'array' AND jsonb_array_length(squares) <= 1024);
