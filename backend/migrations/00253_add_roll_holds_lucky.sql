-- +goose Up
-- A held attack roll (roll_holds, 00220) also waits for the halfling's Lucky (SRD 5.1, Halfling:
-- a 1 on the d20 of an attack roll, an ability check or a saving throw may be rolled again, and the
-- new roll must be used). A lucky attacker whose d20 shows a 1 gets the question "Tirou 1: rolar de
-- novo?" before the result, the same way a Bardic Inspiration die waits; with advantage or
-- disadvantage only one of the two d20 is rolled again, the one that shows a 1, even when it is the
-- die that does not count.
--   - lucky_index is the die that shows the 1, as an index into the faces in the order rolled (0
--     for a single d20, 0 or 1 for a pair); NULL when the hold asks no Lucky question.
--   - lucky_answer_key is the idempotency key of the Lucky answer, set when it is given, so that
--     a retry of the answer finds it given and another answer is refused. The question waits while
--     lucky_index is set and lucky_answer_key and answer_key are NULL; a hold whose attacker also
--     holds a Bardic Inspiration die stays open after it, for the die's question on the new roll.
--   - lucky_to is the new face of the die rolled again; NULL when the player kept the 1.
-- face and request then hold the d20 after the answer. Production is live: the columns only add,
-- and the checks are dropped before they are added, so running the migration twice is safe.
ALTER TABLE roll_holds ADD COLUMN IF NOT EXISTS lucky_index INT4 NULL;
ALTER TABLE roll_holds ADD COLUMN IF NOT EXISTS lucky_answer_key TEXT NULL;
ALTER TABLE roll_holds ADD COLUMN IF NOT EXISTS lucky_to INT4 NULL;

ALTER TABLE roll_holds DROP CONSTRAINT IF EXISTS roll_holds_lucky_index_valid;
ALTER TABLE roll_holds ADD CONSTRAINT roll_holds_lucky_index_valid CHECK (lucky_index IS NULL OR (lucky_index >= 0 AND lucky_index <= 1));
ALTER TABLE roll_holds DROP CONSTRAINT IF EXISTS roll_holds_lucky_to_valid;
ALTER TABLE roll_holds ADD CONSTRAINT roll_holds_lucky_to_valid CHECK (lucky_to IS NULL OR (lucky_to >= 1 AND lucky_to <= 20));

-- +goose Down
ALTER TABLE roll_holds DROP CONSTRAINT IF EXISTS roll_holds_lucky_to_valid;
ALTER TABLE roll_holds DROP CONSTRAINT IF EXISTS roll_holds_lucky_index_valid;
ALTER TABLE roll_holds DROP COLUMN IF EXISTS lucky_to;
ALTER TABLE roll_holds DROP COLUMN IF EXISTS lucky_answer_key;
ALTER TABLE roll_holds DROP COLUMN IF EXISTS lucky_index;
