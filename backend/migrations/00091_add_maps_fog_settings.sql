-- +goose Up
-- The map's fog of war settings and the revision of its painted layers
-- (MR-036, Etapa 9, D6, D2).
--
-- fog_enabled turns the fog of war on for the map (off by default: existing
-- maps behave as before). The API refuses to turn it on for a map without a
-- grid (the fog is made of squares), and removing the grid turns it off.
-- base_light is the light of a square nobody lit and the master did not paint:
-- 'dark' (the default), 'dim' or 'bright'. group_vision makes every player see
-- what any player character sees (off by default). Turning the fog off keeps
-- the painted layers (map_layers, 00092) and the other two settings.
--
-- layers_revision goes up by one every time a painted layer (not the light)
-- changes or all of them are cleared; light_revision does the same for the
-- painted light alone, which no player reads, so painting it must not move the
-- number players see (the master reads the sum), so a reader can tell a newer copy of the layers from an older one.
-- It is apart from revision on purpose: revision guards the name and the image
-- (UpdateMap answers `aborted` to a stale one), and painting must not make a
-- rename from another tab fail.
--
-- One statement with several parts, as 00067, so re-running it is safe.
ALTER TABLE maps
    ADD COLUMN IF NOT EXISTS fog_enabled BOOL NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS base_light TEXT NOT NULL DEFAULT 'dark',
    ADD COLUMN IF NOT EXISTS group_vision BOOL NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS layers_revision INT4 NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS light_revision INT4 NOT NULL DEFAULT 0,
    DROP CONSTRAINT IF EXISTS maps_base_light_valid,
    ADD CONSTRAINT maps_base_light_valid CHECK (base_light IN ('dark', 'dim', 'bright')),
    DROP CONSTRAINT IF EXISTS maps_layers_revision_valid,
    ADD CONSTRAINT maps_layers_revision_valid CHECK (layers_revision >= 0 AND light_revision >= 0);

-- +goose Down
ALTER TABLE maps
    DROP CONSTRAINT IF EXISTS maps_layers_revision_valid,
    DROP CONSTRAINT IF EXISTS maps_base_light_valid,
    DROP COLUMN IF EXISTS light_revision,
    DROP COLUMN IF EXISTS layers_revision,
    DROP COLUMN IF EXISTS group_vision,
    DROP COLUMN IF EXISTS base_light,
    DROP COLUMN IF EXISTS fog_enabled;
