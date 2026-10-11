-- +goose Up
-- Two jump numbers a feature changes (MR-034, SRD 5.1). jump_long_bonus_dft is the
-- extra distance of a long jump with a running start, in tenths of a foot, that
-- the Thief's Second-Story Work gives (the Dexterity modifier, never below 0); it
-- is copied from the sheet when the combatant joins, like jump_long_dft, and a
-- standing jump does not have it. jump_doubled says the monk used Step of the Wind
-- this turn, which doubles its jump distance for the rest of the turn; the start
-- of the combatant's next turn clears it, like dashed.
--
-- One statement with several parts, so re-running it is safe (see 00036).
ALTER TABLE combatants
    ADD COLUMN IF NOT EXISTS jump_long_bonus_dft INT4 NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS jump_doubled BOOL NOT NULL DEFAULT false,
    DROP CONSTRAINT IF EXISTS combatants_jump_long_bonus_valid,
    ADD CONSTRAINT combatants_jump_long_bonus_valid CHECK (jump_long_bonus_dft BETWEEN 0 AND 6000);

-- +goose Down
ALTER TABLE combatants
    DROP CONSTRAINT IF EXISTS combatants_jump_long_bonus_valid,
    DROP COLUMN IF EXISTS jump_doubled,
    DROP COLUMN IF EXISTS jump_long_bonus_dft;
