-- +goose Up
-- A hit on a player's character that can cast Escudo waits for the target's
-- reaction before its damage can be rolled: the new status 'awaiting_reaction'
-- (MR-014, Etapa 6). The CHECK lists every status, dropped, if present, and
-- added again, so re-running this migration is safe.
ALTER TABLE pending_damages
    DROP CONSTRAINT IF EXISTS pending_damages_status_valid,
    ADD CONSTRAINT pending_damages_status_valid CHECK (status IN ('awaiting_reaction', 'awaiting_roll', 'rolled', 'applied', 'discarded'));

-- +goose Down
UPDATE pending_damages SET status = 'awaiting_roll' WHERE status = 'awaiting_reaction';

ALTER TABLE pending_damages
    DROP CONSTRAINT IF EXISTS pending_damages_status_valid,
    ADD CONSTRAINT pending_damages_status_valid CHECK (status IN ('awaiting_roll', 'rolled', 'applied', 'discarded'));
