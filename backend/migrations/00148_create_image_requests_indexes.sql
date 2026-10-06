-- +goose Up
-- One request per idempotency key in a campaign: a retry finds the first.
CREATE UNIQUE INDEX IF NOT EXISTS image_requests_campaign_key_idx
    ON image_requests (campaign_id, idempotency_key);

-- The monthly count and the "Imagem n" number read a campaign's rows by month.
CREATE INDEX IF NOT EXISTS image_requests_campaign_month_idx
    ON image_requests (campaign_id, quota_month);

-- The edit chain reads the children of an image.
CREATE INDEX IF NOT EXISTS gallery_images_parent_image_id_idx
    ON gallery_images (parent_image_id)
    WHERE parent_image_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS gallery_images_parent_image_id_idx;
DROP INDEX IF EXISTS image_requests_campaign_month_idx;
DROP INDEX IF EXISTS image_requests_campaign_key_idx;
