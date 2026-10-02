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

-- name: GetOpenGameSessionForUpdate :one
-- GetOpenGameSession, locking the session's row until the transaction ends.
-- Every change made during a session locks it first, so two changes at
-- once take turns: each one reads the next event number after the other
-- wrote its own, and EndGameSession waits for a change in progress.
SELECT * FROM game_sessions
WHERE campaign_id = $1 AND ended_at IS NULL
FOR UPDATE;

-- name: GetGameSessionForUpdate :one
-- One session of the campaign, locked until the transaction ends.
SELECT * FROM game_sessions
WHERE campaign_id = $1 AND id = $2
FOR UPDATE;

-- name: ListOpenGameSessions :many
-- The open sessions of the given campaigns, newest first (RN-06). The
-- partial unique index game_sessions_one_open_per_campaign finds each one.
SELECT * FROM game_sessions
WHERE campaign_id = ANY(sqlc.arg(campaign_ids)::UUID[]) AND ended_at IS NULL
ORDER BY started_at DESC, id;

-- name: GetSessionEventByIdempotencyKey :one
-- The event a change with this key already wrote, if any.
SELECT id, seq, kind, character_id FROM session_events
WHERE game_session_id = $1 AND idempotency_key = $2;

-- name: NextSessionEventSeq :one
-- Events count from 1 in each session. The caller holds the session's row
-- lock (GetOpenGameSessionForUpdate), so no other change reads the same
-- number; UNIQUE (game_session_id, seq) is the backstop.
SELECT (COALESCE(max(seq), 0) + 1)::INT4 AS next
FROM session_events
WHERE game_session_id = $1;

-- name: InsertSessionEvent :one
INSERT INTO session_events
    (game_session_id, seq, kind, actor_user_id, character_id, payload, idempotency_key, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, seq;

-- name: GetOnScreen :one
-- What the open session shows: its current map and the image the master
-- shows (either NULL when none). No row: no open session.
SELECT current_map_id, shown_image_id FROM game_sessions
WHERE campaign_id = $1 AND ended_at IS NULL;

-- name: SetCurrentMap :one
-- The caller holds the session's row lock (GetOpenGameSessionForUpdate).
UPDATE game_sessions
SET current_map_id = sqlc.narg(current_map_id)
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: SetShownImage :one
-- The caller holds the session's row lock (GetOpenGameSessionForUpdate).
UPDATE game_sessions
SET shown_image_id = sqlc.narg(shown_image_id), shown_image_keep = sqlc.arg(shown_image_keep)
WHERE id = sqlc.arg(id)
RETURNING *;
