-- +goose Up
-- mage_armor_ac is the armor class Mage Armor gives a combatant (13 + its
-- Dexterity modifier, SRD 5.1) when the combat starts or the character joins it;
-- NULL when the spell is not on it. The combat uses the greater of the sheet's
-- armor class and this one. The spell is cast outside the combat (spell_casts) and
-- lasts hours, so the combatant copies it when it joins, as it copies the speed, and
-- the master's ending of the spell clears it.
ALTER TABLE combatants
    ADD COLUMN IF NOT EXISTS mage_armor_ac INT4 NULL,
    DROP CONSTRAINT IF EXISTS combatants_mage_armor_ac_valid,
    ADD CONSTRAINT combatants_mage_armor_ac_valid CHECK (mage_armor_ac IS NULL OR mage_armor_ac BETWEEN 1 AND 60);

-- +goose Down
ALTER TABLE combatants
    DROP CONSTRAINT IF EXISTS combatants_mage_armor_ac_valid,
    DROP COLUMN IF EXISTS mage_armor_ac;
