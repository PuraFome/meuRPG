-- +goose Up
-- last_roll_pending says the last charge went from an item that rolls a d20 for it and the roll has not been made.
ALTER TABLE character_item_charges ADD COLUMN IF NOT EXISTS last_roll_pending BOOLEAN NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE character_item_charges DROP COLUMN IF EXISTS last_roll_pending;
