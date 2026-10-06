-- +goose Up
-- The gallery image the generator drew for the map (MR-010, slice 10.6d). "Redesenhar"
-- only ever replaces, and deletes, this image: when the map's image is another one (the
-- master uploaded their own) the map is no longer the generated one. NULL reads as "not
-- known", which never matches an image. It follows the image when the image is deleted.
ALTER TABLE generated_dungeons
    ADD COLUMN IF NOT EXISTS image_id UUID NULL,
    DROP CONSTRAINT IF EXISTS generated_dungeons_image_id_fkey,
    ADD CONSTRAINT generated_dungeons_image_id_fkey
        FOREIGN KEY (image_id) REFERENCES gallery_images (id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE generated_dungeons
    DROP CONSTRAINT IF EXISTS generated_dungeons_image_id_fkey,
    DROP COLUMN IF EXISTS image_id;
