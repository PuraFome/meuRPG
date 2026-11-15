-- +goose Up
-- The master's Amanhecer is a session event of its own, so the key of a retry finds the dawn that was taken even when no
-- item changed (the figurines count a dawn with no charge to show).
INSERT INTO session_event_kinds (kind) VALUES ('dawn_taken')
ON CONFLICT (kind) DO NOTHING;

-- +goose Down
DELETE FROM session_events WHERE kind = 'dawn_taken';
DELETE FROM session_event_kinds WHERE kind = 'dawn_taken';
