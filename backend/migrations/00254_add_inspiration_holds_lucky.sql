-- +goose Up
-- The held rolls that are not a weapon attack (inspiration_holds, 00244 and 00251: a scene check,
-- a group check, a trap search, a puzzle hint try, and, in a combat, a spell attack against one
-- target, an effect's saving throw and a concentration save) also wait for the halfling's Lucky
-- (SRD 5.1, Halfling: a 1 on the d20 of an attack roll, an ability check or a saving throw may be
-- rolled again, and the new roll must be used), the way the weapon attacks of roll_holds do
-- (00253). A lucky character whose d20 shows a 1 is held even when it holds no Bardic Inspiration
-- die, and its player gets "Tirou 1: rolar de novo?" before the result; with advantage or
-- disadvantage only the die that shows a 1 is rolled again, even when it is the one that does
-- not count.
--   - lucky_index is the die that shows the 1, as an index into faces (0 for a single d20, 0 or 1
--     for a pair); NULL when the hold asks no Lucky question.
--   - lucky_answer_key is the idempotency key of the Lucky answer, set when it is given, so that
--     a retry of the answer finds it given and another answer is refused. The question waits while
--     lucky_index is set and lucky_answer_key and answer_key are NULL; a hold whose character also
--     holds a Bardic Inspiration die stays open after it, for the die's question on the new roll.
--   - lucky_to is the new face of the die rolled again; NULL when the player kept the 1.
-- faces, counted and total then hold the d20 after the answer.
--
-- A puzzle hint try keeps its d20 in its own row (puzzle_hint_tries, 00152), so it keeps the die
-- Lucky rolled again there too: lucky_index is that die, as an index into the faces in the order
-- rolled; d20 and d20_b already hold the new face, and the 1 it replaced is known (Lucky only
-- asks about a 1). NULL when nothing was rolled again. A retry of the try's request (with the 1
-- it typed) is matched against the faces before the reroll.
--
-- Production is live: the columns only add, and the checks are dropped before they are added, so
-- running the migration twice is safe.
ALTER TABLE inspiration_holds ADD COLUMN IF NOT EXISTS lucky_index INT4 NULL;
ALTER TABLE inspiration_holds ADD COLUMN IF NOT EXISTS lucky_answer_key UUID NULL;
ALTER TABLE inspiration_holds ADD COLUMN IF NOT EXISTS lucky_to INT4 NULL;

ALTER TABLE inspiration_holds DROP CONSTRAINT IF EXISTS inspiration_holds_lucky_index_valid;
ALTER TABLE inspiration_holds ADD CONSTRAINT inspiration_holds_lucky_index_valid CHECK (lucky_index IS NULL OR (lucky_index >= 0 AND lucky_index <= 1));
ALTER TABLE inspiration_holds DROP CONSTRAINT IF EXISTS inspiration_holds_lucky_to_valid;
ALTER TABLE inspiration_holds ADD CONSTRAINT inspiration_holds_lucky_to_valid CHECK (lucky_to IS NULL OR (lucky_to >= 1 AND lucky_to <= 20));

ALTER TABLE puzzle_hint_tries ADD COLUMN IF NOT EXISTS lucky_index INT4 NULL;

ALTER TABLE puzzle_hint_tries DROP CONSTRAINT IF EXISTS puzzle_hint_tries_lucky_index_valid;
ALTER TABLE puzzle_hint_tries ADD CONSTRAINT puzzle_hint_tries_lucky_index_valid CHECK (lucky_index IS NULL OR (lucky_index >= 0 AND lucky_index <= 1));

-- +goose Down
ALTER TABLE puzzle_hint_tries DROP CONSTRAINT IF EXISTS puzzle_hint_tries_lucky_index_valid;
ALTER TABLE puzzle_hint_tries DROP COLUMN IF EXISTS lucky_index;
ALTER TABLE inspiration_holds DROP CONSTRAINT IF EXISTS inspiration_holds_lucky_to_valid;
ALTER TABLE inspiration_holds DROP CONSTRAINT IF EXISTS inspiration_holds_lucky_index_valid;
ALTER TABLE inspiration_holds DROP COLUMN IF EXISTS lucky_to;
ALTER TABLE inspiration_holds DROP COLUMN IF EXISTS lucky_answer_key;
ALTER TABLE inspiration_holds DROP COLUMN IF EXISTS lucky_index;
