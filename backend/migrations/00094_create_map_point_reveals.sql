-- +goose Up
-- map_point_reveals records which characters know about a trap (MR-035, Etapa
-- 9, D5, RN-10): one row per trap point and character. A trap that is not
-- revealed to everyone (map_points.revealed_at) and has not been triggered is
-- sent only to the players of the characters listed here, and to the master.
--
-- how says why: 'noticed' (the character's passive Perception caught it, slice
-- 9.8), 'searched' (the player's search found it, slice 9.8) or 'master' (the
-- master revealed it to this character, RevealTrap). Revealing again to a
-- character who already knows keeps the first row. There is no way to take it
-- back: what a character saw stays seen, as for the clues (00070).
--
-- Deleting the trap or the character deletes the row. There is no index by
-- character_id: only deleting a character looks rows up that way, as for
-- map_tokens (00030). No personal data: IDs, a word and a time.
CREATE TABLE IF NOT EXISTS map_point_reveals (
    point_id UUID NOT NULL REFERENCES map_points (id) ON DELETE CASCADE,
    character_id UUID NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    how TEXT NOT NULL,
    at TIMESTAMPTZ NOT NULL,
    CONSTRAINT map_point_reveals_pkey PRIMARY KEY (point_id, character_id),
    CONSTRAINT map_point_reveals_how_valid CHECK (how IN ('noticed', 'searched', 'master'))
);

-- +goose Down
DROP TABLE IF EXISTS map_point_reveals;
