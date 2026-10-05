-- +goose Up
-- "Ver pelos olhos do familiar" (MR-036, Etapa 9, D6): while it is on, the
-- player sees what the character's familiar sees. familiar_sight_creature_id
-- is the familiar (a character_creatures row), NULL while it is off, and goes
-- back to NULL by itself if the creature is deleted. familiar_sight_in_combat
-- says it was started in a combat, where it ends at the start of the
-- character's next turn; outside a combat it lasts until the player stops.
-- familiar_sight_conditions are the conditions ('condition:blinded',
-- 'condition:deafened') the start gave the character's combatant, which the end
-- takes away again: one the combatant already had is never listed, so it is
-- never taken away.
--
-- One statement, as 00104 (the foreign key is dropped and added again, never
-- written inline). IDs and keys only.
ALTER TABLE character_vitals
    ADD COLUMN IF NOT EXISTS familiar_sight_creature_id UUID NULL,
    DROP CONSTRAINT IF EXISTS character_vitals_familiar_sight_creature_id_fkey,
    ADD CONSTRAINT character_vitals_familiar_sight_creature_id_fkey FOREIGN KEY (familiar_sight_creature_id) REFERENCES character_creatures (id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS familiar_sight_in_combat BOOL NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS familiar_sight_conditions TEXT[] NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE character_vitals
    DROP COLUMN IF EXISTS familiar_sight_conditions,
    DROP COLUMN IF EXISTS familiar_sight_in_combat,
    DROP CONSTRAINT IF EXISTS character_vitals_familiar_sight_creature_id_fkey,
    DROP COLUMN IF EXISTS familiar_sight_creature_id;
