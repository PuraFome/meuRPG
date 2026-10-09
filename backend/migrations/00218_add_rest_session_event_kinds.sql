-- +goose Up
-- The session events of the rests: the master's short or long rest for the party
-- (`rest_taken`, which holds the kind and the characters it touched: IDs and
-- numbers only) and a player's hit die spent on a short rest (`hit_dice_spent`).
INSERT INTO session_event_kinds (kind) VALUES
    ('rest_taken'),
    ('hit_dice_spent')
ON CONFLICT (kind) DO NOTHING;

-- +goose Down
DELETE FROM session_events WHERE kind IN ('rest_taken', 'hit_dice_spent');
DELETE FROM session_event_kinds WHERE kind IN ('rest_taken', 'hit_dice_spent');
