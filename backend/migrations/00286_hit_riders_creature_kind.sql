-- +goose Up
-- A creature's attack can offer the master a rider too (the save of a ghoul's claws, the grapple of
-- a constrictor): the offer is a hit_riders row of the kind 'creature', and action_key says which
-- action of the stat block it comes from ("monster:ghoul#claws"). The check on kind was written
-- inline, so it has the name its engine gave it (check_kind on CockroachDB, hit_riders_kind_check on
-- PostgreSQL); both are dropped when present.
ALTER TABLE hit_riders DROP CONSTRAINT IF EXISTS check_kind;
ALTER TABLE hit_riders DROP CONSTRAINT IF EXISTS hit_riders_kind_check;
ALTER TABLE hit_riders DROP CONSTRAINT IF EXISTS hit_riders_kind_valid;
ALTER TABLE hit_riders ADD CONSTRAINT hit_riders_kind_valid
    CHECK (kind IN ('open_hand', 'stunning_strike', 'creature'));
ALTER TABLE hit_riders ADD COLUMN IF NOT EXISTS action_key TEXT NULL;
-- pending_id is the damage whose landing opened the offer: undoing that damage takes the offer back.
ALTER TABLE hit_riders ADD COLUMN IF NOT EXISTS pending_id UUID NULL;

-- +goose Down
DELETE FROM hit_riders WHERE kind = 'creature';
ALTER TABLE hit_riders DROP COLUMN IF EXISTS pending_id;
ALTER TABLE hit_riders DROP COLUMN IF EXISTS action_key;
ALTER TABLE hit_riders DROP CONSTRAINT IF EXISTS hit_riders_kind_valid;
ALTER TABLE hit_riders DROP CONSTRAINT IF EXISTS hit_riders_kind_valid;
ALTER TABLE hit_riders ADD CONSTRAINT hit_riders_kind_valid
    CHECK (kind IN ('open_hand', 'stunning_strike'));
