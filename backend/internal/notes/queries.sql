-- name: CountPlayerNotes :one
-- How many notes a player has in a campaign: the 300-note limit counts them.
SELECT count(*)::INT4 AS note_count FROM player_notes
WHERE campaign_id = $1 AND author_user_id = $2;

-- name: InsertPlayerNote :one
-- create_key and create_hash are the idempotency key and the hash of the request (NULL when the
-- call sent no key): two calls with the same key at once make one note, and the loser gets no
-- row and reads the winner's (GetPlayerNoteByCreateKey).
INSERT INTO player_notes (campaign_id, author_user_id, text, scene_point_id, create_key, create_hash, created_at, updated_at)
VALUES (sqlc.arg(campaign_id), sqlc.arg(author_user_id), sqlc.arg(text), sqlc.narg(scene_point_id), sqlc.narg(create_key), sqlc.narg(create_hash), sqlc.arg(now), sqlc.arg(now))
ON CONFLICT (create_key) WHERE create_key IS NOT NULL DO NOTHING
RETURNING *;

-- name: GetPlayerNoteByCreateKey :one
-- The note a CreateNote with this idempotency key made, if any. The key carries the campaign's
-- and the author's IDs.
SELECT * FROM player_notes WHERE create_key = $1;

-- name: ListPlayerNotes :many
-- A player's notes in a campaign, newest write first. The author is in the
-- WHERE of every query below: another person's note is never found.
SELECT * FROM player_notes
WHERE campaign_id = $1 AND author_user_id = $2
ORDER BY updated_at DESC, id;

-- name: GetPlayerNoteForUpdate :one
SELECT * FROM player_notes
WHERE campaign_id = $1 AND author_user_id = $2 AND id = $3
FOR UPDATE;

-- name: UpdatePlayerNote :one
UPDATE player_notes
SET text = sqlc.arg(text), scene_point_id = sqlc.narg(scene_point_id), updated_at = sqlc.arg(now)
WHERE campaign_id = sqlc.arg(campaign_id) AND author_user_id = sqlc.arg(author_user_id) AND id = sqlc.arg(id)
RETURNING *;

-- name: DeletePlayerNote :execrows
DELETE FROM player_notes
WHERE campaign_id = $1 AND author_user_id = $2 AND id = $3;
