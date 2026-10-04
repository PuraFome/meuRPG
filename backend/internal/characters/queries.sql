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

-- name: ClearPortraits :execrows
-- The master deleted a gallery image: the NPCs that had it as their portrait
-- lose it (MR-031). The revision goes up, so a stale editor is told. Only the
-- sheets that have it are touched.
UPDATE characters
SET sheet = (sheet #- '{full,portrait_image_id}') #- '{basic,portrait_image_id}',
    revision = revision + 1, updated_at = sqlc.arg(now)
WHERE campaign_id = sqlc.arg(campaign_id)::UUID
  AND (sheet -> 'full' ->> 'portrait_image_id' = sqlc.arg(image_id)::TEXT
       OR sheet -> 'basic' ->> 'portrait_image_id' = sqlc.arg(image_id)::TEXT);

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

-- name: ApprovePendingCharacterOfPlayer :execrows
-- An ordinary invite promotes a pending member (RN-15, Q25): that counts as
-- the master's approval, so their character waiting for approval, if any,
-- is approved with exactly the effect of ApproveCharacter above, found by
-- player instead of by ID. A player has at most one living character
-- (RN-03), so at most one row matches; none matches when they have not
-- created it yet.
UPDATE characters
SET status = 'active'
WHERE campaign_id = sqlc.arg(campaign_id)::UUID AND player_user_id = sqlc.arg(player_user_id)
  AND kind = 'player' AND status = 'pending';

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

-- name: ListVitals :many
-- The vitals of the campaign's living, active player characters (RN-02),
-- oldest first: the party at the table. A character without a
-- character_vitals row has fresh vitals, so the columns from it may be NULL.
-- The sheet comes along because the maximums are derived from it.
SELECT c.id, c.name, c.player_user_id, c.sheet,
       v.hit_points_current, v.hit_points_temporary, v.spell_slots_used,
       v.pact_slots_used, v.hit_dice_used, v.resources_used, v.revision, v.updated_at
FROM characters AS c
LEFT JOIN character_vitals AS v ON v.character_id = c.id
WHERE c.campaign_id = sqlc.arg(campaign_id)::UUID
  AND c.kind = 'player' AND c.status = 'active'
ORDER BY c.created_at, c.id;

-- name: GetVitals :one
-- ListVitals for one character. No row means the character is not a
-- living, active player character of the campaign.
SELECT c.id, c.name, c.player_user_id, c.sheet,
       v.hit_points_current, v.hit_points_temporary, v.spell_slots_used,
       v.pact_slots_used, v.hit_dice_used, v.resources_used, v.revision, v.updated_at
FROM characters AS c
LEFT JOIN character_vitals AS v ON v.character_id = c.id
WHERE c.campaign_id = sqlc.arg(campaign_id)::UUID AND c.id = sqlc.arg(id)
  AND c.kind = 'player' AND c.status = 'active';

-- name: UpsertVitals :one
-- Saves a character's vitals: the first save creates the row with revision
-- 1, and every later one adds 1.
INSERT INTO character_vitals
    (character_id, hit_points_current, hit_points_temporary, spell_slots_used,
     pact_slots_used, hit_dice_used, resources_used, revision, updated_at)
VALUES (
    sqlc.arg(character_id), sqlc.arg(hit_points_current), sqlc.arg(hit_points_temporary),
    sqlc.arg(spell_slots_used)::INT4[], sqlc.arg(pact_slots_used), sqlc.arg(hit_dice_used),
    sqlc.arg(resources_used)::JSONB, 1, sqlc.arg(now)
)
ON CONFLICT (character_id) DO UPDATE SET
    hit_points_current = excluded.hit_points_current,
    hit_points_temporary = excluded.hit_points_temporary,
    spell_slots_used = excluded.spell_slots_used,
    pact_slots_used = excluded.pact_slots_used,
    hit_dice_used = excluded.hit_dice_used,
    resources_used = excluded.resources_used,
    revision = character_vitals.revision + 1,
    updated_at = excluded.updated_at
RETURNING revision, updated_at;

-- name: ListMapCharacters :many
-- Those of the given characters that may stand on a map of the campaign as
-- tokens (package maps): its living characters, the players' (active: not
-- dead, not waiting for approval) and the NPCs. Players' characters first,
-- then NPCs, each group oldest first, as ListCharacters. No sheet, no story.
SELECT id, kind, name, player_user_id FROM characters
WHERE campaign_id = sqlc.arg(campaign_id)::UUID
  AND id = ANY(sqlc.arg(ids)::UUID[])
  AND status = 'active'
ORDER BY kind <> 'player', created_at, id;

-- name: ListCombatParty :many
-- The campaign's living, active player characters, oldest first: the party
-- that fights (package play). The sheet comes along for the numbers a
-- combatant starts with (initiative, speed).
SELECT id, kind, name, player_user_id, sheet FROM characters
WHERE campaign_id = sqlc.arg(campaign_id)::UUID
  AND kind = 'player' AND status = 'active'
ORDER BY created_at, id;

-- name: ListCombatCharacters :many
-- Those of the given characters that may fight in a combat of the campaign:
-- its living characters, players' and NPCs, oldest first (package play).
SELECT id, kind, name, player_user_id, sheet FROM characters
WHERE campaign_id = sqlc.arg(campaign_id)::UUID
  AND id = ANY(sqlc.arg(ids)::UUID[])
  AND status = 'active'
ORDER BY created_at, id;

-- name: ListCharacterNames :many
-- The names of the given characters of the campaign, whatever their kind or
-- status (package progression names the characters of an award, even one that
-- died since).
SELECT id, name FROM characters
WHERE campaign_id = sqlc.arg(campaign_id)::UUID
  AND id = ANY(sqlc.arg(ids)::UUID[]);
