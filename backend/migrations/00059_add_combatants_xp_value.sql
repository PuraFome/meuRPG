-- +goose Up
-- xp_value is the XP an NPC combatant gives when it is defeated (MR-016, RN-09),
-- copied from the NPC's sheet when it joins the fight, like speed_ft: editing
-- the sheet later never moves a fight already going. The end-of-combat summary
-- adds it up for the master's "Dar XP"; a player never receives it (RN-20).
-- It stays 0 for a player's character and for an NPC whose sheet gives none.
--
-- One statement with several parts, so re-running it is safe (see 00036).
ALTER TABLE combatants
    ADD COLUMN IF NOT EXISTS xp_value INT4 NOT NULL DEFAULT 0,
    DROP CONSTRAINT IF EXISTS combatants_xp_value_valid,
    ADD CONSTRAINT combatants_xp_value_valid CHECK (xp_value BETWEEN 0 AND 1000000);

-- +goose Down
ALTER TABLE combatants
    DROP CONSTRAINT IF EXISTS combatants_xp_value_valid,
    DROP COLUMN IF EXISTS xp_value;
