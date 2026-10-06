-- +goose Up
-- critical_max is what a critical hit adds to the damage without rolling it: the
-- maximum of the damage dice, under the table's rule "o máximo mais uma rolagem"
-- (RN-24, MR-025). With that rule dice_count is the number of dice that are
-- rolled (not doubled) and the damage is the roll, dice_bonus and critical_max.
-- 0 for any hit that is not a critical one, and under the SRD's "dados dobrados"
-- (dice_count already holds the doubled dice), which is every row made before
-- this column. A trap's damage (in a combat, pending_damages; outside one,
-- trap_damages) follows the same rule.
-- critical_max_rule says the table's rule was "máximo mais uma rolagem" when the
-- hit was opened, so a critical hit with a flat damage (no dice, so nothing to
-- keep) still reports the rule it was made under.
ALTER TABLE pending_damages
    ADD COLUMN IF NOT EXISTS critical_max INT4 NOT NULL DEFAULT 0;

ALTER TABLE pending_damages
    ADD COLUMN IF NOT EXISTS critical_max_rule BOOL NOT NULL DEFAULT false;

ALTER TABLE trap_damages
    ADD COLUMN IF NOT EXISTS critical_max INT4 NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE trap_damages
    DROP COLUMN IF EXISTS critical_max;

ALTER TABLE pending_damages
    DROP COLUMN IF EXISTS critical_max_rule;

ALTER TABLE pending_damages
    DROP COLUMN IF EXISTS critical_max;
