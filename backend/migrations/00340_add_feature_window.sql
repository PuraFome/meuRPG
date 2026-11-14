-- +goose Up
-- A reaction feature of the table whose use declares a trigger (use.trigger) opens a reaction window of its own kind when an
-- enemy starts its turn near the owner or comes near: the list of 00339 (00334's plus 'mount_save' and 'dismount') plus 'feature'.
-- Dropped first by name, so the migration can run twice; it only widens the CHECK, so every row stays valid.
-- Whoever adds a kind after this one lists every kind the CHECK has then, so that no migration drops another's.
-- The Down puts back the list of 00339, so a stored 'mount_save' or 'dismount' window survives it.
ALTER TABLE reaction_windows
    DROP CONSTRAINT IF EXISTS reaction_windows_kind_valid,
    ADD CONSTRAINT reaction_windows_kind_valid CHECK (kind IN (
        'shield', 'uncanny_dodge', 'hellish_rebuke', 'counterspell', 'cutting_words',
        'deflect_missiles', 'feather_fall', 'concentration_save', 'master_check', 'effect_save', 'contest',
        'maneuver_reduce', 'protection', 'legendary_resistance', 'parry', 'slow_fall', 'readied', 'mount_save', 'dismount', 'feature'
    ));

-- +goose Down
DELETE FROM reaction_windows WHERE kind = 'feature';
ALTER TABLE reaction_windows
    DROP CONSTRAINT IF EXISTS reaction_windows_kind_valid,
    ADD CONSTRAINT reaction_windows_kind_valid CHECK (kind IN (
        'shield', 'uncanny_dodge', 'hellish_rebuke', 'counterspell', 'cutting_words',
        'deflect_missiles', 'feather_fall', 'concentration_save', 'master_check', 'effect_save', 'contest',
        'maneuver_reduce', 'protection', 'legendary_resistance', 'parry', 'slow_fall', 'readied', 'mount_save', 'dismount'
    ));
