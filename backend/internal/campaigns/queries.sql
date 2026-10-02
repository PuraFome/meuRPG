-- name: InsertCampaign :one
INSERT INTO campaigns (name, xp_mode, created_by)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetCampaign :one
SELECT * FROM campaigns WHERE id = $1;

-- name: ListCampaignsOfUser :many
-- Newest first. The index on campaign_members (user_id) finds the rows.
-- Pending memberships (RN-15) come too: the handler shows only the name.
SELECT sqlc.embed(c), m.role, m.status
FROM campaign_members AS m
JOIN campaigns AS c ON c.id = m.campaign_id
WHERE m.user_id = $1
ORDER BY c.created_at DESC, c.id;

-- name: InsertMember :one
-- pending_expires_at is NULL, except for a pending member, who has no
-- character yet: joined time + 30 days (RN-15, migration 00034).
INSERT INTO campaign_members (campaign_id, user_id, role, status, pending_expires_at)
VALUES ($1, $2, $3, $4, sqlc.narg(pending_expires_at))
RETURNING *;

-- name: GetMembership :one
-- The query behind every authorization check (package authz): one read of
-- the primary key.
SELECT role, status FROM campaign_members WHERE campaign_id = $1 AND user_id = $2;

-- name: ListMembers :many
-- The master first, then the players in the order they joined. Pending
-- members (RN-15) are not members yet, so they are left out.
SELECT * FROM campaign_members
WHERE campaign_id = $1 AND status = 'active'
ORDER BY role = 'master' DESC, joined_at, user_id;

-- name: ActivatePendingMember :execrows
-- The master approved the pending member's character (RN-15): the
-- membership becomes an ordinary one. An active membership matches no row
-- and stays as it is.
UPDATE campaign_members
SET status = 'active', pending_expires_at = NULL
WHERE campaign_id = $1 AND user_id = $2 AND status = 'pending';

-- name: ClearPendingExpiry :execrows
-- A pending member created their character (RN-15): the master decides on
-- it now, so the 30-day deadline for a pending member without a character
-- no longer applies.
UPDATE campaign_members
SET pending_expires_at = NULL
WHERE campaign_id = $1 AND user_id = $2 AND status = 'pending';

-- name: ListPendingMembersWithoutCharacter :many
-- The pending members who have not created a character, in the order they
-- joined. pending_expires_at is set exactly for them (migration 00034).
SELECT user_id, joined_at, pending_expires_at FROM campaign_members
WHERE campaign_id = $1 AND status = 'pending' AND pending_expires_at IS NOT NULL
ORDER BY joined_at, user_id;

-- name: DeletePendingMemberWithoutCharacter :execrows
-- The master removed a pending member who has no character. The WHERE
-- clause matches only that: an active member, or a pending member whose
-- character waits for approval (pending_expires_at is NULL), is never
-- deleted here; that one goes through RejectCharacter.
DELETE FROM campaign_members
WHERE campaign_id = $1 AND user_id = $2 AND status = 'pending' AND pending_expires_at IS NOT NULL;

-- name: DeletePendingMember :execrows
-- The master rejected the pending member's character (RN-15): the pending
-- membership goes with it. status = 'pending' in the WHERE clause means an
-- active membership is never deleted here.
DELETE FROM campaign_members
WHERE campaign_id = $1 AND user_id = $2 AND status = 'pending';

-- name: InsertInvite :one
INSERT INTO campaign_invites
    (campaign_id, token_hash, created_by, max_uses, created_at, expires_at, requires_approval)
VALUES ($1, $2, $3, $4, $5, $6, $7)
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

-- name: GetCampaignDocument :one
-- The campaign's document (MR-018). No row means it was never saved: an
-- empty document at revision 0.
SELECT * FROM campaign_documents WHERE campaign_id = $1;

-- name: InsertCampaignDocument :one
-- The first save of a campaign's document, at revision 1. When someone else
-- saved first, ON CONFLICT DO NOTHING returns no row: the caller's revision
-- (0) is stale.
INSERT INTO campaign_documents (campaign_id, body, revision, updated_at, updated_by)
VALUES (sqlc.arg(campaign_id), sqlc.arg(body), 1, sqlc.arg(updated_at), sqlc.arg(updated_by))
ON CONFLICT (campaign_id) DO NOTHING
RETURNING *;

-- name: UpdateCampaignDocument :one
-- Every later save: it replaces the body only while the stored revision is
-- the one the caller read (compare-and-swap), and raises it by one. No row
-- means the revision is stale.
UPDATE campaign_documents
SET body = sqlc.arg(body),
    revision = revision + 1,
    updated_at = sqlc.arg(updated_at),
    updated_by = sqlc.arg(updated_by)
WHERE campaign_id = sqlc.arg(campaign_id) AND revision = sqlc.arg(expected_revision)
RETURNING *;
