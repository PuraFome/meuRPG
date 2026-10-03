-- +goose Up
-- An encounter's events in order (the combat log), and the ON DELETE CASCADE
-- when it is deleted. Only the events that belong to one are indexed.
CREATE INDEX IF NOT EXISTS session_events_encounter_id_seq_idx
    ON session_events (encounter_id, seq)
    WHERE encounter_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS session_events_encounter_id_seq_idx;
