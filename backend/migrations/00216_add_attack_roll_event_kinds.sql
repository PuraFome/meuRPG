-- +goose Up
-- The session event kinds of the attack rolls: a player's request for a better mode
-- and the master's answer, an extra the master took out of a damage, and a state of a
-- combatant that began or ended. A new kind is one INSERT (00168).
INSERT INTO session_event_kinds (kind) VALUES
    ('roll_mode_requested'),
    ('roll_mode_answered'),
    ('damage_part_removed'),
    ('state_changed')
ON CONFLICT (kind) DO NOTHING;

-- +goose Down
DELETE FROM session_events WHERE kind IN ('roll_mode_requested', 'roll_mode_answered', 'damage_part_removed', 'state_changed');
DELETE FROM session_event_kinds WHERE kind IN ('roll_mode_requested', 'roll_mode_answered', 'damage_part_removed', 'state_changed');
