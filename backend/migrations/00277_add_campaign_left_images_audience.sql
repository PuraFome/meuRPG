-- +goose Up
-- An image left with the players ("Deixar com os jogadores", MR-028) is left only with the players who
-- were in its audience when it stopped being shown. audience_everyone is true when it was left with the
-- whole table (today's behaviour, and the default); when it is false the players are the rows of
-- campaign_left_image_viewers. The flag is a column, not "no rows", because erasing the account of the
-- last player must not leave the image with everyone.
--
-- The rows go with the left image (CASCADE: taking it back, deleting it) and with the player's account
-- (CASCADE). Production is live: every left image keeps the whole table, so running the migration twice
-- is safe.
ALTER TABLE campaign_left_images ADD COLUMN IF NOT EXISTS audience_everyone BOOL NOT NULL DEFAULT true;

CREATE TABLE IF NOT EXISTS campaign_left_image_viewers (
    campaign_id UUID NOT NULL,
    image_id UUID NOT NULL,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    PRIMARY KEY (campaign_id, image_id, user_id),
    FOREIGN KEY (campaign_id, image_id) REFERENCES campaign_left_images (campaign_id, image_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS campaign_left_image_viewers_user_id_idx ON campaign_left_image_viewers (user_id);

-- +goose Down
DROP INDEX IF EXISTS campaign_left_image_viewers_user_id_idx;
DROP TABLE IF EXISTS campaign_left_image_viewers;
ALTER TABLE campaign_left_images DROP COLUMN IF EXISTS audience_everyone;
