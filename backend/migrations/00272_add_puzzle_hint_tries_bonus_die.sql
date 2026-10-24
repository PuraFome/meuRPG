-- +goose Up
-- A puzzle hint try keeps the Bardic Inspiration die the player added after the d20 (SRD 5.1: the
-- die adds to an ability check), so a retry of the try's request answers with the die as the first
-- answer did. modifier and total already hold it; these say which die it was.
--   - bonus_sides is the die's sides, bonus_face what it showed; both NULL when no die was added.
--
-- Production is live: the columns only add, and the checks are dropped before they are added, so
-- running the migration twice is safe.
ALTER TABLE puzzle_hint_tries ADD COLUMN IF NOT EXISTS bonus_sides INT4 NULL;
ALTER TABLE puzzle_hint_tries ADD COLUMN IF NOT EXISTS bonus_face INT4 NULL;

ALTER TABLE puzzle_hint_tries DROP CONSTRAINT IF EXISTS puzzle_hint_tries_bonus_die_valid;
ALTER TABLE puzzle_hint_tries ADD CONSTRAINT puzzle_hint_tries_bonus_die_valid CHECK (
    (bonus_sides IS NULL AND bonus_face IS NULL)
    OR (bonus_sides >= 2 AND bonus_sides <= 100 AND bonus_face >= 1 AND bonus_face <= bonus_sides)
);

-- +goose Down
ALTER TABLE puzzle_hint_tries DROP CONSTRAINT IF EXISTS puzzle_hint_tries_bonus_die_valid;
ALTER TABLE puzzle_hint_tries DROP COLUMN IF EXISTS bonus_face;
ALTER TABLE puzzle_hint_tries DROP COLUMN IF EXISTS bonus_sides;
