-- The bards asked about an NPC's check outside a combat (migration 00354): one row for each bard, in the order they are asked.

-- name: InsertNpcCheckAsk :one
INSERT INTO npc_check_asks (hold_id, bard_character_id, position, sides, creature_seen, physical, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
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
-- The bard's answer: cut (with the face of the die) or passed. Only a question that is still open takes it (no row: it
-- was closed meanwhile).
UPDATE npc_check_asks
SET state = $2, die_face = $3, physical = $4, answer_key = $5, answered_at = $6
WHERE id = $1 AND state = 'asked'
RETURNING *;

-- name: CloseNpcCheckAsks :many
-- The questions of a held check that still wait close with no answer (the master went on without the bard, or the bard could
-- not answer any more).
UPDATE npc_check_asks
SET state = 'closed', answered_at = $2
WHERE hold_id = $1 AND state = 'asked'
RETURNING *;
