-- +goose Up
-- A character has at most one bonded creature, its Find Steed steed (the bonded column of 00351). The unique partial index is
-- what refuses a second one. Nothing but the index changes, so running it twice is safe.
CREATE UNIQUE INDEX IF NOT EXISTS character_creatures_one_bonded_idx ON character_creatures (character_id) WHERE bonded;

-- +goose Down
-- CASCADE: CockroachDB keeps a unique index as a constraint and refuses to drop it without it.
DROP INDEX IF EXISTS character_creatures_one_bonded_idx CASCADE;
