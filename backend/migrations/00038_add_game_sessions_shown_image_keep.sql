-- +goose Up
-- shown_image_keep is the master's "Deixar com os jogadores" switch for the
-- image the session shows now (PlayService.SetShownImage, MR-028). While it
-- is on, stopping the show, showing another image, or ending the session
-- moves the image to the campaign's left images (campaign_left_images,
-- 00039) instead of taking it away from the players. It belongs to the
-- image being shown, so it goes back to false whenever the shown image
-- changes. No foreign key and no index: it is a plain flag on the row.
--
-- One statement, as 00032, so re-running it is safe.
ALTER TABLE game_sessions
    ADD COLUMN IF NOT EXISTS shown_image_keep BOOL NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE game_sessions
    DROP COLUMN IF EXISTS shown_image_keep;
