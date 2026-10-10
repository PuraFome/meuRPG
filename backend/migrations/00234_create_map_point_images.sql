-- +goose Up
-- map_point_images holds the gallery images the master attached to an RP
-- scene point ("Imagens da cena", MR-015): an ordered list, at most 8 per
-- point (the API checks, inside the transaction that replaces the list). Only
-- a point of kind 'scene' has any (the API checks that too, and clears the
-- list when the point changes kind). An image is on a point at most once.
--
-- The list is the master's preparation, like the clues: a player is never told
-- which images a scene holds (RN-10). A player sees one of them only when the
-- master shows it with the gallery's "Mostrar aos jogadores" (MR-019), which
-- is its own action and does not read this table.
--
-- position orders the images of a point, from 0; the API rewrites the whole
-- list in one transaction, so it is never read half changed. Deleting the
-- point or the gallery image deletes the row (CASCADE): the other one stays.
CREATE TABLE IF NOT EXISTS map_point_images (
    point_id UUID NOT NULL REFERENCES map_points (id) ON DELETE CASCADE,
    image_id UUID NOT NULL REFERENCES gallery_images (id) ON DELETE CASCADE,
    position INT4 NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (point_id, image_id),
    CONSTRAINT map_point_images_position_valid CHECK (position >= 0)
);

-- +goose Down
DROP TABLE IF EXISTS map_point_images;
