-- +goose Up
-- A rider owes a DC 10 Dexterity saving throw to stay on a mount that was moved against its will or when it is knocked prone
-- (SRD 5.1, "Mounted Combat"). The question is a reaction window of its own kind, `mount_save`, like the concentration save: the
-- turn waits for it, and the rider's roll is a held roll of the kind `mount_save` (Lucky and a Bardic Inspiration die are asked
-- about as for any saving throw). The answer is `mount_save_rolled` (the save, which closes the undo chain) and, when the rider
-- falls, `rider_thrown` just before it (the Prone condition, the square it landed on and its link cleared). The window itself writes no
-- event: the push or the prone that opened it stays the last thing the master can undo until it is answered. A mount knocked prone or
-- dropped to 0 hit points under its rider opens a window of the kind `dismount`: the rider's reaction to land on its feet. The widened
-- CHECKs only add values, so every row stays valid.
ALTER TABLE reaction_windows
    DROP CONSTRAINT IF EXISTS reaction_windows_kind_valid,
    ADD CONSTRAINT reaction_windows_kind_valid CHECK (kind IN (
        'shield', 'uncanny_dodge', 'hellish_rebuke', 'counterspell', 'cutting_words',
        'deflect_missiles', 'feather_fall', 'concentration_save', 'master_check', 'effect_save', 'contest',
        'maneuver_reduce', 'protection', 'legendary_resistance', 'parry', 'slow_fall', 'readied', 'mount_save', 'dismount'
    ));
ALTER TABLE inspiration_holds
    DROP CONSTRAINT IF EXISTS inspiration_holds_kind_valid,
    ADD CONSTRAINT inspiration_holds_kind_valid CHECK (kind IN (
        'scene_check', 'group_check', 'trap_search', 'hint_try', 'spell_attack', 'effect_save', 'concentration_save',
        'weapon_attack', 'death_save', 'initiative', 'contest_check', 'hide_check', 'counterspell_check', 'mount_save'
    ));
INSERT INTO session_event_kinds (kind) VALUES
    ('mount_save_rolled'),
    ('rider_thrown')
ON CONFLICT (kind) DO NOTHING;

-- +goose Down
DELETE FROM session_events WHERE kind IN ('mount_save_rolled', 'rider_thrown');
DELETE FROM session_event_kinds WHERE kind IN ('mount_save_rolled', 'rider_thrown');
DELETE FROM reaction_windows WHERE kind IN ('mount_save', 'dismount');
DELETE FROM inspiration_holds WHERE kind = 'mount_save';
ALTER TABLE inspiration_holds
    DROP CONSTRAINT IF EXISTS inspiration_holds_kind_valid,
    ADD CONSTRAINT inspiration_holds_kind_valid CHECK (kind IN (
        'scene_check', 'group_check', 'trap_search', 'hint_try', 'spell_attack', 'effect_save', 'concentration_save',
        'weapon_attack', 'death_save', 'initiative', 'contest_check', 'hide_check', 'counterspell_check'
    ));
ALTER TABLE reaction_windows
    DROP CONSTRAINT IF EXISTS reaction_windows_kind_valid,
    ADD CONSTRAINT reaction_windows_kind_valid CHECK (kind IN (
        'shield', 'uncanny_dodge', 'hellish_rebuke', 'counterspell', 'cutting_words',
        'deflect_missiles', 'feather_fall', 'concentration_save', 'master_check', 'effect_save', 'contest',
        'maneuver_reduce', 'protection', 'legendary_resistance', 'parry', 'slow_fall', 'readied'
    ));
