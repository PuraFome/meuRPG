-- +goose Up
-- A session's lines in the order they were sent: what ListSessionMessages pages through (and the
-- cascade from the session scans), with the id as the tie-break of two lines of the same instant.
CREATE INDEX IF NOT EXISTS session_messages_game_session_id_created_at_id_idx
    ON session_messages (game_session_id, created_at, id);

-- +goose Down
DROP INDEX IF EXISTS session_messages_game_session_id_created_at_id_idx;
