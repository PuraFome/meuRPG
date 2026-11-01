-- +goose Up
-- A wall zone (Wall of Fire) keeps the side that deals damage and how the wall runs, so that the zone can rebuild which squares
-- hurt after a restart: wall_side is left or right of a straight wall, inner or outer of a ring, and empty for a zone that is no wall;
-- wall_dx and wall_dy are the way a straight wall runs from its origin; wall_length_ft is the straight wall's length or the ring's
-- diameter. The checks are dropped when present and added again, so running them twice is safe on PostgreSQL and on CockroachDB.
ALTER TABLE spell_zones ADD COLUMN IF NOT EXISTS wall_side TEXT NOT NULL DEFAULT '';
ALTER TABLE spell_zones ADD COLUMN IF NOT EXISTS wall_dx SMALLINT NOT NULL DEFAULT 0;
ALTER TABLE spell_zones ADD COLUMN IF NOT EXISTS wall_dy SMALLINT NOT NULL DEFAULT 0;
ALTER TABLE spell_zones ADD COLUMN IF NOT EXISTS wall_length_ft INTEGER NOT NULL DEFAULT 0;

ALTER TABLE spell_zones DROP CONSTRAINT IF EXISTS spell_zones_wall_side_valid;
ALTER TABLE spell_zones ADD CONSTRAINT spell_zones_wall_side_valid
    CHECK (wall_side IN ('', 'left', 'right', 'inner', 'outer'));
ALTER TABLE spell_zones DROP CONSTRAINT IF EXISTS spell_zones_wall_way_valid;
ALTER TABLE spell_zones ADD CONSTRAINT spell_zones_wall_way_valid
    CHECK (wall_dx BETWEEN -1 AND 1 AND wall_dy BETWEEN -1 AND 1);
ALTER TABLE spell_zones DROP CONSTRAINT IF EXISTS spell_zones_wall_length_valid;
ALTER TABLE spell_zones ADD CONSTRAINT spell_zones_wall_length_valid
    CHECK (wall_length_ft BETWEEN 0 AND 60 AND wall_length_ft % 5 = 0);
ALTER TABLE spell_zones DROP CONSTRAINT IF EXISTS spell_zones_wall_pair_valid;
ALTER TABLE spell_zones ADD CONSTRAINT spell_zones_wall_pair_valid
    CHECK ((wall_side = '') = (wall_length_ft = 0));

-- +goose Down
ALTER TABLE spell_zones DROP CONSTRAINT IF EXISTS spell_zones_wall_pair_valid;
ALTER TABLE spell_zones DROP CONSTRAINT IF EXISTS spell_zones_wall_length_valid;
ALTER TABLE spell_zones DROP CONSTRAINT IF EXISTS spell_zones_wall_way_valid;
ALTER TABLE spell_zones DROP CONSTRAINT IF EXISTS spell_zones_wall_side_valid;
ALTER TABLE spell_zones DROP COLUMN IF EXISTS wall_length_ft;
ALTER TABLE spell_zones DROP COLUMN IF EXISTS wall_dy;
ALTER TABLE spell_zones DROP COLUMN IF EXISTS wall_dx;
ALTER TABLE spell_zones DROP COLUMN IF EXISTS wall_side;
