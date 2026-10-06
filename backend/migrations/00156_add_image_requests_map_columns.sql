-- +goose Up
-- The requests made from a map (MR-039, RN-28, slice 10.8b): the scene art and
-- the isometric view of what the players see now, and the textured map of the
-- whole map. map_id is the map; map_image_id, map_grid_columns, map_grid_factor,
-- map_width and map_height are the map as the request saw it, so
-- "Usar como imagem do mapa" can tell that the map is still the one the textured
-- map fits (the same image, the same grid, the same calibration, the same size in
-- pixels) and refuse when it is not. map_plan_hash is a hash of the floor plan the
-- drawing showed (the walls, a secret door as wall): a wall painted or a secret door
-- revealed since changes it, and the textured map no longer fits. pad_x0..pad_y1 are
-- the map's rectangle on the drawing's padded canvas, as fractions (0 to 1), which
-- the answer is cropped with. used_map_image_id is the image "Usar como imagem do mapa"
-- set on the map (the picture itself, or the copy a fog map got), so a retry of a Use
-- that already happened answers the same while that is still the map's image. All of them are NULL for the other kinds.
--
-- map_id SET NULL when the map goes, map_image_id when the old image is deleted
-- (then the textured map no longer fits anything). The columns hold IDs and
-- numbers, no text of the master's (docs/privacidade.md).
--
-- kind has three values more: 'map_scene', 'isometric' and 'textured_map'.
--
-- One statement, so it is safe to run twice (the constraint is dropped and added
-- again, as the TTL migrations do).
ALTER TABLE image_requests
    ADD COLUMN IF NOT EXISTS map_id UUID NULL,
    ADD COLUMN IF NOT EXISTS map_image_id UUID NULL,
    ADD COLUMN IF NOT EXISTS map_grid_columns INT4 NULL,
    ADD COLUMN IF NOT EXISTS map_grid_factor INT4 NULL,
    ADD COLUMN IF NOT EXISTS map_width INT4 NULL,
    ADD COLUMN IF NOT EXISTS map_height INT4 NULL,
    ADD COLUMN IF NOT EXISTS map_plan_hash TEXT NULL,
    ADD COLUMN IF NOT EXISTS pad_x0 FLOAT8 NULL,
    ADD COLUMN IF NOT EXISTS pad_y0 FLOAT8 NULL,
    ADD COLUMN IF NOT EXISTS pad_x1 FLOAT8 NULL,
    ADD COLUMN IF NOT EXISTS pad_y1 FLOAT8 NULL,
    ADD COLUMN IF NOT EXISTS used_map_image_id UUID NULL,
    DROP CONSTRAINT IF EXISTS image_requests_map_id_fkey,
    ADD CONSTRAINT image_requests_map_id_fkey FOREIGN KEY (map_id) REFERENCES maps (id) ON DELETE SET NULL,
    DROP CONSTRAINT IF EXISTS image_requests_map_image_id_fkey,
    ADD CONSTRAINT image_requests_map_image_id_fkey FOREIGN KEY (map_image_id) REFERENCES gallery_images (id) ON DELETE SET NULL,
    DROP CONSTRAINT IF EXISTS image_requests_kind_valid,
    ADD CONSTRAINT image_requests_kind_valid CHECK (kind IN ('scene', 'edit', 'map_scene', 'isometric', 'textured_map'));

-- +goose Down
ALTER TABLE image_requests
    DROP CONSTRAINT IF EXISTS image_requests_kind_valid,
    ADD CONSTRAINT image_requests_kind_valid CHECK (kind IN ('scene', 'edit')),
    DROP COLUMN IF EXISTS used_map_image_id,
    DROP COLUMN IF EXISTS pad_y1,
    DROP COLUMN IF EXISTS pad_x1,
    DROP COLUMN IF EXISTS pad_y0,
    DROP COLUMN IF EXISTS pad_x0,
    DROP COLUMN IF EXISTS map_plan_hash,
    DROP COLUMN IF EXISTS map_height,
    DROP COLUMN IF EXISTS map_width,
    DROP COLUMN IF EXISTS map_grid_factor,
    DROP COLUMN IF EXISTS map_grid_columns,
    DROP COLUMN IF EXISTS map_image_id,
    DROP COLUMN IF EXISTS map_id;
