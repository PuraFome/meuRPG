-- +goose Up
-- The indexes of the maps module's tables (00028 to 00030), one statement
-- each, all safe to run again (IF NOT EXISTS). map_tokens needs none: its
-- primary key (map_id, character_id) lists a map's tokens.
--
-- A campaign's maps, oldest first (MapService.ListMaps), and the ON DELETE
-- CASCADE when a campaign is deleted.
CREATE INDEX IF NOT EXISTS maps_campaign_id_created_at_idx
    ON maps (campaign_id, created_at);

-- The maps that use a gallery image: the ON DELETE RESTRICT check when the
-- image is deleted, and DeleteGalleryImage's answer, which names them.
CREATE INDEX IF NOT EXISTS maps_image_id_idx
    ON maps (image_id);

-- A map's points, oldest first (MapService.GetMap), and the ON DELETE
-- CASCADE when the map is deleted.
CREATE INDEX IF NOT EXISTS map_points_map_id_created_at_idx
    ON map_points (map_id, created_at);

-- The submap points that lead to a map: the ON DELETE SET NULL when that
-- map is deleted.
CREATE INDEX IF NOT EXISTS map_points_target_map_id_idx
    ON map_points (target_map_id);

-- +goose Down
DROP INDEX IF EXISTS map_points_target_map_id_idx;
DROP INDEX IF EXISTS map_points_map_id_created_at_idx;
DROP INDEX IF EXISTS maps_image_id_idx;
DROP INDEX IF EXISTS maps_campaign_id_created_at_idx;
