-- +goose Up
-- stage_npcs is the stage of a game session (MR-031, D7): the NPCs the master
-- put "em cena" in the open RP scene, in the order they came in. A session
-- has at most 4 at once (the API checks, inside the transaction that inserts
-- one, with the session's row locked), and at most one of them speaks
-- (stage_npcs_one_speaker, 00069, backs the API's check). Closing or switching
-- the scene empties the stage: its rows are deleted.
--
-- id is the place on the stage, made when the NPC comes in: it is what a
-- player's copy of the stage carries instead of the character's ID, which
-- stays the master's (RN-20). position orders the NPCs, from 0, and is not
-- reused: a new NPC takes the highest position plus one, so taking one off
-- leaves a hole that changes no order.
--
-- Deleting the session or the character deletes the row (CASCADE): an NPC the
-- master deletes leaves the stage. No personal data: fiction and IDs.
CREATE TABLE IF NOT EXISTS stage_npcs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    game_session_id UUID NOT NULL REFERENCES game_sessions (id) ON DELETE CASCADE,
    character_id UUID NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    position INT4 NOT NULL,
    speaking BOOL NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT stage_npcs_position_valid CHECK (position >= 0),
    CONSTRAINT stage_npcs_one_per_session UNIQUE (game_session_id, character_id)
);

-- +goose Down
DROP TABLE IF EXISTS stage_npcs;
