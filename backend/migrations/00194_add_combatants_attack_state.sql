-- +goose Up
-- action_attack_key is the key of the sheet attack the combatant made last with
-- its action this turn: the bonus action attacks (Two-Weapon Fighting, Martial
-- Arts) go with the weapon of the Attack action, and the beams of a cantrip
-- (Eldritch Blast) belong to one cast. bonus_attacks_left is how many unarmed
-- strikes of Flurry of Blows are still to make. Both go back to their start when
-- the combatant's own turn starts (ResetCombatantTurn).
--
-- One statement with several parts, so re-running it is safe (see 00036).
ALTER TABLE combatants
    ADD COLUMN IF NOT EXISTS action_attack_key TEXT NULL,
    ADD COLUMN IF NOT EXISTS bonus_attacks_left INT4 NOT NULL DEFAULT 0,
    DROP CONSTRAINT IF EXISTS combatants_bonus_attacks_left_valid,
    ADD CONSTRAINT combatants_bonus_attacks_left_valid CHECK (bonus_attacks_left >= 0);

-- +goose Down
ALTER TABLE combatants
    DROP CONSTRAINT IF EXISTS combatants_bonus_attacks_left_valid,
    DROP COLUMN IF EXISTS bonus_attacks_left,
    DROP COLUMN IF EXISTS action_attack_key;
