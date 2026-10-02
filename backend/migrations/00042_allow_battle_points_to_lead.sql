-- +goose Up
-- A battle point may name the map where its fight happens (target_map_id),
-- like a submap point leads to another map (MR-013): starting the combat
-- from the point then makes that map the session's current map. Scene
-- points still lead nowhere. The API also checks that the target is a map
-- of the same campaign, and never the point's own map (map_points_not_own_target).
--
-- The old CHECK is dropped and a new one, with a name that says what it
-- allows, is added in the same statement, so re-running it is safe.
ALTER TABLE map_points
    DROP CONSTRAINT IF EXISTS map_points_only_submaps_lead,
    DROP CONSTRAINT IF EXISTS map_points_only_submaps_and_battles_lead,
    ADD CONSTRAINT map_points_only_submaps_and_battles_lead CHECK (kind IN ('submap', 'battle') OR target_map_id IS NULL);

-- +goose Down
UPDATE map_points SET target_map_id = NULL WHERE kind = 'battle';

ALTER TABLE map_points
    DROP CONSTRAINT IF EXISTS map_points_only_submaps_and_battles_lead,
    DROP CONSTRAINT IF EXISTS map_points_only_submaps_lead,
    ADD CONSTRAINT map_points_only_submaps_lead CHECK (kind = 'submap' OR target_map_id IS NULL);
