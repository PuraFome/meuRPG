-- +goose Up
-- encounters are the combats of a game session (MR-013): "setup" while the
-- master picks who fights and everybody rolls initiative, "active" while the
-- turns run, "ended" afterwards. A session has at most one that is not
-- ended (the partial unique index of 00045), and it runs on one map's grid.
-- The ended ones stay, as the session's record.
--
-- map_id is the map the fight is on. Deleting the map unsets it (the fight
-- goes on, without a drawing); the grid is copied here (grid_columns,
-- grid_rows) when the encounter is created, so changing the map's grid
-- later never moves a combatant. map_point_id is the battle point the
-- master started it from, if any (SET NULL when the point goes away).
--
-- round is 0 in setup and counts from 1 once combat begins.
-- current_combatant_id is whose turn it is: NULL in setup and when it ended.
-- It has no foreign key on purpose: combatants refer to encounters, and the
-- service moves the turn before it deletes the current combatant.
--
-- revision goes up by one on every change, so the app can tell a newer copy
-- from an older one. name is free text the master writes (1 to 80
-- characters), like a map's name. Deleting the session, or its campaign,
-- deletes its encounters and, with them, their combatants. No personal data
-- here: names of fiction, numbers and IDs.
CREATE TABLE IF NOT EXISTS encounters (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    game_session_id UUID NOT NULL REFERENCES game_sessions (id) ON DELETE CASCADE,
    map_id UUID NULL REFERENCES maps (id) ON DELETE SET NULL,
    map_point_id UUID NULL REFERENCES map_points (id) ON DELETE SET NULL,
    name TEXT NOT NULL,
    status TEXT NOT NULL,
    round INT4 NOT NULL DEFAULT 0,
    current_combatant_id UUID NULL,
    grid_columns INT4 NOT NULL,
    grid_rows INT4 NOT NULL,
    revision INT4 NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL,
    started_at TIMESTAMPTZ NULL,
    ended_at TIMESTAMPTZ NULL,
    CONSTRAINT encounters_name_length CHECK (char_length(name) BETWEEN 1 AND 80),
    CONSTRAINT encounters_status_valid CHECK (status IN ('setup', 'active', 'ended')),
    CONSTRAINT encounters_round_valid CHECK (round >= 0),
    CONSTRAINT encounters_grid_valid CHECK (grid_columns BETWEEN 4 AND 200 AND grid_rows BETWEEN 1 AND 400),
    CONSTRAINT encounters_revision_valid CHECK (revision >= 1)
);

-- +goose Down
DROP TABLE IF EXISTS encounters;
