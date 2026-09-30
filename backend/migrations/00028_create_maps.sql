-- +goose Up
-- maps holds a campaign's maps (MR-008): an image of the campaign's gallery
-- with points of interest (map_points) and tokens (map_tokens) on top of it.
--
-- image_id is the gallery image under the map. ON DELETE RESTRICT: an image
-- a map uses cannot be deleted (MR-019: "o app diz em qual mapa ela está");
-- the master changes the map's image first. The API also checks that the
-- image belongs to the map's campaign. Deleting the campaign still works:
-- CockroachDB checks RESTRICT at the end of the statement, after the
-- campaign's maps are gone too.
--
-- revealed_at is when the master showed the map to the players (RN-10).
-- NULL means hidden, and a new map starts hidden ("O mapa nasce
-- escondido"). A player sees a map when it is revealed or when it is the
-- open game session's current map (game_sessions.current_map_id, 00032);
-- the server never sends them any other.
--
-- name is free text the master writes; the players read it once the map is
-- revealed. revision goes up by one every time the name or the image
-- change, so two tabs editing the same map cannot silently overwrite each
-- other (the API answers `aborted` to a stale revision). Revealing and
-- hiding do not change it.
--
-- Deleting the campaign deletes its maps, and with them their points and
-- tokens. There is no personal data here besides the name.
CREATE TABLE IF NOT EXISTS maps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id UUID NOT NULL REFERENCES campaigns (id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    image_id UUID NOT NULL REFERENCES gallery_images (id) ON DELETE RESTRICT,
    revealed_at TIMESTAMPTZ NULL,
    revision INT4 NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT maps_name_length CHECK (char_length(name) BETWEEN 1 AND 80),
    CONSTRAINT maps_revision_valid CHECK (revision >= 1)
);

-- +goose Down
DROP TABLE IF EXISTS maps;
