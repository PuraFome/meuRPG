-- +goose Up
-- current_map_id is the map the master shows at the table during a game
-- session (PlayService.SetCurrentMap): the session page draws it for
-- everyone. Setting a map current also reveals it (RN-10). NULL means the
-- master has not chosen one; a new session starts without one. An ended
-- session keeps the last one, as a record.
--
-- Deleting the map unsets it (SET NULL). There is no index on it: only
-- deleting a map looks sessions up by it, and game_sessions stays small
-- (one row per evening played), so that scan is cheap.
--
-- One statement with several parts, so re-running it is safe: ADD COLUMN IF
-- NOT EXISTS skips a column that exists, and the named foreign key is
-- dropped, if present, and added again. (An inline REFERENCES in ADD COLUMN
-- IF NOT EXISTS would add a second copy of the foreign key on every run.)
ALTER TABLE game_sessions
    ADD COLUMN IF NOT EXISTS current_map_id UUID NULL,
    DROP CONSTRAINT IF EXISTS game_sessions_current_map_id_fkey,
    ADD CONSTRAINT game_sessions_current_map_id_fkey
        FOREIGN KEY (current_map_id) REFERENCES maps (id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE game_sessions
    DROP CONSTRAINT IF EXISTS game_sessions_current_map_id_fkey,
    DROP COLUMN IF EXISTS current_map_id;
