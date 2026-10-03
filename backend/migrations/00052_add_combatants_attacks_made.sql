-- +goose Up
-- attacks_made counts the attacks the Attack action made this turn (Extra
-- Attack, MR-012, Etapa 6): the first attack spends the action, and the next
-- ones of the same action are allowed while the sheet's attacks per action are
-- not all made. It goes back to 0 when the combatant's own turn starts
-- (ResetCombatantTurn), with the rest of the turn's economy.
--
-- One statement with several parts, so re-running it is safe (see 00036).
ALTER TABLE combatants
    ADD COLUMN IF NOT EXISTS attacks_made INT4 NOT NULL DEFAULT 0,
    DROP CONSTRAINT IF EXISTS combatants_attacks_made_valid,
    ADD CONSTRAINT combatants_attacks_made_valid CHECK (attacks_made >= 0);

-- +goose Down
ALTER TABLE combatants
    DROP CONSTRAINT IF EXISTS combatants_attacks_made_valid,
    DROP COLUMN IF EXISTS attacks_made;
