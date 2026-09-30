-- +goose Up
-- Lists a campaign's gallery, newest first, and adds up its usage for the
-- quota (count and bytes) without reading the table: byte_size is stored
-- in the index. It also serves the ON DELETE CASCADE when a campaign is
-- deleted.
CREATE INDEX IF NOT EXISTS gallery_images_campaign_id_created_at_idx
    ON gallery_images (campaign_id, created_at DESC)
    INCLUDE (byte_size);

-- +goose Down
DROP INDEX IF EXISTS gallery_images_campaign_id_created_at_idx;
