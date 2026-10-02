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
SET shown_image_id = sqlc.narg(shown_image_id)
WHERE id = sqlc.arg(id)
RETURNING *;

-- Combat (MR-013). Every write below runs after the caller locked the open
-- session's row (GetOpenGameSessionForUpdate), so two changes to a combat take
-- turns, as for the vitals.

-- name: GetLatestEncounter :one
-- The session's latest combat, ended or not: GetEncounter shows it, so the app
-- can also show the end of a combat that just ended.
SELECT * FROM encounters
WHERE game_session_id = $1
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: GetOpenEncounter :one
-- The session's combat that is not ended, if any. The partial unique index
-- encounters_one_open_per_session allows at most one.
SELECT * FROM encounters
WHERE game_session_id = $1 AND status <> 'ended';

-- name: GetEncounterInSession :one
-- A combat by its ID, if it is in the session (so a combat of another
-- campaign matches no row).
SELECT * FROM encounters
WHERE game_session_id = $1 AND id = $2;

-- name: InsertEncounter :one
-- A new combat starts in setup, in round 0.
INSERT INTO encounters (game_session_id, map_id, map_point_id, name, status, grid_columns, grid_rows, created_at)
VALUES ($1, $2, $3, $4, 'setup', $5, $6, $7)
RETURNING *;

-- name: SetEncounterState :one
-- Where the combat is: its status, round and whose turn it is. Every change
-- to a combat raises its revision, so a write that changes only combatants
-- uses TouchEncounter.
UPDATE encounters
SET status = sqlc.arg(status), round = sqlc.arg(round), current_combatant_id = sqlc.narg(current_combatant_id),
    started_at = sqlc.narg(started_at), ended_at = sqlc.narg(ended_at), revision = revision + 1
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: TouchEncounter :one
UPDATE encounters
SET revision = revision + 1
WHERE id = $1
RETURNING *;

-- name: ListCombatants :many
-- The combat's combatants in turn order.
SELECT * FROM combatants
WHERE encounter_id = $1
ORDER BY order_index, created_at, id;

-- name: InsertCombatant :one
INSERT INTO combatants (
    encounter_id, character_id, user_id, label, kind, hidden, initiative, initiative_bonus, initiative_face,
    order_index, grid_col, grid_row, speed_ft, hp_current, hp_max, hp_temp, created_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9,
    $10, $11, $12, $13, $14, $15, $16, $17
)
RETURNING *;

-- name: SetCombatantInitiative :exec
-- A new roll breaks any tie order decided before (tie_ordered).
UPDATE combatants
SET initiative = $2, initiative_face = $3, tie_ordered = false
WHERE id = $1;

-- name: SetCombatantOrder :exec
UPDATE combatants
SET order_index = $2, tie_ordered = $3
WHERE id = $1;

-- name: SetCombatantSquare :exec
UPDATE combatants
SET grid_col = $2, grid_row = $3, movement_used_ft = $4
WHERE id = $1;

-- name: SetCombatantHidden :exec
UPDATE combatants
SET hidden = $2
WHERE id = $1;

-- name: ResetCombatantTurn :exec
-- The start of a combatant's own turn: movement, action, bonus action, dash and
-- reaction come back.
UPDATE combatants
SET movement_used_ft = 0, dashed = false, action_used = false, bonus_action_used = false, reaction_used = false
WHERE id = $1;

-- name: MarkCombatantDashed :exec
UPDATE combatants
SET dashed = true
WHERE id = $1;

-- name: DeleteCombatant :exec
DELETE FROM combatants
WHERE id = $1;
