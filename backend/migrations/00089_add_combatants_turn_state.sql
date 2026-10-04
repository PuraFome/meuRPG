-- +goose Up
-- turn_state is where a combatant stands in the turn the combat is on (joint
-- turns, MR-013, RN-19). A group of combatants adjacent in the turn order with
-- the same initiative total takes one turn together; when that turn starts,
-- every living member of the group gets 'acting' (the group is worked out from
-- the order and the totals at that moment, and this column keeps the answer, so
-- a reorder, an added combatant or a hidden member cannot change a turn that
-- already began). A member whose part ended is 'ended'; everyone else is
-- 'idle'. The turn passes when no member is 'acting'.
--
-- One statement with several parts, so re-running it is safe (see 00036).
ALTER TABLE combatants
    ADD COLUMN IF NOT EXISTS turn_state TEXT NOT NULL DEFAULT 'idle',
    DROP CONSTRAINT IF EXISTS combatants_turn_state_valid,
    ADD CONSTRAINT combatants_turn_state_valid CHECK (turn_state IN ('idle', 'acting', 'ended'));

-- +goose Down
ALTER TABLE combatants
    DROP CONSTRAINT IF EXISTS combatants_turn_state_valid,
    DROP COLUMN IF EXISTS turn_state;
