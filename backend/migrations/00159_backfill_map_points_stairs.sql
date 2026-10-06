-- +goose Up
-- The stairs the generator made before the column existed: submap points with no target and the
-- generator's own names (a one-time fill; new stairs are written with their direction).
UPDATE map_points SET stairs = 'up'
WHERE stairs IS NULL AND kind = 'submap' AND target_map_id IS NULL AND name = 'Escada para cima'
  AND map_id IN (SELECT map_id FROM generated_dungeons);
UPDATE map_points SET stairs = 'down'
WHERE stairs IS NULL AND kind = 'submap' AND target_map_id IS NULL AND name = 'Escada para baixo'
  AND map_id IN (SELECT map_id FROM generated_dungeons);

-- +goose Down
SELECT 1;
