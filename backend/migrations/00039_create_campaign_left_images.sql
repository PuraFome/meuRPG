-- +goose Up
-- campaign_left_images lists the gallery images the master left with the
-- players (MR-028, "Deixar com os jogadores"): the players of the campaign
-- may keep looking at them until the master takes them back
-- (PlayService.TakeBackLeftImage). They belong to the campaign, not to a
-- game session: a left image stays after the session ends, and the next
-- session starts with the same list. Only the images' IDs travel to the
-- players, and the image route serves a player an image of this table
-- (RN-10).
--
-- The primary key (campaign_id, image_id) lists a campaign's images and
-- keeps an image from being left twice; left_at orders the list. Deleting
-- the campaign or the gallery image deletes the row (CASCADE): a deleted
-- image is gone for the players too. No index on image_id: only deleting
-- an image looks it up, and a campaign leaves a handful of images.
CREATE TABLE IF NOT EXISTS campaign_left_images (
    campaign_id UUID NOT NULL REFERENCES campaigns (id) ON DELETE CASCADE,
    image_id UUID NOT NULL REFERENCES gallery_images (id) ON DELETE CASCADE,
    left_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (campaign_id, image_id)
);

-- +goose Down
DROP TABLE IF EXISTS campaign_left_images;
