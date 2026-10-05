-- +goose Up
-- A trap's damage waits for the master as an attack's does (MR-035, Etapa 9,
-- D5, RN-02), so it uses the same table: attacker_id becomes optional (a trap
-- attacks nobody) and trap_point_id says which trap made it. The attack_key of
-- such a row is 'trap'. The server rolls a trap's damage when the trap fires, so
-- a row is born 'rolled' (a player's character, waiting for the master) or
-- 'applied' (an NPC or a creature, which takes it at once).
--
-- trap_point_id has no foreign key, on purpose: map_points and encounters
-- already refer to each other (the battle point), and a reference from here to
-- map_points would close a cycle the test databases' table copy cannot build
-- (package dbtest). A trap deleted later leaves the row, which says what was
-- done; its name is simply not read back.
--
-- One statement with several parts, so re-running it is safe (see 00036). Every
-- row has an attacker or a trap.
ALTER TABLE pending_damages
    ALTER COLUMN attacker_id DROP NOT NULL,
    ADD COLUMN IF NOT EXISTS trap_point_id UUID NULL,
    DROP CONSTRAINT IF EXISTS pending_damages_source_valid,
    ADD CONSTRAINT pending_damages_source_valid CHECK (attacker_id IS NOT NULL OR trap_point_id IS NOT NULL);

-- +goose Down
ALTER TABLE pending_damages
    DROP CONSTRAINT IF EXISTS pending_damages_source_valid,
    DROP COLUMN IF EXISTS trap_point_id;
