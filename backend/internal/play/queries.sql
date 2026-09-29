-- name: GetOpenGameSession :one
-- The campaign's open session, if any. The partial unique index
-- game_sessions_one_open_per_campaign allows at most one.
SELECT * FROM game_sessions
WHERE campaign_id = $1 AND ended_at IS NULL;

-- name: NextSessionNumber :one
-- Sessions count from 1. Two starts racing both read the same number;
-- CockroachDB's SERIALIZABLE isolation makes one of them retry, and the
-- UNIQUE (campaign_id, session_number) constraint is the backstop.
SELECT (COALESCE(max(session_number), 0) + 1)::INT4 AS next
FROM game_sessions
WHERE campaign_id = $1;

-- name: InsertGameSession :one
INSERT INTO game_sessions (campaign_id, session_number, started_at)
VALUES ($1, $2, $3)
RETURNING *;

-- name: EndGameSession :one
-- Ending a session twice keeps the first ended_at, so the call is
-- idempotent. GREATEST keeps ended_at from being before started_at if two
-- servers' clocks disagree by a little (game_sessions_ends_after_start).
UPDATE game_sessions
SET ended_at = COALESCE(ended_at, GREATEST(sqlc.arg(now)::TIMESTAMPTZ, started_at))
WHERE campaign_id = $1 AND id = $2
RETURNING *;

-- name: ListGameSessions :many
-- Newest first.
SELECT * FROM game_sessions
WHERE campaign_id = $1
ORDER BY session_number DESC;
