-- +goose Up
-- map_points holds a map's points of interest (MR-008): a battle, a submap
-- or an RP scene, at a spot of the map's image. Opening a point shows its
-- name and description; a submap point also leads to another map
-- (target_map_id). Battles open their encounter with combat (Etapa 6) and
-- scenes their RP scene (Etapa 7).
--
-- x_bp and y_bp are the point's position in basis points of the image's
-- width and height: 0 is the left (top) edge, 10000 the right (bottom)
-- edge. They do not depend on the image's size in pixels, so the map can be
-- drawn at any zoom.
--
-- revealed_at works as in maps (RN-10, MR-009): NULL is hidden, and a new
-- point starts hidden. The server never sends a hidden point to a player,
-- not even its ID.
--
-- name and description are free text the master writes for the players to
-- read once the point is revealed: the description is at most 2,000
-- characters and may have line breaks.
--
-- target_map_id is only for a submap point, and never the point's own map
-- (the CHECKs). The API also checks that it is a map of the same campaign.
-- Deleting the target map leaves the point without a target (SET NULL);
-- deleting the point's own map deletes the point (CASCADE).
CREATE TABLE IF NOT EXISTS map_points (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    map_id UUID NOT NULL REFERENCES maps (id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    x_bp INT4 NOT NULL,
    y_bp INT4 NOT NULL,
    target_map_id UUID NULL REFERENCES maps (id) ON DELETE SET NULL,
    revealed_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT map_points_kind_valid CHECK (kind IN ('battle', 'submap', 'scene')),
    CONSTRAINT map_points_name_length CHECK (char_length(name) BETWEEN 1 AND 80),
    CONSTRAINT map_points_description_length CHECK (char_length(description) <= 2000),
    CONSTRAINT map_points_position_valid CHECK (x_bp BETWEEN 0 AND 10000 AND y_bp BETWEEN 0 AND 10000),
    CONSTRAINT map_points_only_submaps_lead CHECK (kind = 'submap' OR target_map_id IS NULL),
    CONSTRAINT map_points_not_own_target CHECK (target_map_id IS NULL OR target_map_id <> map_id)
);

-- +goose Down
DROP TABLE IF EXISTS map_points;
