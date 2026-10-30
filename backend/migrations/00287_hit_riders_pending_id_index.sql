-- +goose Up
-- The number 00286 is held by another branch; the gap is deliberate.
-- Undoing the damage that opened a creature's rider takes the offer back (DeleteHitRidersOfPending) and
-- the offer is looked up by it when the damage lands (CountHitRidersOfPending): both read hit_riders by
-- pending_id, a column 00285 added with no index. Nothing but the index changes, so running it twice is safe.
CREATE INDEX IF NOT EXISTS hit_riders_pending_id_idx ON hit_riders (pending_id);

-- +goose Down
DROP INDEX IF EXISTS hit_riders_pending_id_idx;
