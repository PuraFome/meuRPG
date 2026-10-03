-- +goose Up
-- xp_award_shares are what each character got from an award (MR-016): one row
-- per character and award. xp is the character's share (the total split,
-- rounded down, question 46), 0 for a milestone. level_at_mark is, for a
-- milestone only, the character's total level when the master marked it: the
-- "pode subir de nível" tag lasts until the level on the sheet goes past it
-- (RN-12, question 48), so it is compared on read and nothing is written when
-- the master raises the level.
--
-- Deleting the award, or the character, deletes the share (a deleted pending
-- character, or an account deletion's characters, never keep XP history).
CREATE TABLE IF NOT EXISTS xp_award_shares (
    award_id UUID NOT NULL REFERENCES xp_awards (id) ON DELETE CASCADE,
    character_id UUID NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    xp INT4 NOT NULL,
    level_at_mark INT4 NULL,
    PRIMARY KEY (award_id, character_id),
    CONSTRAINT xp_award_shares_xp_valid CHECK (xp BETWEEN 0 AND 1000000),
    CONSTRAINT xp_award_shares_level_valid CHECK (level_at_mark IS NULL OR level_at_mark BETWEEN 1 AND 20)
);

-- +goose Down
DROP TABLE IF EXISTS xp_award_shares;
