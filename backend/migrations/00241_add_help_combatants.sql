-- +goose Up
-- A Help in a combat names the helper's and the ally's combatants, not only their characters:
-- a player's creatures (a familiar, the wolves of Conjure Animals) carry their owner's
-- character_id, so the characters alone cannot tell Pensantus from his familiar. A familiar
-- can't attack, but it can take other actions as normal (SRD 5.1, Find Familiar), Help among
-- them. NULL outside a combat, and on the helps kept before this migration (read by character).
ALTER TABLE combat_helps ADD COLUMN IF NOT EXISTS helper_combatant_id UUID NULL REFERENCES combatants (id) ON DELETE CASCADE;
ALTER TABLE combat_helps ADD COLUMN IF NOT EXISTS ally_combatant_id UUID NULL REFERENCES combatants (id) ON DELETE CASCADE;

-- +goose Down
ALTER TABLE combat_helps DROP COLUMN IF EXISTS ally_combatant_id;
ALTER TABLE combat_helps DROP COLUMN IF EXISTS helper_combatant_id;
