-- +goose Up
-- A d20 that shows a low face also waits for the reroll the character's table features give (the engine's reroll
-- effect, RerollRule: "Applies" a roll kind, "AtMost" the highest face that may be rolled again, "Resource" and "Cost"
-- the uses it spends, none when it is free). SRD 5.1 puts the same words on the halfling's Lucky: the die is rolled
-- again and the new roll must be used, even when it is worse. It comes after the halfling's Lucky and before the
-- Bardic Inspiration die (inspiration_holds, 00244, 00254), since the die is added to whatever the d20 is after it.
--   - reroll_ask is true once the question was asked: the d20 that counts showed a face of at most reroll_at_most and
--     the character could pay the rule. It waits while it is true and reroll_answer_key and answer_key are NULL.
--   - reroll_index is the die that showed the low face (0, or 0 and 1 for a pair: the one that counts), reroll_at_most
--     the rule's limit, reroll_resource and reroll_cost the uses the answer spends ('' and 0 for a free rule) and
--     reroll_source the feature that gives it. They are kept on the row so the answer needs nothing from the sheet.
--   - reroll_answer_key is the idempotency key of the answer, set when it is given.
--   - reroll_from and reroll_to are the face that counted and the new face; NULL when the player kept the roll.
-- A Bardic Inspiration die answered while a later question follows is kept on the row with Indomitable's columns
-- (die_answer_key, die_used, 00290).
--
-- Production is live: the columns only add and have defaults or allow NULL, and the checks are dropped before they are
-- added, so running the migration twice is safe.
ALTER TABLE inspiration_holds ADD COLUMN IF NOT EXISTS reroll_ask BOOL NOT NULL DEFAULT FALSE;
ALTER TABLE inspiration_holds ADD COLUMN IF NOT EXISTS reroll_index INT4 NULL;
ALTER TABLE inspiration_holds ADD COLUMN IF NOT EXISTS reroll_at_most INT4 NULL;
ALTER TABLE inspiration_holds ADD COLUMN IF NOT EXISTS reroll_resource TEXT NOT NULL DEFAULT '';
ALTER TABLE inspiration_holds ADD COLUMN IF NOT EXISTS reroll_cost INT4 NOT NULL DEFAULT 0;
ALTER TABLE inspiration_holds ADD COLUMN IF NOT EXISTS reroll_source TEXT NOT NULL DEFAULT '';
ALTER TABLE inspiration_holds ADD COLUMN IF NOT EXISTS reroll_answer_key UUID NULL;
ALTER TABLE inspiration_holds ADD COLUMN IF NOT EXISTS reroll_from INT4 NULL;
ALTER TABLE inspiration_holds ADD COLUMN IF NOT EXISTS reroll_to INT4 NULL;

ALTER TABLE inspiration_holds DROP CONSTRAINT IF EXISTS inspiration_holds_reroll_face_valid;
ALTER TABLE inspiration_holds ADD CONSTRAINT inspiration_holds_reroll_face_valid CHECK (
    (reroll_from IS NULL OR reroll_from BETWEEN 1 AND 20)
    AND (reroll_to IS NULL OR reroll_to BETWEEN 1 AND 20)
    AND (reroll_index IS NULL OR reroll_index BETWEEN 0 AND 1)
    AND reroll_cost >= 0);

-- +goose Down
ALTER TABLE inspiration_holds DROP CONSTRAINT IF EXISTS inspiration_holds_reroll_face_valid;
ALTER TABLE inspiration_holds DROP COLUMN IF EXISTS reroll_to;
ALTER TABLE inspiration_holds DROP COLUMN IF EXISTS reroll_from;
ALTER TABLE inspiration_holds DROP COLUMN IF EXISTS reroll_answer_key;
ALTER TABLE inspiration_holds DROP COLUMN IF EXISTS reroll_source;
ALTER TABLE inspiration_holds DROP COLUMN IF EXISTS reroll_cost;
ALTER TABLE inspiration_holds DROP COLUMN IF EXISTS reroll_resource;
ALTER TABLE inspiration_holds DROP COLUMN IF EXISTS reroll_at_most;
ALTER TABLE inspiration_holds DROP COLUMN IF EXISTS reroll_index;
ALTER TABLE inspiration_holds DROP COLUMN IF EXISTS reroll_ask;
