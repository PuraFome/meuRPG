-- +goose Up
-- Once per player: the guard of "revealing again changes nothing".
CREATE UNIQUE INDEX IF NOT EXISTS scene_clue_reveals_clue_id_user_id_idx
    ON scene_clue_reveals (clue_id, user_id);

-- A player's received clues, newest first: every ListNotes asks.
CREATE INDEX IF NOT EXISTS scene_clue_reveals_campaign_id_user_id_revealed_at_idx
    ON scene_clue_reveals (campaign_id, user_id, revealed_at DESC);

-- A clue's recipients, for the master's "quem tem": every map and scene read.
CREATE INDEX IF NOT EXISTS scene_clue_reveals_point_id_idx
    ON scene_clue_reveals (point_id);

-- +goose Down
DROP INDEX IF EXISTS scene_clue_reveals_point_id_idx;
DROP INDEX IF EXISTS scene_clue_reveals_campaign_id_user_id_revealed_at_idx;
DROP INDEX IF EXISTS scene_clue_reveals_clue_id_user_id_idx;
