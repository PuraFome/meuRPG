-- +goose Up
-- vision_epoch is the generation of what the players remember of a map (MR-036,
-- D6): it goes up every time that memory is cleared (a new grid or image, "Esquecer
-- o que foi visto"). A remembered bitmap (map_vision_memory.epoch, 00106) only
-- counts while it carries the map's current epoch.
ALTER TABLE maps
    ADD COLUMN IF NOT EXISTS vision_epoch INT4 NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE maps
    DROP COLUMN IF EXISTS vision_epoch;
