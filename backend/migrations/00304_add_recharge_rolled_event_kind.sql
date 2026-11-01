-- +goose Up
-- The server rolled a d6 for a Recharge action of a creature at the start of its turn
-- (`recharge_rolled`). The payload holds the creature's combatant id, the round, the action,
-- the face and whether the action recharged. The master reads the event alone: its row is secret.
INSERT INTO session_event_kinds (kind) VALUES
    ('recharge_rolled')
ON CONFLICT (kind) DO NOTHING;

-- +goose Down
DELETE FROM session_events WHERE kind = 'recharge_rolled';
DELETE FROM session_event_kinds WHERE kind = 'recharge_rolled';
