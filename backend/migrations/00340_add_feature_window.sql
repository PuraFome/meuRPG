-- +goose Up
-- A reaction feature of the table whose use declares a trigger (use.trigger) opens a reaction window of its own kind when an
-- enemy starts its turn near the owner or comes near: the list of 00334 plus 'feature'.
-- Dropped first by name, so the migration can run twice; it only widens the CHECK, so every row stays valid.
-- Whoever adds a kind after this one lists every kind the CHECK has then (this list is the one on main when 00340 was
-- written), so that no migration drops another's.
ALTER TABLE reaction_windows
    DROP CONSTRAINT IF EXISTS reaction_windows_kind_valid,
    ADD CONSTRAINT reaction_windows_kind_valid CHECK (kind IN (
        'shield', 'uncanny_dodge', 'hellish_rebuke', 'counterspell', 'cutting_words',
        'deflect_missiles', 'feather_fall', 'concentration_save', 'master_check', 'effect_save', 'contest',
        'maneuver_reduce', 'protection', 'legendary_resistance', 'parry', 'slow_fall', 'readied', 'feature'
    ));

-- +goose Down
DELETE FROM reaction_windows WHERE kind = 'feature';
ALTER TABLE reaction_windows
    DROP CONSTRAINT IF EXISTS reaction_windows_kind_valid,
    ADD CONSTRAINT reaction_windows_kind_valid CHECK (kind IN (
        'shield', 'uncanny_dodge', 'hellish_rebuke', 'counterspell', 'cutting_words',
        'deflect_missiles', 'feather_fall', 'concentration_save', 'master_check', 'effect_save', 'contest',
        'maneuver_reduce', 'protection', 'legendary_resistance', 'parry', 'slow_fall', 'readied'
    ));
