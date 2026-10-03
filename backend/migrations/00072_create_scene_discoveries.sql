-- +goose Up
-- scene_discoveries lists the scenes the group has discovered (MR-030, Etapa 8,
-- D6, question 61): a SCENE point was revealed on the map, or the master opened
-- it in a session (even while it was hidden). A player may tag a note only with
-- a discovered scene, and never learns the name of any other. It is shared by
-- the whole campaign and stays when the point is hidden again: what the players
-- have seen, they have seen. Written by the maps module only; one row per
-- point, the first time wins. Deleting the point or the campaign deletes it.
CREATE TABLE IF NOT EXISTS scene_discoveries (
    campaign_id UUID NOT NULL REFERENCES campaigns (id) ON DELETE CASCADE,
    point_id UUID NOT NULL REFERENCES map_points (id) ON DELETE CASCADE,
    discovered_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (campaign_id, point_id)
);

-- +goose Down
DROP TABLE IF EXISTS scene_discoveries;
