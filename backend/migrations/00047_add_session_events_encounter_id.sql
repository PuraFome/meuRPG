-- +goose Up
-- The combat log (MR-012, MR-013) reads the session's events of one
-- encounter, and the undo walks back from the last one, so an event says
-- which combat it belongs to. encounter_id is set on every event a combat
-- writes from now on (the attacks, the damage, the actions, the turns...); it
-- stays NULL for the vitals corrections, which are about a character, and for
-- the events written before this column existed. Deleting the encounter
-- deletes its events, as deleting the session already does.
--
-- The foreign key is a named constraint of its own, dropped and added again:
-- ADD COLUMN IF NOT EXISTS skips the column when it is there but would add a
-- second, copy of an inline REFERENCES on every re-run.
ALTER TABLE session_events
    ADD COLUMN IF NOT EXISTS encounter_id UUID NULL;

ALTER TABLE session_events
    DROP CONSTRAINT IF EXISTS session_events_encounter_id_fkey,
    ADD CONSTRAINT session_events_encounter_id_fkey FOREIGN KEY (encounter_id) REFERENCES encounters (id) ON DELETE CASCADE;

-- +goose Down
ALTER TABLE session_events DROP COLUMN IF EXISTS encounter_id;
