-- Reserved characters and their claim links (MR-049). A reserved character is a
-- player character with no owner that no player can see (characters.reserved); a
-- claim link hands it to a player. The tables are in migration 00196.

-- name: InsertClaimLink :one
INSERT INTO claim_links (campaign_id, character_id, token_hash, created_by, created_at, expires_at)
VALUES (
    sqlc.arg(campaign_id)::UUID, sqlc.arg(character_id)::UUID, sqlc.arg(token_hash), sqlc.arg(created_by)::UUID,
    sqlc.arg(now), sqlc.arg(expires_at)
)
RETURNING *;

-- name: RevokeLiveClaimLinks :execrows
-- A new link revokes the character's live one, in the same transaction. A link that
-- ran out is revoked too: the unique index on live links counts it until it is.
UPDATE claim_links
SET revoked_at = sqlc.arg(now)
WHERE character_id = sqlc.arg(character_id)::UUID AND revoked_at IS NULL AND used_at IS NULL;

-- name: GetClaimLinkByTokenHashForUpdate :one
-- The link a token opens, locked until the transaction ends: two claims of one link,
-- or a claim and a revoke, wait for each other and the second finds the first's result.
SELECT * FROM claim_links WHERE token_hash = sqlc.arg(token_hash) FOR UPDATE;

-- name: GetClaimLinkByTokenHash :one
SELECT * FROM claim_links WHERE token_hash = sqlc.arg(token_hash);

-- name: GetLatestClaimLinkForUpdate :one
-- The character's latest link, locked: RevokeClaimLink and the claim race for it.
SELECT * FROM claim_links
WHERE campaign_id = sqlc.arg(campaign_id)::UUID AND character_id = sqlc.arg(character_id)::UUID
ORDER BY created_at DESC, id DESC
LIMIT 1
FOR UPDATE;

-- name: MarkClaimLinkUsed :execrows
-- Spends a link that still works: the WHERE clause repeats what the caller checked
-- under the row lock, so a bug there cannot spend a revoked or expired link.
UPDATE claim_links
SET used_at = sqlc.arg(now)::TIMESTAMPTZ, used_by = sqlc.arg(used_by)::UUID
WHERE id = sqlc.arg(id)::UUID AND used_at IS NULL AND revoked_at IS NULL AND expires_at > sqlc.arg(now)::TIMESTAMPTZ;

-- name: RevokeClaimLinkByID :execrows
UPDATE claim_links
SET revoked_at = sqlc.arg(now)::TIMESTAMPTZ
WHERE id = sqlc.arg(id)::UUID AND used_at IS NULL AND revoked_at IS NULL;

-- name: ListLatestClaimLinks :many
-- The latest link of each character of the campaign that has one, for the master's list.
SELECT DISTINCT ON (character_id) character_id, expires_at, revoked_at, used_at, used_by
FROM claim_links
WHERE campaign_id = sqlc.arg(campaign_id)::UUID
ORDER BY character_id, created_at DESC, id DESC;

-- name: UpsertClaimSignIn :exec
-- The link a person signed in with (sign-in intent character_claim): one row per person.
INSERT INTO claim_sign_ins (user_id, token_hash, expires_at)
VALUES (sqlc.arg(user_id)::UUID, sqlc.arg(token_hash), sqlc.arg(expires_at))
ON CONFLICT (user_id) DO UPDATE SET token_hash = excluded.token_hash, expires_at = excluded.expires_at;

-- name: GetClaimSignIn :one
SELECT token_hash FROM claim_sign_ins
WHERE user_id = sqlc.arg(user_id)::UUID AND expires_at > sqlc.arg(now)::TIMESTAMPTZ;

-- name: DeleteClaimSignIn :exec
DELETE FROM claim_sign_ins WHERE user_id = sqlc.arg(user_id)::UUID;

-- name: ClaimReservedCharacter :one
-- A player takes a reserved character. Like approval and death, a change of owner does
-- not change the revision or updated_at: the sheet and the story are as the master left them.
UPDATE characters
SET reserved = false, player_user_id = sqlc.arg(player_user_id)::UUID, claimed_at = sqlc.arg(now)::TIMESTAMPTZ
WHERE campaign_id = sqlc.arg(campaign_id)::UUID AND id = sqlc.arg(id)::UUID AND reserved
RETURNING *;

-- name: ReturnCharacterToReserve :one
-- The master takes a claimed character back: no owner, reserved again. Only a living
-- player character a claim brought to its player (claimed_at) goes back.
UPDATE characters
SET reserved = true, player_user_id = NULL, claimed_at = NULL
WHERE campaign_id = sqlc.arg(campaign_id)::UUID AND id = sqlc.arg(id)::UUID
  AND kind = 'player' AND NOT reserved AND claimed_at IS NOT NULL AND status = 'active'
RETURNING *;

-- name: DeleteReservedCharacter :execrows
-- Only a reserved character is ever deleted here: it never had an owner (RN-03 keeps
-- every other one).
DELETE FROM characters
WHERE campaign_id = sqlc.arg(campaign_id)::UUID AND id = sqlc.arg(id)::UUID AND reserved;

-- name: GetCharacterNameAndOwner :one
SELECT id, name, player_user_id FROM characters
WHERE campaign_id = sqlc.arg(campaign_id)::UUID AND id = sqlc.arg(id)::UUID;
