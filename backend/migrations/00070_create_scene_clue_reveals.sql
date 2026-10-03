-- +goose Up
-- scene_clue_reveals records that the master revealed a clue to a player
-- (MR-029, Etapa 8, D5, question 59): one row per clue and player. There is no
-- un-reveal. It is also what the player reads in their notes (MR-030): the row
-- keeps a COPY of the clue's text, so what the player received stays as it was
-- said at the table, even when the master edits or removes the clue, or deletes
-- the point.
--
-- user_id is the player (the notes are the player's, and a character can lose
-- its player); character_id is the character the master picked, for the
-- master's "Só a Brisa". The unique index on (clue_id, user_id) (00071) makes
-- revealing twice to the same player change nothing; it has no effect once
-- clue_id is NULL, which is fine: the clue is gone.
--
-- clue_id and point_id become NULL when the clue or the point is deleted: the
-- player keeps the text, without the scene tag. Deleting the account deletes
-- what the person received (CASCADE on user_id); deleting the campaign deletes
-- everything.
CREATE TABLE IF NOT EXISTS scene_clue_reveals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id UUID NOT NULL REFERENCES campaigns (id) ON DELETE CASCADE,
    clue_id UUID NULL REFERENCES scene_clues (id) ON DELETE SET NULL,
    point_id UUID NULL REFERENCES map_points (id) ON DELETE SET NULL,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    character_id UUID NULL REFERENCES characters (id) ON DELETE SET NULL,
    text TEXT NOT NULL,
    revealed_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT scene_clue_reveals_text_length CHECK (char_length(text) BETWEEN 1 AND 500)
);

-- +goose Down
DROP TABLE IF EXISTS scene_clue_reveals;
