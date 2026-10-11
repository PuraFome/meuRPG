-- +goose Up
-- The rolls that wait for the player's answer about a Bardic Inspiration die (inspiration_holds,
-- 00244) are no longer only the scene check and the group check: a trap search, a puzzle hint try,
-- a spell attack, an effect's saving throw and a concentration save wait too (SRD 5.1: the creature
-- may wait until after the d20 and decide before the GM says whether the roll succeeds). The kind
-- list grows, and round is the combat's round for a roll held in a combat, where the die is the
-- combatant's: a hold of an earlier round is stale and forgotten, like the attack holds of
-- roll_holds. NULL for a roll out of a combat. The kind check is dropped by both names it can
-- have (PostgreSQL's and CockroachDB's), and the new one is dropped first too, so the migration
-- can run twice.
ALTER TABLE inspiration_holds
    DROP CONSTRAINT IF EXISTS inspiration_holds_kind_check,
    DROP CONSTRAINT IF EXISTS check_kind,
    DROP CONSTRAINT IF EXISTS inspiration_holds_kind_valid,
    ADD CONSTRAINT inspiration_holds_kind_valid CHECK (kind IN (
        'scene_check', 'group_check', 'trap_search', 'hint_try', 'spell_attack', 'effect_save', 'concentration_save'
    )),
    ADD COLUMN IF NOT EXISTS round INT4 NULL;

-- +goose Down
DELETE FROM inspiration_holds WHERE kind NOT IN ('scene_check', 'group_check');
ALTER TABLE inspiration_holds
    DROP COLUMN IF EXISTS round,
    DROP CONSTRAINT IF EXISTS inspiration_holds_kind_valid,
    ADD CONSTRAINT inspiration_holds_kind_valid CHECK (kind IN ('scene_check', 'group_check'));
