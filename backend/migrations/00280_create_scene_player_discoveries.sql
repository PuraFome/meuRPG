-- +goose Up
-- A scene open to only some players is discovered by those players alone (MR-030, RN-10): they may tag their
-- notes with it, and nobody else learns its name from their notes. scene_discoveries stays the group's list
-- (a scene revealed on the map, or opened to the whole table); this table holds, per player, the scenes only
-- they discovered: the master opened a scene to them, brought them in, or they joined it. It stays when the
-- player leaves the scene: what a player read, they read. The rows go with the point, the campaign or the
-- player's account. Written by the maps module only; the first time wins.
--
-- Production is live: the table only adds, so running the migration twice is safe.
CREATE TABLE IF NOT EXISTS scene_player_discoveries (
    campaign_id UUID NOT NULL REFERENCES campaigns (id) ON DELETE CASCADE,
    point_id UUID NOT NULL REFERENCES map_points (id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    discovered_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (campaign_id, point_id, user_id)
);

CREATE INDEX IF NOT EXISTS scene_player_discoveries_user_id_idx ON scene_player_discoveries (user_id);

-- +goose Down
DROP INDEX IF EXISTS scene_player_discoveries_user_id_idx;
DROP TABLE IF EXISTS scene_player_discoveries;
