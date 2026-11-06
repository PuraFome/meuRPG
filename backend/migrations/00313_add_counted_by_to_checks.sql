-- +goose Up
-- A table's roll_minimum can raise the d20 of a puzzle hint try and of an initiative.
-- The row keeps the face as it came up and says which feature raised it.
-- A retry and a read tell the same.
--   - puzzle_hint_tries.counted_by is the content key of the feature whose floor the total counts, '' when the d20 counted as it came up.
--   - combatants.initiative_counted_as is the number the initiative d20 counted as, NULL when it counted as it came up.
--   - combatants.initiative_counted_by is the content key of the feature that did it, '' when none did.
ALTER TABLE puzzle_hint_tries ADD COLUMN IF NOT EXISTS counted_by TEXT NOT NULL DEFAULT '';
ALTER TABLE combatants ADD COLUMN IF NOT EXISTS initiative_counted_as INT4 NULL;
ALTER TABLE combatants ADD COLUMN IF NOT EXISTS initiative_counted_by TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE combatants DROP COLUMN IF EXISTS initiative_counted_by;
ALTER TABLE combatants DROP COLUMN IF EXISTS initiative_counted_as;
ALTER TABLE puzzle_hint_tries DROP COLUMN IF EXISTS counted_by;
