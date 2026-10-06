-- +goose Up
-- battle_encounters is the encounter the master keeps on a battle point of a map (MR-043): the
-- creatures and counts, the base names, how the hit points are chosen and whether the
-- monsters start hidden. "Começar este combate" starts a combat from it. One row per point
-- (a new save replaces it).
--
-- encounter is the JSON of the API's BattleEncounter message. Only creature keys, counts and
-- the master's own base names are in it: the XP and the band are never kept, they are worked out
-- again against the party of the day. The row is the master's secret (RN-10): no read that
-- serves a player touches this table. map_id and campaign_id are copies of the point's, for the
-- list of a map's points that keep an encounter and for the campaign's own cascade.
--
-- Deleting the point, its map or the campaign deletes the row. No personal data: fiction only.
CREATE TABLE IF NOT EXISTS battle_encounters (
    map_point_id UUID PRIMARY KEY REFERENCES map_points (id) ON DELETE CASCADE,
    campaign_id UUID NOT NULL REFERENCES campaigns (id) ON DELETE CASCADE,
    map_id UUID NOT NULL REFERENCES maps (id) ON DELETE CASCADE,
    encounter JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS battle_encounters;
