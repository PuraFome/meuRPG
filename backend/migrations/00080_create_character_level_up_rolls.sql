-- +goose Up
-- character_level_up_rolls keeps the hit die the server rolled for a
-- character's next level (MR-040, RN-18), so asking again returns the same
-- result: the player cannot reroll until they like it, and the result "fica no
-- registro". One row per character and target level (the character's total
-- level after the level-up), not per class: a multiclass character gets one
-- roll for the level, so it cannot roll a d6 and a d12 for it and keep the
-- better. class_key is the class that rolled, and the roll serves only that
-- class. LevelUpCharacter deletes the row it uses, and its value goes on in
-- character_level_ups.
--
-- A roll that was never used stays until the level is reached: if the master
-- lowers the character's level on the sheet, the roll for a level above it is
-- kept and still stands when the character gets there again (the level-up that
-- reaches it deletes it).
--
-- die is the class's hit die (6, 8, 10 or 12) and value the result, 1 to die.
-- Deleting the character deletes the rows. No personal data, only numbers.
CREATE TABLE IF NOT EXISTS character_level_up_rolls (
    character_id UUID NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    class_key TEXT NOT NULL,
    to_level INT4 NOT NULL,
    die INT4 NOT NULL,
    value INT4 NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (character_id, to_level),
    CONSTRAINT character_level_up_rolls_valid CHECK (die IN (6, 8, 10, 12) AND value >= 1 AND value <= die AND to_level >= 2 AND to_level <= 20)
);

-- +goose Down
DROP TABLE IF EXISTS character_level_up_rolls;
