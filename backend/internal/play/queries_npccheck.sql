-- The bards asked about an NPC's check outside a combat: one row for each bard, in the order they are asked.

-- name: InsertNpcCheckAsk :one
INSERT INTO npc_check_asks (hold_id, bard_character_id, position, sides, creature_seen, physical, created_at, map_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: ListNpcCheckAsks :many
-- The questions of one held check, in the order the bards are asked.
SELECT * FROM npc_check_asks WHERE hold_id = $1 ORDER BY position;

-- name: GetNpcCheckAskByAnswerKey :one
-- The question an answer's key closed, in the campaign: a retry of the answer finds it.
SELECT a.* FROM npc_check_asks AS a
JOIN inspiration_holds AS h ON h.id = a.hold_id
WHERE h.campaign_id = $1 AND a.answer_key = $2;

-- name: AnswerNpcCheckAsk :one
-- The bard's answer: cut (with the face of the die) or passed, with the key and the hash of the request that gave it (a retry
-- with the same key and another body is told apart). Only a question that is still open takes it (no row: it was closed
-- meanwhile).
UPDATE npc_check_asks
SET state = $2, die_face = $3, physical = $4, answer_key = $5, answered_at = $6, answer_hash = $7
WHERE id = $1 AND state = 'asked'
RETURNING *;

-- name: CloseNpcCheckAsks :exec
-- The questions of a held check that still wait close with no answer (the master went on without the bard, or the bard could
-- not answer any more).
UPDATE npc_check_asks
SET state = 'closed', answered_at = $2
WHERE hold_id = $1 AND state = 'asked';

-- name: ListOpenNpcCheckHolds :many
-- The master's checks of the campaign that still wait for the bards: a combat that opens and a session that ends settle them.
SELECT * FROM inspiration_holds WHERE campaign_id = $1 AND kind = 'npc_check' AND answer_key IS NULL ORDER BY created_at, id;

-- name: GetLatestNpcCheckEvent :one
-- The event of the check rolled last in the session, held or settled at once: the master's read of the check answers with it until the
-- next check is rolled (GetNpcCheck). The roll of a check is its npc_check_held event, or the npc_check_resolved of a check that never
-- had a hold; the resolved event of a held check is no roll (it may come after the next roll's event), the hold tells how it stands.
SELECT * FROM session_events
WHERE game_session_id = $1
  AND (kind = 'npc_check_held' OR (kind = 'npc_check_resolved' AND coalesce(payload->>'hold_id', '') = ''))
ORDER BY seq DESC
LIMIT 1;

-- name: ListNpcCheckHolds :many
-- Every NPC check hold of the campaign, settled or not: the session that ends deletes them all.
SELECT * FROM inspiration_holds WHERE campaign_id = $1 AND kind = 'npc_check' ORDER BY created_at, id;

-- name: PassNpcCheckAsk :one
-- The bard's question is let pass with no answer of the bard: its character is no longer in play (reserve, dead, deleted, no player).
UPDATE npc_check_asks
SET state = 'passed', answered_at = $2
WHERE id = $1 AND state = 'asked'
RETURNING *;
