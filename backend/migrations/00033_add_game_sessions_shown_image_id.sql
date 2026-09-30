-- +goose Up
-- shown_image_id is the gallery image the master shows the players during
-- a game session (PlayService.SetShownImage, MR-028): a handout, such as a
-- portrait, a letter or a scene. One at a time, and apart from the current
-- map (00032): the players may have both on screen. It reveals nothing
-- else; a player only learns the image's ID while it is shown. NULL means
-- nothing is shown; a new session starts with nothing, and an ended session
-- keeps the last one, as a record.
--
-- Deleting the image stops showing it (SET NULL). There is no index on it:
-- only deleting an image looks sessions up by it, and game_sessions stays
-- small (one row per evening played), so that scan is cheap.
--
-- One statement with several parts, as 00032, so re-running it is safe.
ALTER TABLE game_sessions
    ADD COLUMN IF NOT EXISTS shown_image_id UUID NULL,
    DROP CONSTRAINT IF EXISTS game_sessions_shown_image_id_fkey,
    ADD CONSTRAINT game_sessions_shown_image_id_fkey
        FOREIGN KEY (shown_image_id) REFERENCES gallery_images (id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE game_sessions
    DROP CONSTRAINT IF EXISTS game_sessions_shown_image_id_fkey,
    DROP COLUMN IF EXISTS shown_image_id;
