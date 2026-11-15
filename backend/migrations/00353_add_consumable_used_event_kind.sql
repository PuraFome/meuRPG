-- +goose Up
-- Using up a consumable (a potion, a scroll) is a session event of its own, so the key of a retry finds the use that was made.
INSERT INTO session_event_kinds (kind) VALUES ('consumable_used')
ON CONFLICT (kind) DO NOTHING;

-- +goose Down
DELETE FROM session_events WHERE kind = 'consumable_used';
DELETE FROM session_event_kinds WHERE kind = 'consumable_used';
