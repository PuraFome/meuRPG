-- +goose Up
-- The image the master shows has an audience (MR-028, RN-10): 'table' is every member (today's behaviour,
-- and the default), 'scene' is the players of the scene open to only some players
-- (game_session_scene_players, so it follows who is in while that scene stays open), and 'chosen' is the
-- players of game_session_image_viewers. When the scene stops being the image's (it closes, another
-- opens, or it opens to everyone) the server turns 'scene' into 'chosen' with the players who were in,
-- so nothing already shown reaches anyone else.
--
-- The rows belong to the session (CASCADE) and go with the player's account (CASCADE); the kind stays a
-- column, so erasing the last viewer never widens the audience. Production is live: every shown image
-- keeps the whole table, so running the migration twice is safe.
ALTER TABLE game_sessions ADD COLUMN IF NOT EXISTS shown_image_audience TEXT NOT NULL DEFAULT 'table';

ALTER TABLE game_sessions DROP CONSTRAINT IF EXISTS game_sessions_shown_image_audience_check;
ALTER TABLE game_sessions ADD CONSTRAINT game_sessions_shown_image_audience_check CHECK (shown_image_audience IN ('table', 'scene', 'chosen'));

CREATE TABLE IF NOT EXISTS game_session_image_viewers (
    game_session_id UUID NOT NULL REFERENCES game_sessions (id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    PRIMARY KEY (game_session_id, user_id)
);

CREATE INDEX IF NOT EXISTS game_session_image_viewers_user_id_idx ON game_session_image_viewers (user_id);

-- +goose Down
DROP INDEX IF EXISTS game_session_image_viewers_user_id_idx;
DROP TABLE IF EXISTS game_session_image_viewers;
ALTER TABLE game_sessions DROP CONSTRAINT IF EXISTS game_sessions_shown_image_audience_check;
ALTER TABLE game_sessions DROP COLUMN IF EXISTS shown_image_audience;
