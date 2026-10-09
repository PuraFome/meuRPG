-- +goose Up
-- The session event kinds of the contests and the special actions: the contest that
-- began, was answered (or left to the master), was closed by the master, the shove's choice,
-- a grapple let go, a hide attempt, what the master decided and the hiding that ended, a Help and its
-- clearing, a group check (asked, answered, closed) and a creature marked surprised.
-- A new kind is one INSERT (00168).
INSERT INTO session_event_kinds (kind) VALUES
    ('contest_started'),
    ('contest_resolved'),
    ('contest_deferred'),
    ('contest_closed'),
    ('shove_resolved'),
    ('grapple_released'),
    ('hide_attempted'),
    ('hide_resolved'),
    ('hide_ended'),
    ('help_given'),
    ('help_cleared'),
    ('group_check_requested'),
    ('group_check_rolled'),
    ('group_check_closed'),
    ('surprise_set')
ON CONFLICT (kind) DO NOTHING;

-- +goose Down
DELETE FROM session_events WHERE kind IN (
    'contest_started', 'contest_resolved', 'contest_deferred', 'contest_closed', 'shove_resolved', 'grapple_released',
    'hide_attempted', 'hide_resolved', 'hide_ended', 'help_given', 'help_cleared',
    'group_check_requested', 'group_check_rolled', 'group_check_closed', 'surprise_set'
);
DELETE FROM session_event_kinds WHERE kind IN (
    'contest_started', 'contest_resolved', 'contest_deferred', 'contest_closed', 'shove_resolved', 'grapple_released',
    'hide_attempted', 'hide_resolved', 'hide_ended', 'help_given', 'help_cleared',
    'group_check_requested', 'group_check_rolled', 'group_check_closed', 'surprise_set'
);
