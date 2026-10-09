-- +goose Up
-- The saving throw an effect asks at a turn (Imobilizar Pessoa at the end of the
-- target's turn, Teia at its start) is a reaction window of its own kind, like the
-- concentration save (PM-04): 'effect_save'. A window of it closes by itself with
-- 'effect_ended' (the effect ended meanwhile) or 'caster_lost_concentration'.
-- The checks are their own named constraints, dropped first, so the migration can
-- run twice.
ALTER TABLE reaction_windows
    DROP CONSTRAINT IF EXISTS reaction_windows_kind_valid,
    ADD CONSTRAINT reaction_windows_kind_valid CHECK (kind IN (
        'shield', 'uncanny_dodge', 'hellish_rebuke', 'counterspell', 'cutting_words',
        'deflect_missiles', 'feather_fall', 'concentration_save', 'master_check', 'effect_save'
    )),
    DROP CONSTRAINT IF EXISTS reaction_windows_closed_reason_valid,
    ADD CONSTRAINT reaction_windows_closed_reason_valid CHECK (
        closed_reason IS NULL OR closed_reason IN ('reaction_spent', 'reactor_incapacitated', 'trigger_gone', 'effect_ended', 'caster_lost_concentration')
    );

-- The kinds of session event the effects write. A new kind is one INSERT (00168).
INSERT INTO session_event_kinds (kind) VALUES
    ('lasting_effect_added'),
    ('lasting_effect_changed'),
    ('lasting_effect_ended'),
    ('lasting_effect_visibility_changed'),
    ('lasting_effect_saved'),
    ('lasting_effect_triggered'),
    ('exhaustion_changed'),
    ('game_time_advanced')
ON CONFLICT (kind) DO NOTHING;

-- +goose Down
DELETE FROM session_events WHERE kind IN ('lasting_effect_added', 'lasting_effect_changed', 'lasting_effect_ended', 'lasting_effect_visibility_changed', 'lasting_effect_saved', 'lasting_effect_triggered', 'exhaustion_changed', 'game_time_advanced');
DELETE FROM session_event_kinds WHERE kind IN ('lasting_effect_added', 'lasting_effect_changed', 'lasting_effect_ended', 'lasting_effect_visibility_changed', 'lasting_effect_saved', 'lasting_effect_triggered', 'exhaustion_changed', 'game_time_advanced');
DELETE FROM reaction_windows WHERE kind = 'effect_save';
ALTER TABLE reaction_windows
    DROP CONSTRAINT IF EXISTS reaction_windows_closed_reason_valid,
    ADD CONSTRAINT reaction_windows_closed_reason_valid CHECK (
        closed_reason IS NULL OR closed_reason IN ('reaction_spent', 'reactor_incapacitated', 'trigger_gone')
    ),
    DROP CONSTRAINT IF EXISTS reaction_windows_kind_valid,
    ADD CONSTRAINT reaction_windows_kind_valid CHECK (kind IN (
        'shield', 'uncanny_dodge', 'hellish_rebuke', 'counterspell', 'cutting_words',
        'deflect_missiles', 'feather_fall', 'concentration_save', 'master_check'
    ));
