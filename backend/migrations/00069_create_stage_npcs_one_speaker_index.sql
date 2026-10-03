-- +goose Up
-- At most one NPC speaks on a session's stage (MR-031): the API clears the
-- old speaker before it marks a new one, and this index is what makes the rule
-- hold when two calls race.
CREATE UNIQUE INDEX IF NOT EXISTS stage_npcs_one_speaker ON stage_npcs (game_session_id) WHERE speaking;

-- +goose Down
DROP INDEX IF EXISTS stage_npcs_one_speaker;
