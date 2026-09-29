-- name: InsertCampaign :one
INSERT INTO campaigns (name, xp_mode, created_by)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetCampaign :one
SELECT * FROM campaigns WHERE id = $1;

-- name: ListCampaignsOfUser :many
-- Newest first. The index on campaign_members (user_id) finds the rows.
SELECT sqlc.embed(c), m.role
FROM campaign_members AS m
JOIN campaigns AS c ON c.id = m.campaign_id
WHERE m.user_id = $1
ORDER BY c.created_at DESC, c.id;

-- name: InsertMember :one
INSERT INTO campaign_members (campaign_id, user_id, role)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetMemberRole :one
-- The query behind every authorization check (package authz): one read of
-- the primary key.
SELECT role FROM campaign_members WHERE campaign_id = $1 AND user_id = $2;

-- name: ListMembers :many
-- The master first, then the players in the order they joined.
SELECT * FROM campaign_members
WHERE campaign_id = $1
ORDER BY role = 'master' DESC, joined_at, user_id;

-- name: InsertInvite :one
INSERT INTO campaign_invites
    (campaign_id, token_hash, created_by, max_uses, created_at, expires_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListInvites :many
-- Newest first. Invites that expired more than 30 days ago are gone (TTL).
SELECT * FROM campaign_invites
WHERE campaign_id = $1
ORDER BY created_at DESC, id;

-- name: RevokeInvite :one
-- Revoking twice keeps the first revoked_at, so RevokeInvite is idempotent.
-- An invite of another campaign matches no row, which reads as "not found".
UPDATE campaign_invites
SET revoked_at = COALESCE(revoked_at, sqlc.arg(now))
WHERE campaign_id = $1 AND id = $2
RETURNING *;

-- name: GetInviteByTokenHashForUpdate :one
-- FOR UPDATE locks the invite until the transaction ends. A second
-- AcceptInvite for the same invite waits here, instead of both reading the
-- same use_count, and then sees the first one's result.
SELECT * FROM campaign_invites WHERE token_hash = $1 FOR UPDATE;

-- name: IncrementInviteUses :execrows
-- The WHERE clause repeats the rules, so an invite is never used more than
-- max_uses times, after it expires or after it is revoked, even if the Go
-- checks before it had a bug.
UPDATE campaign_invites
SET use_count = use_count + 1
WHERE id = $1 AND use_count < max_uses AND revoked_at IS NULL AND expires_at > sqlc.arg(now);
