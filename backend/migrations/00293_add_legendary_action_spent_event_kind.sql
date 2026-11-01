-- +goose Up
-- The master spent legendary actions on an option of a stat block that the engine does not
-- resolve (`legendary_action_spent`). The payload holds the creature's combatant id,
-- the round and the cost; the option's name is the stat block's. The master reads the event
-- alone: its row is secret.
INSERT INTO session_event_kinds (kind) VALUES
    ('legendary_action_spent')
ON CONFLICT (kind) DO NOTHING;

-- +goose Down
DELETE FROM session_events WHERE kind = 'legendary_action_spent';
DELETE FROM session_event_kinds WHERE kind = 'legendary_action_spent';
