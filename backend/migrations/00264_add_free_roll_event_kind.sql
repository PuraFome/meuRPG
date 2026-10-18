-- +goose Up
-- The free dice roller (`free_roll_rolled`): dice the app rolled on request, outside every game
-- rule. The payload holds the die, the faces, the modifier, the total and whether the master kept
-- the roll to himself ("secret"); a secret roll is read by the master alone.
INSERT INTO session_event_kinds (kind) VALUES
    ('free_roll_rolled')
ON CONFLICT (kind) DO NOTHING;

-- +goose Down
DELETE FROM session_events WHERE kind = 'free_roll_rolled';
DELETE FROM session_event_kinds WHERE kind = 'free_roll_rolled';
