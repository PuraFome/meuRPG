-- +goose Up
-- The master's trap card and the session's pending list read a session's trap
-- damages (MR-035).
CREATE INDEX IF NOT EXISTS trap_damages_game_session_id_idx
    ON trap_damages (game_session_id, created_at);

-- +goose Down
DROP INDEX IF EXISTS trap_damages_game_session_id_idx;
