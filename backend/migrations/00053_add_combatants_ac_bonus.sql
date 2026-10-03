-- +goose Up
-- ac_bonus is the armor class a combatant has on top of its sheet's, from a
-- spell that lasts until the start of its next turn: Escudo gives +5 (RN-02,
-- Etapa 6). Every attack against the combatant adds it. It goes back to 0 when
-- the combatant's own turn starts (ResetCombatantTurn). The sheet is never
-- changed: combat never writes to a character's sheet (RN-04).
--
-- One statement with several parts, so re-running it is safe (see 00036).
ALTER TABLE combatants
    ADD COLUMN IF NOT EXISTS ac_bonus INT4 NOT NULL DEFAULT 0,
    DROP CONSTRAINT IF EXISTS combatants_ac_bonus_valid,
    ADD CONSTRAINT combatants_ac_bonus_valid CHECK (ac_bonus BETWEEN 0 AND 30);

-- +goose Down
ALTER TABLE combatants
    DROP CONSTRAINT IF EXISTS combatants_ac_bonus_valid,
    DROP COLUMN IF EXISTS ac_bonus;
