-- +goose Up
-- gallery_images.generated_kind is the way a generated image was made ('scene', 'map_scene',
-- 'isometric', 'textured_map'; '' for an upload), and an edit inherits the way of the image it
-- adjusts (MR-039, RN-10). A textured map is the whole map, the rooms the players have not found
-- included, so the app never offers it for showing to the players with one tap: it reads
-- generated_kind = 'textured_map' as "shows the whole map".
ALTER TABLE gallery_images ADD COLUMN IF NOT EXISTS generated_kind TEXT NOT NULL DEFAULT '';

-- image_requests.image_name is the name the gallery image will have (the master's, or the default
-- the server made): a player who is shown the image reads it, so it is never "Imagem N".
ALTER TABLE image_requests ADD COLUMN IF NOT EXISTS image_name TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE image_requests DROP COLUMN IF EXISTS image_name;
ALTER TABLE gallery_images DROP COLUMN IF EXISTS generated_kind;
