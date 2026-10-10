-- +goose Up
-- The wizard's Arcane Recovery (`arcane_recovery_used`): the slots a player
-- recovered after a short rest. Levels and counts only.
INSERT INTO session_event_kinds (kind) VALUES
    ('arcane_recovery_used')
ON CONFLICT (kind) DO NOTHING;

-- +goose Down
DELETE FROM session_events WHERE kind = 'arcane_recovery_used';
DELETE FROM session_event_kinds WHERE kind = 'arcane_recovery_used';
