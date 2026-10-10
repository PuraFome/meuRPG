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

-- A contest waits in a reaction window of its own kind (W7-X, on 00221).
ALTER TABLE reaction_windows DROP CONSTRAINT reaction_windows_kind_valid;
ALTER TABLE reaction_windows ADD CONSTRAINT reaction_windows_kind_valid CHECK (kind IN (
    'shield', 'uncanny_dodge', 'hellish_rebuke', 'counterspell', 'cutting_words',
    'deflect_missiles', 'feather_fall', 'concentration_save', 'master_check', 'contest'
));

-- +goose Down
DELETE FROM reaction_windows WHERE kind = 'contest';
ALTER TABLE reaction_windows DROP CONSTRAINT reaction_windows_kind_valid;
ALTER TABLE reaction_windows ADD CONSTRAINT reaction_windows_kind_valid CHECK (kind IN (
    'shield', 'uncanny_dodge', 'hellish_rebuke', 'counterspell', 'cutting_words',
    'deflect_missiles', 'feather_fall', 'concentration_save', 'master_check'
));
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
