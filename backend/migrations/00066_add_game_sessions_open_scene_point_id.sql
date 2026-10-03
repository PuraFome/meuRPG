-- +goose Up
-- open_scene_point_id is the RP scene the master opened in a game session
-- (PlayService.OpenScene, MR-015, D7): a map point of kind 'scene'. One at a
-- time, apart from the current map (00032) and the shown image (00033): the
-- players may have all three on screen. NULL means no scene is open; a new
-- session starts with none, and an ended session keeps the last one, as a
-- record. A hidden point can be opened: the scene does not reveal it on the
-- map (RN-10).
--
-- Deleting the point closes the scene (SET NULL). There is no index on it:
-- only deleting a point looks sessions up by it, and game_sessions stays
-- small (one row per evening played).
--
-- One statement with several parts, as 00033, so re-running it is safe.
ALTER TABLE game_sessions
    ADD COLUMN IF NOT EXISTS open_scene_point_id UUID NULL,
    DROP CONSTRAINT IF EXISTS game_sessions_open_scene_point_id_fkey,
    ADD CONSTRAINT game_sessions_open_scene_point_id_fkey
        FOREIGN KEY (open_scene_point_id) REFERENCES map_points (id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE game_sessions
    DROP CONSTRAINT IF EXISTS game_sessions_open_scene_point_id_fkey,
    DROP COLUMN IF EXISTS open_scene_point_id;
