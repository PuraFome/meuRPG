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
SELECT id, seq, kind, character_id, payload FROM session_events
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
    (game_session_id, seq, kind, actor_user_id, character_id, payload, idempotency_key, created_at, encounter_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
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

-- name: SetCombatantHitPoints :exec
-- An NPC's hit points, temporary hit points and defeated flag (damage, healing,
-- the master's hand, an undo).
UPDATE combatants
SET hp_current = $2, hp_temp = $3, defeated = $4
WHERE id = $1;

-- name: SetCombatantEconomy :exec
-- The turn's economy as an action, or its undo, leaves it.
UPDATE combatants
SET action_used = $2, bonus_action_used = $3, reaction_used = $4, dashed = $5
WHERE id = $1;

-- Pending damage (MR-012, MR-014): the damage of an attack that hit. Every write
-- below runs after the caller locked the open session's row.

-- name: InsertPendingDamage :one
INSERT INTO pending_damages (
    encounter_id, attacker_id, target_id, attack_key, status, critical,
    dice_count, dice_sides, dice_bonus, damage_type, created_at
) VALUES ($1, $2, $3, $4, 'awaiting_roll', $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: GetPendingDamage :one
-- A pending damage by its ID, if it is in the combat.
SELECT * FROM pending_damages
WHERE encounter_id = $1 AND id = $2;

-- name: ListOpenPendingDamages :many
-- What still waits to be rolled or applied in the combat, oldest first.
SELECT * FROM pending_damages
WHERE encounter_id = $1 AND status IN ('awaiting_roll', 'rolled')
ORDER BY created_at, id;

-- name: SetPendingDamageRolled :one
-- The roll of a pending damage: 'rolled' for a player's character (waits for
-- the master), 'applied' for an NPC.
UPDATE pending_damages
SET status = $2, faces = $3, physical = $4, amount = $5, resolved_at = $6
WHERE id = $1
RETURNING *;

-- name: SetPendingDamageStatus :one
-- Applied or discarded by the master, or back to where it was (an undo).
UPDATE pending_damages
SET status = $2, resolved_at = $3
WHERE id = $1
RETURNING *;

-- name: ClearPendingDamageRoll :one
-- An undo of the damage roll: it waits to be rolled again.
UPDATE pending_damages
SET status = 'awaiting_roll', faces = '{}', physical = false, amount = NULL, resolved_at = NULL
WHERE id = $1
RETURNING *;

-- name: DeletePendingDamage :exec
DELETE FROM pending_damages
WHERE id = $1;

-- The combat log and the undo read the session's events (ADR-0007).

-- name: ListEncounterEvents :many
-- A combat's latest events, newest first: when a combat is longer than the
-- limit, it is the oldest lines that fall off the log, never the newest (or the
-- one an undo would take back). The caller reverses them.
SELECT id, seq, kind, actor_user_id, payload, created_at FROM session_events
WHERE encounter_id = $1
ORDER BY seq DESC
LIMIT $2;

-- name: ListRecentSessionEvents :many
-- The session's latest events, newest first (the undo looks for the last action).
SELECT id, seq, kind, encounter_id, payload FROM session_events
WHERE game_session_id = $1
ORDER BY seq DESC
LIMIT $2;
