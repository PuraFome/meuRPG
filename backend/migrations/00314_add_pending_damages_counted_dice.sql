-- +goose Up
-- A table's roll_minimum can raise a die of damage. A damage rolled with no parts of its own keeps what each die counted
-- as in the pending damage, next to the faces as they came up, so a read after the roll tells the same as the roll did.
-- counted_dice is a JSON list of {i, face, to, source_key}, one for each die the minimum changed; empty when none was.
-- The damage of a hit with parts keeps the same in its part_rolls.
ALTER TABLE pending_damages ADD COLUMN IF NOT EXISTS counted_dice JSONB NOT NULL DEFAULT '[]';

-- +goose Down
ALTER TABLE pending_damages DROP COLUMN IF EXISTS counted_dice;
