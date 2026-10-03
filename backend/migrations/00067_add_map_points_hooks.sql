-- +goose Up
-- hooks is the master's private text on a SCENE point, "Ganchos e anotações"
-- (MR-029, Etapa 8, D5): Markdown, like the campaign document, at most 4,000
-- characters, empty for none. Only the master ever receives it (RN-20): no
-- player read, no stream event and no session event carries it. It saves with
-- the point, like the description. Only a scene has hooks: the API keeps it
-- empty on any other kind and clears it when the point stops being a scene.
--
-- One statement with several parts, as 00033, so re-running it is safe.
ALTER TABLE map_points
    ADD COLUMN IF NOT EXISTS hooks TEXT NOT NULL DEFAULT '',
    DROP CONSTRAINT IF EXISTS map_points_hooks_length,
    ADD CONSTRAINT map_points_hooks_length CHECK (char_length(hooks) <= 4000);

-- +goose Down
ALTER TABLE map_points
    DROP CONSTRAINT IF EXISTS map_points_hooks_length,
    DROP COLUMN IF EXISTS hooks;
