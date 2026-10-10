-- +goose Up
-- A Help in a combat names the helper's and the ally's combatants, not only their characters:
-- a player's creatures (a familiar, the wolves of Conjure Animals) carry their owner's
-- character_id, so the characters alone cannot tell Pensantus from his familiar. A familiar
-- can't attack, but it can take other actions as normal (SRD 5.1, Find Familiar), Help among
-- them. NULL outside a combat, and on the helps kept before this migration (read by character).
-- The foreign keys are named and dropped first, so the migration can run twice.
ALTER TABLE combat_helps
    ADD COLUMN IF NOT EXISTS helper_combatant_id UUID NULL,
    DROP CONSTRAINT IF EXISTS combat_helps_helper_combatant_id_fkey,
    ADD CONSTRAINT combat_helps_helper_combatant_id_fkey FOREIGN KEY (helper_combatant_id) REFERENCES combatants (id) ON DELETE CASCADE,
    ADD COLUMN IF NOT EXISTS ally_combatant_id UUID NULL,
    DROP CONSTRAINT IF EXISTS combat_helps_ally_combatant_id_fkey,
    ADD CONSTRAINT combat_helps_ally_combatant_id_fkey FOREIGN KEY (ally_combatant_id) REFERENCES combatants (id) ON DELETE CASCADE;

CREATE INDEX IF NOT EXISTS combat_helps_helper_combatant_id_idx ON combat_helps (helper_combatant_id);
CREATE INDEX IF NOT EXISTS combat_helps_ally_combatant_id_idx ON combat_helps (ally_combatant_id);

-- +goose Down
DROP INDEX IF EXISTS combat_helps_ally_combatant_id_idx;
DROP INDEX IF EXISTS combat_helps_helper_combatant_id_idx;
ALTER TABLE combat_helps DROP COLUMN IF EXISTS ally_combatant_id;
ALTER TABLE combat_helps DROP COLUMN IF EXISTS helper_combatant_id;
