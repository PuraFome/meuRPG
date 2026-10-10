-- +goose Up
-- Deleting a gallery image looks up the points that hold it (the CASCADE of
-- map_point_images.image_id). The primary key starts with point_id, so this
-- is the index that lookup uses.
CREATE INDEX IF NOT EXISTS map_point_images_image_id_idx
    ON map_point_images (image_id);

-- +goose Down
DROP INDEX IF EXISTS map_point_images_image_id_idx;
