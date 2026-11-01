-- +goose Up
-- A held d20 can count as a higher number than it came up (a table's roll_minimum, Reliable Talent).
-- The question shows the face as it came up, the number it counted as and the feature that did it.
-- The row keeps the feature with the faces and the total.
-- The question reads the same when it is asked, read again and retried.
--   - counted_by is the content key of that feature, '' when the die counts as it came up.
ALTER TABLE inspiration_holds ADD COLUMN IF NOT EXISTS counted_by TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE inspiration_holds DROP COLUMN IF EXISTS counted_by;
