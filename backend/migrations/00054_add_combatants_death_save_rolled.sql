-- +goose Up
-- death_save_rolled says the combatant already rolled its death save this turn
-- (RN-03, Etapa 6): a player's character at 0 hit points rolls one when its
-- turn starts, and the turn waits for it. It goes back to false when the
-- combatant's own turn starts (ResetCombatantTurn). The counts themselves
-- (death_successes, death_failures) are in 00044.
--
-- One statement, so re-running it is safe (see 00036).
ALTER TABLE combatants
    ADD COLUMN IF NOT EXISTS death_save_rolled BOOL NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE combatants
    DROP COLUMN IF EXISTS death_save_rolled;
