-- +goose Up
-- A cast whose saving throw a creature with Legendary Resistance failed waits in a master-only
-- reaction window of its own kind: the list of 00265 plus 'legendary_resistance'. Dropped first by
-- name, so the migration can run twice.
ALTER TABLE reaction_windows
    DROP CONSTRAINT IF EXISTS reaction_windows_kind_valid,
    ADD CONSTRAINT reaction_windows_kind_valid CHECK (kind IN (
        'shield', 'uncanny_dodge', 'hellish_rebuke', 'counterspell', 'cutting_words',
        'deflect_missiles', 'feather_fall', 'concentration_save', 'master_check', 'effect_save', 'contest',
        'maneuver_reduce', 'protection', 'legendary_resistance'
    ));

-- +goose Down
DELETE FROM reaction_windows WHERE kind = 'legendary_resistance';
ALTER TABLE reaction_windows
    DROP CONSTRAINT IF EXISTS reaction_windows_kind_valid,
    ADD CONSTRAINT reaction_windows_kind_valid CHECK (kind IN (
        'shield', 'uncanny_dodge', 'hellish_rebuke', 'counterspell', 'cutting_words',
        'deflect_missiles', 'feather_fall', 'concentration_save', 'master_check', 'effect_save', 'contest',
        'maneuver_reduce', 'protection'
    ));
