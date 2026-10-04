-- +goose Up
-- Whether a milestone award is "Dar a mais alguém" (an extra mark on a milestone
-- that was already reached) and not the one that reached it (MR-016). A retry
-- of the same idempotency key must be the same kind of request, and the kind
-- cannot be told from the rows: a milestone marked again after an undo looks
-- like a first mark. False for every other award.
ALTER TABLE xp_awards ADD COLUMN IF NOT EXISTS milestone_again BOOL NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE xp_awards DROP COLUMN IF EXISTS milestone_again;
