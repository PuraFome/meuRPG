-- +goose Up
-- The columns combat movement needs (MR-034, RN-21, Etapa 9, slice 9.6).
--
-- A move costs the exact length of its straight line, so movement is kept in
-- tenths of a foot ("dft"): movement_used_dft is what was walked this turn (a
-- square straight is 50, a diagonal one 71) and last_move_dft the length of the
-- last move on foot this turn, which a jump reads as its running start (10 ft or
-- more). movement_used_ft stays, filled with the same number rounded down, for
-- the web that is already shipped.
--
-- side is whose side the combatant fights on: 'party' (the players' characters,
-- and an NPC the master marks "Aliado") or 'enemy' (the default; the service
-- gives a player's character 'party' when it joins, and 00111 fixes the rows
-- that already existed). size, speed_fly_ft and the two jump limits are copied
-- from the sheet when the combatant joins, like speed_ft: size in the SRD's
-- order (Medium is the default), a fly speed in feet (0 for none), and the long
-- and high jump with a running start in tenths of a foot (half of each without
-- one). cover_mark is the cover the master marked on the combatant by hand; the
-- service clears it when the combatant moves. disengaged is the Disengage action
-- taken this turn, like dashed.
--
-- One statement with several parts, so re-running it is safe (see 00036).
ALTER TABLE combatants
    ADD COLUMN IF NOT EXISTS movement_used_dft INT4 NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS last_move_dft INT4 NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS side TEXT NOT NULL DEFAULT 'enemy',
    ADD COLUMN IF NOT EXISTS size TEXT NOT NULL DEFAULT 'medium',
    ADD COLUMN IF NOT EXISTS speed_fly_ft INT4 NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS jump_long_dft INT4 NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS jump_high_dft INT4 NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS cover_mark TEXT NOT NULL DEFAULT 'none',
    ADD COLUMN IF NOT EXISTS disengaged BOOL NOT NULL DEFAULT false,
    DROP CONSTRAINT IF EXISTS combatants_movement_dft_valid,
    ADD CONSTRAINT combatants_movement_dft_valid CHECK (movement_used_dft >= 0 AND last_move_dft >= 0),
    DROP CONSTRAINT IF EXISTS combatants_side_valid,
    ADD CONSTRAINT combatants_side_valid CHECK (side IN ('party', 'enemy')),
    DROP CONSTRAINT IF EXISTS combatants_size_valid,
    ADD CONSTRAINT combatants_size_valid CHECK (size IN ('tiny', 'small', 'medium', 'large', 'huge', 'gargantuan')),
    DROP CONSTRAINT IF EXISTS combatants_fly_and_jumps_valid,
    ADD CONSTRAINT combatants_fly_and_jumps_valid CHECK (speed_fly_ft BETWEEN 0 AND 600 AND jump_long_dft BETWEEN 0 AND 6000 AND jump_high_dft BETWEEN 0 AND 6000),
    DROP CONSTRAINT IF EXISTS combatants_cover_mark_valid,
    ADD CONSTRAINT combatants_cover_mark_valid CHECK (cover_mark IN ('none', 'half', 'three_quarters', 'total'));

-- +goose Down
ALTER TABLE combatants
    DROP CONSTRAINT IF EXISTS combatants_cover_mark_valid,
    DROP CONSTRAINT IF EXISTS combatants_fly_and_jumps_valid,
    DROP CONSTRAINT IF EXISTS combatants_size_valid,
    DROP CONSTRAINT IF EXISTS combatants_side_valid,
    DROP CONSTRAINT IF EXISTS combatants_movement_dft_valid,
    DROP COLUMN IF EXISTS disengaged,
    DROP COLUMN IF EXISTS cover_mark,
    DROP COLUMN IF EXISTS jump_high_dft,
    DROP COLUMN IF EXISTS jump_long_dft,
    DROP COLUMN IF EXISTS speed_fly_ft,
    DROP COLUMN IF EXISTS size,
    DROP COLUMN IF EXISTS side,
    DROP COLUMN IF EXISTS last_move_dft,
    DROP COLUMN IF EXISTS movement_used_dft;
