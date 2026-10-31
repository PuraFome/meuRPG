-- +goose Up
-- The Circle of the Land druid's Natural Recovery (`natural_recovery_used`): the
-- slots a player recovered after a short rest. Levels and counts only.
INSERT INTO session_event_kinds (kind) VALUES
    ('natural_recovery_used')
ON CONFLICT (kind) DO NOTHING;

-- +goose Down
DELETE FROM session_events WHERE kind = 'natural_recovery_used';
DELETE FROM session_event_kinds WHERE kind = 'natural_recovery_used';
