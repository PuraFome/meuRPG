-- +goose Up
-- The foreign keys of image_requests to the gallery and the maps (MR-039): deleting
-- an image or a map looks up the requests that point at it (ON DELETE SET NULL),
-- and "Usar como imagem do mapa" and the edit chain look a request up by its image.
-- Partial indexes: most rows (a plain scene) have none of these.
CREATE INDEX IF NOT EXISTS image_requests_image_id_idx
    ON image_requests (image_id)
    WHERE image_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS image_requests_source_image_id_idx
    ON image_requests (source_image_id)
    WHERE source_image_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS image_requests_map_id_idx
    ON image_requests (map_id)
    WHERE map_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS image_requests_map_image_id_idx
    ON image_requests (map_image_id)
    WHERE map_image_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS image_requests_map_image_id_idx;
DROP INDEX IF EXISTS image_requests_map_id_idx;
DROP INDEX IF EXISTS image_requests_source_image_id_idx;
DROP INDEX IF EXISTS image_requests_image_id_idx;
