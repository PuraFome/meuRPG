-- +goose Up
-- A combatant can ride another one (mounted combat, SRD 5.1 "Mounted Combat"). The rider and the mount stay two rows.
--
-- mounted_on_id is on the RIDER: the combatant it sits on, NULL when it rides nothing. The foreign key sets it back to NULL when
-- the mount's row goes (the mount left the combat), and the partial unique index lets a mount carry one rider only. A row never
-- rides itself.
--
-- mount_kind is on the MOUNT: '' is not a willing mount, 'controlled' obeys its rider and 'independent' keeps its own turn. The
-- master marks it (SetCombatantMount).
--
-- mount_prior_init is NULL or one JSON object {"initiative": <total or null>, "face": <d20 or null>, "tie": <the master ordered it>, "bonus": <the modifier>, "after": <the id of the combatant before it>}:
-- the initiative, the d20, the master's order and the place of a mount.
--
-- mount_move_used says the rider already mounted or dismounted in this move (once per move); the start of its turn clears it.
--
-- The check constraints are named and dropped first, so the migration can run twice (an inline CHECK on ADD COLUMN IF NOT EXISTS
-- adds a second constraint on the rerun in CockroachDB). No personal data: ids, a word and numbers.
ALTER TABLE combatants
    ADD COLUMN IF NOT EXISTS mounted_on_id UUID NULL,
    ADD COLUMN IF NOT EXISTS mount_kind TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS mount_prior_init JSONB NULL,
    ADD COLUMN IF NOT EXISTS mount_move_used BOOL NOT NULL DEFAULT false;
ALTER TABLE combatants DROP CONSTRAINT IF EXISTS combatants_mounted_on_id_fkey;
ALTER TABLE combatants ADD CONSTRAINT combatants_mounted_on_id_fkey
    FOREIGN KEY (mounted_on_id) REFERENCES combatants (id) ON DELETE SET NULL;
ALTER TABLE combatants DROP CONSTRAINT IF EXISTS combatants_mounted_on_not_self;
ALTER TABLE combatants ADD CONSTRAINT combatants_mounted_on_not_self CHECK (mounted_on_id IS NULL OR mounted_on_id <> id);
ALTER TABLE combatants DROP CONSTRAINT IF EXISTS combatants_mount_kind_valid;
ALTER TABLE combatants ADD CONSTRAINT combatants_mount_kind_valid CHECK (mount_kind IN ('', 'controlled', 'independent'));
ALTER TABLE combatants DROP CONSTRAINT IF EXISTS combatants_mount_prior_init_valid;
ALTER TABLE combatants ADD CONSTRAINT combatants_mount_prior_init_valid
    CHECK (mount_prior_init IS NULL OR jsonb_typeof(mount_prior_init) = 'object');

CREATE UNIQUE INDEX IF NOT EXISTS combatants_mounted_on_id_idx ON combatants (mounted_on_id) WHERE mounted_on_id IS NOT NULL;

-- The master marked a combatant as a willing mount, or took the mark off (SetCombatantMount, `mount_kind_set`), and a rider got on
-- a mount or off it (`combatant_mounted`, `combatant_dismounted`). The payloads hold ids, the kind and the numbers the undo puts
-- back. The row is secret when a hidden combatant is in it.
INSERT INTO session_event_kinds (kind) VALUES
    ('mount_kind_set'),
    ('combatant_mounted'),
    ('combatant_dismounted')
ON CONFLICT (kind) DO NOTHING;

-- A rider owes a DC 10 Dexterity saving throw to stay on a mount that was moved against its will or when it is knocked prone
-- (SRD 5.1, "Mounted Combat"). The question is a reaction window of its own kind, `mount_save`, like the concentration save: the
-- turn waits for it, and the rider's roll is a held roll of the kind `mount_save` (Lucky and a Bardic Inspiration die are asked
-- about as for any saving throw). The answer is `mount_save_rolled` (the save, which closes the undo chain) and, when the rider
-- falls, `rider_thrown` just before it (the Prone condition, the square it landed on and its link cleared). The window itself writes no
-- event: the push or the prone that opened it stays the last thing the master can undo until it is answered. The widened
-- CHECKs only add values, so every row stays valid.
ALTER TABLE reaction_windows
    DROP CONSTRAINT IF EXISTS reaction_windows_kind_valid,
    ADD CONSTRAINT reaction_windows_kind_valid CHECK (kind IN (
        'shield', 'uncanny_dodge', 'hellish_rebuke', 'counterspell', 'cutting_words',
        'deflect_missiles', 'feather_fall', 'concentration_save', 'master_check', 'effect_save', 'contest',
        'maneuver_reduce', 'protection', 'legendary_resistance', 'parry', 'slow_fall', 'mount_save'
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
DELETE FROM reaction_windows WHERE kind = 'mount_save';
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
        'maneuver_reduce', 'protection', 'legendary_resistance', 'parry', 'slow_fall'
    ));
DELETE FROM session_events WHERE kind IN ('mount_kind_set', 'combatant_mounted', 'combatant_dismounted');
DELETE FROM session_event_kinds WHERE kind IN ('mount_kind_set', 'combatant_mounted', 'combatant_dismounted');
DROP INDEX IF EXISTS combatants_mounted_on_id_idx;
ALTER TABLE combatants DROP CONSTRAINT IF EXISTS combatants_mount_prior_init_valid;
ALTER TABLE combatants DROP CONSTRAINT IF EXISTS combatants_mount_kind_valid;
ALTER TABLE combatants DROP CONSTRAINT IF EXISTS combatants_mounted_on_not_self;
ALTER TABLE combatants DROP CONSTRAINT IF EXISTS combatants_mounted_on_id_fkey;
ALTER TABLE combatants DROP COLUMN IF EXISTS mount_move_used;
ALTER TABLE combatants DROP COLUMN IF EXISTS mount_prior_init;
ALTER TABLE combatants DROP COLUMN IF EXISTS mount_kind;
ALTER TABLE combatants DROP COLUMN IF EXISTS mounted_on_id;
