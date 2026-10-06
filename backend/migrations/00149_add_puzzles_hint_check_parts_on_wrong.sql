-- +goose Up
-- What slice 10.7b adds to what the master writes in a puzzle (MR-038, RN-27), all
-- of it the master's alone (RN-10): no player reads this table.
--
-- hint_skill and hint_dc are the skill check that wins a player the next hint: the
-- skill's key as a scene check names it ('skill:investigation') and the DC (1 to 30).
-- Both are NULL, or both set. parts is the split information: a JSON array of
-- {character_id, text}, up to 8, the text 1 to 300 characters; character_id is empty
-- for a part with no owner yet. Ids inside it have no foreign key: a character that
-- is deleted later only leaves its part without a reader. on_wrong is the protojson
-- of PuzzleOnWrong ("Ao errar": a trap point, the attempts per player, the limit of
-- moves, the time limit); NULL for nothing. The trap point inside it has no foreign
-- key either: a point deleted later fires nothing.
--
-- The riddle's accepted answers, the sequence and the cipher's message and key are not
-- columns: they are in the solution JSON, as the lock's and the pillars' are.
ALTER TABLE puzzles
    ADD COLUMN IF NOT EXISTS hint_skill TEXT NULL,
    ADD COLUMN IF NOT EXISTS hint_dc INT4 NULL,
    ADD COLUMN IF NOT EXISTS parts JSONB NOT NULL DEFAULT '[]',
    ADD COLUMN IF NOT EXISTS on_wrong JSONB NULL;

-- +goose Down
ALTER TABLE puzzles
    DROP COLUMN IF EXISTS on_wrong,
    DROP COLUMN IF EXISTS parts,
    DROP COLUMN IF EXISTS hint_dc,
    DROP COLUMN IF EXISTS hint_skill;
