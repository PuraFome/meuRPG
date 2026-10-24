-- +goose Up
-- A scene can be open to only some players (MR-015, RN-10). open_scene_everyone is true when the open
-- scene is open to the whole table, a player who joins later included: today's behaviour, and the default.
-- When it is false the scene is open to exactly the players of game_session_scene_players, and a player
-- outside it reads no scene at all, as when none is open. It is a column, not "no rows", because erasing
-- the account of the last player must not turn a scene for some players into a scene for everyone.
--
-- The rows belong to the session (CASCADE) and go with the player's account (CASCADE). Opening another
-- scene or closing the open one clears them. Production is live: the column only adds, with every
-- session open to everyone as before, so running the migration twice is safe.
ALTER TABLE game_sessions ADD COLUMN IF NOT EXISTS open_scene_everyone BOOL NOT NULL DEFAULT true;

CREATE TABLE IF NOT EXISTS game_session_scene_players (
    game_session_id UUID NOT NULL REFERENCES game_sessions (id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    PRIMARY KEY (game_session_id, user_id)
);

CREATE INDEX IF NOT EXISTS game_session_scene_players_user_id_idx ON game_session_scene_players (user_id);

-- +goose Down
DROP INDEX IF EXISTS game_session_scene_players_user_id_idx;
DROP TABLE IF EXISTS game_session_scene_players;
ALTER TABLE game_sessions DROP COLUMN IF EXISTS open_scene_everyone;
