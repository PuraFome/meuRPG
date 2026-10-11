-- +goose Up
-- A table maneuver that reduces a melee hit's damage waits in a reaction window of its own
-- kind: the list of 00240 plus 'maneuver_reduce'. Dropped first by name, so the migration
-- can run twice.
ALTER TABLE reaction_windows
    DROP CONSTRAINT IF EXISTS reaction_windows_kind_valid,
    ADD CONSTRAINT reaction_windows_kind_valid CHECK (kind IN (
        'shield', 'uncanny_dodge', 'hellish_rebuke', 'counterspell', 'cutting_words',
        'deflect_missiles', 'feather_fall', 'concentration_save', 'master_check', 'effect_save', 'contest',
        'maneuver_reduce'
    ));

-- +goose Down
DELETE FROM reaction_windows WHERE kind = 'maneuver_reduce';
ALTER TABLE reaction_windows
    DROP CONSTRAINT IF EXISTS reaction_windows_kind_valid,
    ADD CONSTRAINT reaction_windows_kind_valid CHECK (kind IN (
        'shield', 'uncanny_dodge', 'hellish_rebuke', 'counterspell', 'cutting_words',
        'deflect_missiles', 'feather_fall', 'concentration_save', 'master_check', 'effect_save', 'contest'
    ));
