-- +goose Up
-- A scene open to only some players may let the other players join (MR-015, RN-10). scene_players_may_join is the
-- master's switch "Os jogadores podem entrar": off by default, changed while the scene is open, and set back to off
-- when the scene closes or another opens. With it on a player outside the scene reads its name and that they can join,
-- and nothing else, until they join. joined_by_self marks a player who joined by themselves (JoinScene) against one the
-- master chose, so the master sees who joined and how.
--
-- Production is live: both columns only add, with every row as it was, so running the migration twice is safe.
ALTER TABLE game_sessions ADD COLUMN IF NOT EXISTS scene_players_may_join BOOL NOT NULL DEFAULT false;
ALTER TABLE game_session_scene_players ADD COLUMN IF NOT EXISTS joined_by_self BOOL NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE game_session_scene_players DROP COLUMN IF EXISTS joined_by_self;
ALTER TABLE game_sessions DROP COLUMN IF EXISTS scene_players_may_join;
