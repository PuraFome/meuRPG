-- Every query names the campaign next to the character: a character ID of
-- another campaign matches no row, which the handlers answer as "not found".
-- campaign_id is nullable in the table (a deleted campaign sets it to NULL),
-- so the argument is cast to UUID to make it a plain string in Go.

-- name: InsertCharacter :one
-- status is 'active', or 'pending' for a character created by a pending
-- member (RN-15, MR-024).
INSERT INTO characters
    (campaign_id, kind, player_user_id, master_user_id, status, name, sheet, story, created_at, updated_at)
VALUES (
    sqlc.arg(campaign_id)::UUID, sqlc.arg(kind), sqlc.narg(player_user_id), sqlc.narg(master_user_id),
    sqlc.arg(status), sqlc.arg(name), sqlc.arg(sheet), sqlc.arg(story), sqlc.arg(now), sqlc.arg(now)
)
RETURNING *;

-- name: GetCharacter :one
SELECT * FROM characters
WHERE campaign_id = sqlc.arg(campaign_id)::UUID AND id = sqlc.arg(id);

-- name: GetCharacterForUpdate :one
-- FOR UPDATE locks the row until the transaction ends, so two edits of the
-- same character wait for each other instead of both reading the same
-- revision.
SELECT * FROM characters
WHERE campaign_id = sqlc.arg(campaign_id)::UUID AND id = sqlc.arg(id)
FOR UPDATE;

-- name: GetLivingPlayerCharacterID :one
-- The player's living character in the campaign, if any (RN-03). The
-- partial unique index characters_one_living_player_character allows at
-- most one.
SELECT id FROM characters
WHERE campaign_id = sqlc.arg(campaign_id)::UUID
  AND player_user_id = sqlc.arg(player_user_id)::UUID
  AND kind = 'player' AND status <> 'dead'
LIMIT 1;

-- name: ListCharacters :many
-- Without player_user_id, every character of the campaign (the master's
-- list); with it, only that player's characters. With status, only the
-- characters in that status (a pending member sees only their pending
-- character, RN-15). Players' characters come first, then NPCs, each group
-- oldest first. The story is left out: a list does not show it.
SELECT id, kind, status, name, player_user_id, sheet, sheet_locked_at, created_at
FROM characters
WHERE campaign_id = sqlc.arg(campaign_id)::UUID
  AND (
      sqlc.narg(player_user_id)::UUID IS NULL
      OR (kind = 'player' AND player_user_id = sqlc.narg(player_user_id)::UUID)
  )
  AND (sqlc.narg(status)::TEXT IS NULL OR status = sqlc.narg(status)::TEXT)
ORDER BY kind <> 'player', created_at, id;

-- name: UpdateCharacterSheet :one
-- revision in the WHERE clause is a second guard: the handler already
-- compared it under FOR UPDATE, so no row here means a stale revision.
UPDATE characters
SET name = sqlc.arg(name), sheet = sqlc.arg(sheet), revision = revision + 1, updated_at = sqlc.arg(now)
WHERE campaign_id = sqlc.arg(campaign_id)::UUID AND id = sqlc.arg(id) AND revision = sqlc.arg(revision)
RETURNING *;

-- name: UpdateCharacterStory :one
-- As UpdateCharacterSheet: the story and the sheet share one revision.
UPDATE characters
SET story = sqlc.arg(story), revision = revision + 1, updated_at = sqlc.arg(now)
WHERE campaign_id = sqlc.arg(campaign_id)::UUID AND id = sqlc.arg(id) AND revision = sqlc.arg(revision)
RETURNING *;

-- name: SetStoryEditing :one
-- The master's permission for the player to edit the story. It changes
-- neither the revision nor updated_at, which follow the content.
UPDATE characters
SET story_editing_allowed = sqlc.arg(allowed)
WHERE campaign_id = sqlc.arg(campaign_id)::UUID AND id = sqlc.arg(id)
RETURNING *;

-- name: MarkCharacterDead :one
-- Marking a dead character again keeps the first died_at, so the call is
-- idempotent. It changes neither the revision nor updated_at.
UPDATE characters
SET status = 'dead', died_at = COALESCE(died_at, sqlc.arg(now)::TIMESTAMPTZ)
WHERE campaign_id = sqlc.arg(campaign_id)::UUID AND id = sqlc.arg(id)
RETURNING *;

-- name: ApproveCharacter :one
-- The master approved a pending character (RN-15, MR-024): it becomes an
-- ordinary living character, a draft until the next game session starts.
-- Like death, approval changes neither the revision nor updated_at.
UPDATE characters
SET status = 'active'
WHERE campaign_id = sqlc.arg(campaign_id)::UUID AND id = sqlc.arg(id) AND status = 'pending'
RETURNING *;

-- name: DeletePendingCharacter :execrows
-- The master rejected a pending character (RN-15, MR-024): it never became
-- part of the campaign, so it is deleted, story and all; the master's notes
-- about it go too (character_master_notes cascades). status = 'pending' in
-- the WHERE clause means no other character is ever deleted here: outside
-- account deletion, characters change status, never row (RN-03).
DELETE FROM characters
WHERE campaign_id = sqlc.arg(campaign_id)::UUID AND id = sqlc.arg(id) AND status = 'pending';

-- name: LockSheets :execrows
-- RN-01: when a game session starts, the sheets of the campaign's living
-- player characters that are still drafts lock. A character waiting for
-- approval (MR-024) does not lock yet, and NPCs never lock.
UPDATE characters
SET sheet_locked_at = sqlc.arg(now)::TIMESTAMPTZ
WHERE campaign_id = sqlc.arg(campaign_id)::UUID
  AND kind = 'player' AND status = 'active' AND sheet_locked_at IS NULL;

-- name: EndStoryEditing :execrows
-- When a game session starts, every permission to edit a story ends.
UPDATE characters
SET story_editing_allowed = false
WHERE campaign_id = sqlc.arg(campaign_id)::UUID AND story_editing_allowed;

-- name: CharacterIsInCampaign :one
SELECT EXISTS (
    SELECT 1 FROM characters
    WHERE campaign_id = sqlc.arg(campaign_id)::UUID AND id = sqlc.arg(id)
);

-- name: GetMasterNotes :one
SELECT notes, updated_at FROM character_master_notes
WHERE campaign_id = $1 AND character_id = $2;

-- name: UpsertMasterNotes :one
INSERT INTO character_master_notes (campaign_id, character_id, notes, updated_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (campaign_id, character_id)
DO UPDATE SET notes = excluded.notes, updated_at = excluded.updated_at
RETURNING notes, updated_at;

-- name: DeleteMasterNotes :exec
-- Empty notes are not stored (RN-11 keeps only what is needed).
DELETE FROM character_master_notes
WHERE campaign_id = $1 AND character_id = $2;
