-- +goose Up
-- generated and parent_image_id mark the images the master asked an image
-- model for (MR-039, RN-28, slice 10.8). generated is true for an image that
-- ImageGenerationService stored: the gallery labels it "Gerada por IA". An
-- image made by editing another one (a new instruction, the previous image
-- sent again) points at it with parent_image_id, which is the edit chain.
-- Deleting the parent keeps the child and cuts the link (SET NULL): the child
-- is a complete image on its own.
--
-- One statement with several parts, so re-running it is safe (see 00033).
ALTER TABLE gallery_images
    ADD COLUMN IF NOT EXISTS generated BOOL NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS parent_image_id UUID NULL,
    DROP CONSTRAINT IF EXISTS gallery_images_parent_image_id_fkey,
    ADD CONSTRAINT gallery_images_parent_image_id_fkey
        FOREIGN KEY (parent_image_id) REFERENCES gallery_images (id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE gallery_images
    DROP CONSTRAINT IF EXISTS gallery_images_parent_image_id_fkey,
    DROP COLUMN IF EXISTS parent_image_id,
    DROP COLUMN IF EXISTS generated;
