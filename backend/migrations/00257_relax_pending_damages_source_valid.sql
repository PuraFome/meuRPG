-- +goose Up
-- The damage a lasting effect deals when a turn starts under it (the web that burns) has neither an
-- attacker nor a trap: its pending_damages row carries effect_source_key (00236) instead. The check
-- on the source (00110) only knew the attacker and the trap, so the insert failed and the turn of
-- the burning creature could not pass. The check now accepts a row with an effect source too.
--
-- Production is live and the table is small: the new check validates at once. The old check is
-- dropped by every name it can have (the one 00110 gave it, and the auto-generated name of
-- CockroachDB for a check on the same columns) before the new one is added, so running the
-- migration twice is safe.
ALTER TABLE pending_damages
    DROP CONSTRAINT IF EXISTS pending_damages_source_valid,
    DROP CONSTRAINT IF EXISTS check_attacker_id_trap_point_id,
    ADD CONSTRAINT pending_damages_source_valid CHECK (attacker_id IS NOT NULL OR trap_point_id IS NOT NULL OR effect_source_key IS NOT NULL);

-- +goose Down
-- A row born from an effect has no source the old check accepts, so it goes before the check returns.
DELETE FROM pending_damages WHERE attacker_id IS NULL AND trap_point_id IS NULL;
ALTER TABLE pending_damages
    DROP CONSTRAINT IF EXISTS pending_damages_source_valid,
    ADD CONSTRAINT pending_damages_source_valid CHECK (attacker_id IS NOT NULL OR trap_point_id IS NOT NULL);
