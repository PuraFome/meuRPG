-- +goose Up
-- An effect that lasts can be used up before its clock ends it (RN-22): Zombaria Viciosa's
-- disadvantage goes with the next attack roll its target makes (SRD 5.1, Vicious Mockery), but the
-- row stays until the end of the target's next turn, so the master's undo of that attack can give
-- it back. spent_at is when a roll used it up; NULL while it is not. A spent effect changes no
-- roll. Production is live: the column only adds, and running the migration twice is safe.
ALTER TABLE combatant_states ADD COLUMN IF NOT EXISTS spent_at TIMESTAMPTZ NULL;

-- +goose Down
ALTER TABLE combatant_states DROP COLUMN IF EXISTS spent_at;
