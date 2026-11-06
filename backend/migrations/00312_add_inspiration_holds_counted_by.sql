-- +goose Up
-- A held d20 whose die counts as a higher number (a table's roll_minimum, Reliable Talent) tells it in the question: the
-- offer shows the face as it came up, the number it counted as and the feature that did it. The feature is kept on the row
-- with the faces and the total, so the question reads the same when it is asked, read again and retried.
--   - counted_by is the content key of that feature, '' when the die counts as it came up.
-- Production is live: the column only adds and has a default, so running the migration twice is safe.
ALTER TABLE inspiration_holds ADD COLUMN IF NOT EXISTS counted_by TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE inspiration_holds DROP COLUMN IF EXISTS counted_by;
