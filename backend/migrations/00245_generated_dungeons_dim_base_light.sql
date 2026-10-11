-- +goose Up
-- A map with the base light "Claro" now shows the players its whole floor plan,
-- which would give away the rooms behind a secret door of a generated dungeon
-- (MR-010). New dungeons are made in "Penumbra"; this moves the existing ones that
-- are still in "Claro" to "Penumbra", which keeps what the players saw until now.
-- Only the maps of generated_dungeons, only bright to dim: running it again changes
-- nothing. The light_revision goes up so the cached scenes are worked out again.
UPDATE maps
SET base_light = 'dim', light_revision = light_revision + 1
WHERE base_light = 'bright'
  AND id IN (SELECT map_id FROM generated_dungeons);

-- +goose Down
-- Nothing to undo: the old value cannot be told from one the master chose.
SELECT 1;
