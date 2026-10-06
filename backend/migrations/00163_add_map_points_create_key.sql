-- +goose Up
-- create_key is the idempotency key of "Pôr no mapa" (TreasureService.PlaceTreasure,
-- MR-044): the campaign's ID, a colon and the 1 to 64 characters the app chose once for
-- the dialog it opened, so a key is unique in the campaign. A retry with the same key
-- finds the point the first call made instead of creating a second one. create_hash is
-- the hash of the whole request (map, square, mode, level, seed, name and content
-- version): a retry with the same key and another request is refused. NULL for every
-- other point (the other creates have no key). Both are opaque, not personal data.
ALTER TABLE map_points ADD COLUMN IF NOT EXISTS create_key TEXT NULL;
ALTER TABLE map_points ADD COLUMN IF NOT EXISTS create_hash TEXT NULL;

-- +goose Down
ALTER TABLE map_points DROP COLUMN IF EXISTS create_hash;
ALTER TABLE map_points DROP COLUMN IF EXISTS create_key;
