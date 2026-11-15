-- +goose Up
-- The master can take an extra out of a group of parts that share a damage type. What the damage did is undone and the
-- damage waits for the sum of that damage type alone, in the new status 'awaiting_group_sum'. awaited_damage_type_key is
-- that damage type's content key ("damage-type:piercing"); it is empty in every other status. The CHECK lists every status,
-- dropped, if present, and added again, so running this migration twice is safe. Nothing that exists changes: a row keeps
-- its status and gets the empty key.
ALTER TABLE pending_damages
    ADD COLUMN IF NOT EXISTS awaited_damage_type_key TEXT NOT NULL DEFAULT '';

ALTER TABLE pending_damages
    DROP CONSTRAINT IF EXISTS pending_damages_status_valid,
    ADD CONSTRAINT pending_damages_status_valid CHECK (status IN ('awaiting_reaction', 'awaiting_roll', 'awaiting_group_sum', 'rolled', 'applied', 'discarded'));

-- +goose Down
UPDATE pending_damages SET status = 'awaiting_roll' WHERE status = 'awaiting_group_sum';

ALTER TABLE pending_damages
    DROP CONSTRAINT IF EXISTS pending_damages_status_valid,
    ADD CONSTRAINT pending_damages_status_valid CHECK (status IN ('awaiting_reaction', 'awaiting_roll', 'rolled', 'applied', 'discarded'));

ALTER TABLE pending_damages
    DROP COLUMN IF EXISTS awaited_damage_type_key;
