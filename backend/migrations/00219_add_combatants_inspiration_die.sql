-- +goose Up
-- The Bardic Inspiration die a combatant holds (SRD 5.1, Bard): its size (6, 8, 10
-- or 12), the combatant of the bard that gave it, and the round it runs out in (it
-- lasts 10 minutes, 100 rounds of 6 seconds). NULL sides is "no die": a creature has
-- at most one die at a time. The die ends with the combat, as the combatant does.
--
-- One statement with several parts, so re-running it is safe (see 00036).
ALTER TABLE combatants
    ADD COLUMN IF NOT EXISTS inspiration_sides INT4 NULL,
    ADD COLUMN IF NOT EXISTS inspiration_from UUID NULL,
    ADD COLUMN IF NOT EXISTS inspiration_expires_round INT4 NULL,
    DROP CONSTRAINT IF EXISTS combatants_inspiration_die_valid,
    ADD CONSTRAINT combatants_inspiration_die_valid CHECK (
        (inspiration_sides IS NULL AND inspiration_from IS NULL AND inspiration_expires_round IS NULL)
        OR (inspiration_sides IN (6, 8, 10, 12) AND inspiration_expires_round IS NOT NULL AND inspiration_expires_round >= 1)
    );

-- +goose Down
ALTER TABLE combatants
    DROP CONSTRAINT IF EXISTS combatants_inspiration_die_valid,
    DROP COLUMN IF EXISTS inspiration_expires_round,
    DROP COLUMN IF EXISTS inspiration_from,
    DROP COLUMN IF EXISTS inspiration_sides;
