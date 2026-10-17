-- +goose Up
-- The held rolls (inspiration_holds, 00244, 00251 and 00254) grow to hold the weapon attack too,
-- which waits in roll_holds today (00253), so that one table holds every roll that waits for the
-- player's answer. This migration only adds; nothing reads or writes the new columns yet, and the
-- weapon attack keeps its own table until a later release moves its writes here.
--   - combatant_id is the roller in a combat, and the owner of the hold when it is set: the copies of
--     an NPC and the creatures of a combat share a character_id, a combatant does not. NULL for a
--     roll out of a combat, and for a roll held in a combat before the code wrote it.
--   - encounter_id is the combat the roll was held in, so the combat's view lists its holds by it.
--   - window_hold says the hold is the attack a reaction window kept (roll_holds marked it with a
--     "#window" suffix of its idempotency key, which a UUID column cannot carry).
--   - kind accepts the weapon attack, and the kinds of the rolls that will wait next (a death save,
--     an initiative roll, a contest, a Hide check and the check of a Counterspell): one change of the
--     check on the live table instead of one for each. The code refuses a kind it does not know.
--
-- Production is live: the columns are nullable or have a default, so they validate at once, and the
-- kind check is dropped by every name it can have (PostgreSQL's and CockroachDB's) before the new
-- one is added, so running the migration twice is safe.

-- The foreign keys are dropped by name before they are added: a column added with REFERENCES would
-- get a second key, with another name, at every run (TestMigrationsAreSafeToRerun), as 00241 does.
ALTER TABLE inspiration_holds
    ADD COLUMN IF NOT EXISTS combatant_id UUID NULL,
    DROP CONSTRAINT IF EXISTS inspiration_holds_combatant_id_fkey,
    ADD CONSTRAINT inspiration_holds_combatant_id_fkey FOREIGN KEY (combatant_id) REFERENCES combatants (id) ON DELETE CASCADE,
    ADD COLUMN IF NOT EXISTS encounter_id UUID NULL,
    DROP CONSTRAINT IF EXISTS inspiration_holds_encounter_id_fkey,
    ADD CONSTRAINT inspiration_holds_encounter_id_fkey FOREIGN KEY (encounter_id) REFERENCES encounters (id) ON DELETE CASCADE,
    ADD COLUMN IF NOT EXISTS window_hold BOOL NOT NULL DEFAULT false;

ALTER TABLE inspiration_holds
    DROP CONSTRAINT IF EXISTS inspiration_holds_kind_check,
    DROP CONSTRAINT IF EXISTS check_kind,
    DROP CONSTRAINT IF EXISTS inspiration_holds_kind_valid,
    ADD CONSTRAINT inspiration_holds_kind_valid CHECK (kind IN (
        'scene_check', 'group_check', 'trap_search', 'hint_try', 'spell_attack', 'effect_save', 'concentration_save',
        'weapon_attack', 'death_save', 'initiative', 'contest_check', 'hide_check', 'counterspell_check'
    ));

-- +goose Down
DELETE FROM inspiration_holds WHERE kind NOT IN (
    'scene_check', 'group_check', 'trap_search', 'hint_try', 'spell_attack', 'effect_save', 'concentration_save'
);
ALTER TABLE inspiration_holds
    DROP CONSTRAINT IF EXISTS inspiration_holds_kind_valid,
    ADD CONSTRAINT inspiration_holds_kind_valid CHECK (kind IN (
        'scene_check', 'group_check', 'trap_search', 'hint_try', 'spell_attack', 'effect_save', 'concentration_save'
    ));
ALTER TABLE inspiration_holds
    DROP COLUMN IF EXISTS window_hold,
    DROP COLUMN IF EXISTS encounter_id,
    DROP COLUMN IF EXISTS combatant_id;
