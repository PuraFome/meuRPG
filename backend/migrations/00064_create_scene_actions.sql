-- +goose Up
-- scene_actions holds the checks the master put on an RP scene point
-- (MR-015, D6): a skill check, an ability check or a saving throw, each with
-- an optional name ("Convencer o guarda") and an optional difficulty class.
-- A scene point has at most 20 of them (the API checks, inside the
-- transaction that inserts one) and only a point of kind 'scene' has any (the
-- API checks that too, and deletes them when the point changes kind).
--
-- key is "skill:investigation", "ability:str" or "save:wis": the API checks
-- it against the rules' catalog, and the CHECK keeps nothing else in, so an
-- attack, a spell or a combat feature never becomes a scene action. name is
-- the master's free text, at most 60 characters, empty for none. dc is 1 to
-- 30, NULL for none; only the master ever receives it (RN-20).
--
-- position orders the actions of a point, from 0. It is not unique: a move
-- renumbers the point's actions in one transaction. Deleting the point
-- deletes its actions (CASCADE).
CREATE TABLE IF NOT EXISTS scene_actions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    point_id UUID NOT NULL REFERENCES map_points (id) ON DELETE CASCADE,
    position INT4 NOT NULL,
    key TEXT NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    dc INT4 NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT scene_actions_position_valid CHECK (position >= 0),
    CONSTRAINT scene_actions_key_valid CHECK (key ~ '^(skill:[a-z-]+|ability:(str|dex|con|int|wis|cha)|save:(str|dex|con|int|wis|cha))$'),
    CONSTRAINT scene_actions_name_length CHECK (char_length(name) <= 60),
    CONSTRAINT scene_actions_dc_valid CHECK (dc IS NULL OR dc BETWEEN 1 AND 30)
);

-- +goose Down
DROP TABLE IF EXISTS scene_actions;
