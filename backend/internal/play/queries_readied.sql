-- The readied actions of a combat (migration 00330).

-- name: InsertReadiedAction :one
INSERT INTO readied_actions (encounter_id, combatant_id, round, trigger_text, kind, action_key, action_name_pt, move_ft, target_id, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: ListWaitingReadiedActions :many
-- The readied actions of a combat that wait for their trigger, oldest first.
SELECT * FROM readied_actions
WHERE encounter_id = $1 AND status = 'waiting'
ORDER BY created_at, id;

-- name: GetWaitingReadiedAction :one
SELECT * FROM readied_actions
WHERE id = $1 AND encounter_id = $2 AND status = 'waiting';

-- name: GetWaitingReadiedOf :one
-- The readied action a combatant holds, if any.
SELECT * FROM readied_actions
WHERE combatant_id = $1 AND status = 'waiting';

-- name: SetReadiedActionStatus :exec
-- A readied action that waited ends.
UPDATE readied_actions
SET status = $2
WHERE id = $1 AND status = 'waiting';

-- name: DeleteReadiedAction :exec
-- The undo of a Ready takes its row away.
DELETE FROM readied_actions WHERE id = $1;

-- name: DeleteReadiedActionsOfEncounter :exec
-- The readied actions go with the combat that ends.
DELETE FROM readied_actions WHERE encounter_id = $1;

-- name: ReopenReadiedAction :exec
-- The undo of a readied action's use or pass: it waits again.
UPDATE readied_actions
SET status = 'waiting'
WHERE id = $1 AND status IN ('fired', 'ignored');

-- name: GetReadiedAction :one
-- A readied action of a combat, whatever its status.
SELECT * FROM readied_actions
WHERE id = $1 AND encounter_id = $2;

-- name: DeleteUnansweredReadiedWindow :execrows
-- The undo of FireReadied: the READIED window it opened goes, unless it was answered (the answer is undone first).
DELETE FROM reaction_windows
WHERE id = sqlc.arg(id) AND encounter_id = sqlc.arg(encounter_id) AND kind = 'readied' AND status <> 'answered';
