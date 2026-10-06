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

-- name: PortraitInUse :one
-- Whether any NPC of the campaign has the gallery image as its portrait (the
-- maps module copies a fog map's image when something else uses it, MR-036).
SELECT EXISTS (
    SELECT 1 FROM characters
    WHERE campaign_id = sqlc.arg(campaign_id)::UUID
      AND (sheet -> 'full' ->> 'portrait_image_id' = sqlc.arg(image_id)::TEXT
           OR sheet -> 'basic' ->> 'portrait_image_id' = sqlc.arg(image_id)::TEXT)
);

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
-- The sheet comes along because the maximums are derived from it. The Wild Shape
-- form and the familiar's sight come along too (MR-037, MR-036).
SELECT c.id, c.name, c.player_user_id, c.sheet,
       v.hit_points_current, v.hit_points_temporary, v.spell_slots_used,
       v.pact_slots_used, v.hit_dice_used, v.resources_used, v.revision, v.updated_at,
       ws.beast AS wild_shape_beast, ws.hp AS wild_shape_hp, v.familiar_sight_creature_id, v.familiar_sight_in_combat,
       v.familiar_sight_conditions
FROM characters AS c
LEFT JOIN character_vitals AS v ON v.character_id = c.id
LEFT JOIN character_wild_shapes AS ws ON ws.character_id = c.id
WHERE c.campaign_id = sqlc.arg(campaign_id)::UUID
  AND c.kind = 'player' AND c.status = 'active'
ORDER BY c.created_at, c.id;

-- name: GetVitals :one
-- ListVitals for one character. No row means the character is not a
-- living, active player character of the campaign.
SELECT c.id, c.name, c.player_user_id, c.sheet,
       v.hit_points_current, v.hit_points_temporary, v.spell_slots_used,
       v.pact_slots_used, v.hit_dice_used, v.resources_used, v.revision, v.updated_at,
       ws.beast AS wild_shape_beast, ws.hp AS wild_shape_hp, v.familiar_sight_creature_id, v.familiar_sight_in_combat,
       v.familiar_sight_conditions
FROM characters AS c
LEFT JOIN character_vitals AS v ON v.character_id = c.id
LEFT JOIN character_wild_shapes AS ws ON ws.character_id = c.id
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
-- The beast of a druid in Wild Shape comes along (MR-037): the combatant starts
-- with the beast's speed and size.
SELECT c.id, c.kind, c.name, c.player_user_id, c.sheet, ws.beast AS wild_shape_beast
FROM characters AS c
LEFT JOIN character_wild_shapes AS ws ON ws.character_id = c.id
WHERE c.campaign_id = sqlc.arg(campaign_id)::UUID
  AND c.kind = 'player' AND c.status = 'active'
ORDER BY c.created_at, c.id;

-- name: ListCombatCharacters :many
-- Those of the given characters that may fight in a combat of the campaign:
-- its living characters, players' and NPCs, oldest first (package play).
-- The beast of a druid in Wild Shape comes along, as in ListCombatParty.
SELECT c.id, c.kind, c.name, c.player_user_id, c.sheet, ws.beast AS wild_shape_beast
FROM characters AS c
LEFT JOIN character_wild_shapes AS ws ON ws.character_id = c.id
WHERE c.campaign_id = sqlc.arg(campaign_id)::UUID
  AND c.id = ANY(sqlc.arg(ids)::UUID[])
  AND c.status = 'active'
ORDER BY c.created_at, c.id;

-- name: ListSessionCharacters :many
-- Those of the given characters of the campaign, whatever their status: the
-- session summary names a character that died or left during the session
-- (package play).
SELECT id, kind, name, player_user_id FROM characters
WHERE campaign_id = sqlc.arg(campaign_id)::UUID
  AND id = ANY(sqlc.arg(ids)::UUID[]);

-- name: ListCharacterNames :many
-- The names of the given characters of the campaign, whatever their kind or
-- status (package progression names the characters of an award, even one that
-- died since).
SELECT id, name FROM characters
WHERE campaign_id = sqlc.arg(campaign_id)::UUID
  AND id = ANY(sqlc.arg(ids)::UUID[]);

-- name: GetLevelUpRoll :one
-- The hit die the server rolled for the character's next level (MR-040): one
-- per character and target total level (not per class: a multiclass
-- character cannot roll twice for a level), kept until the level-up uses it.
SELECT class_key, die, value FROM character_level_up_rolls
WHERE character_id = sqlc.arg(character_id)::UUID AND to_level = sqlc.arg(to_level);

-- name: InsertLevelUpRoll :execrows
-- Keeps a roll. A second roll for the same level does nothing (0 rows), so the
-- first one stands: asking again never rerolls.
INSERT INTO character_level_up_rolls (character_id, class_key, to_level, die, value, created_at)
VALUES (sqlc.arg(character_id)::UUID, sqlc.arg(class_key), sqlc.arg(to_level), sqlc.arg(die), sqlc.arg(value), sqlc.arg(now))
ON CONFLICT (character_id, to_level) DO NOTHING;

-- name: DeleteLevelUpRoll :exec
-- The level-up used the roll; its value goes on in character_level_ups.
DELETE FROM character_level_up_rolls
WHERE character_id = sqlc.arg(character_id)::UUID AND to_level = sqlc.arg(to_level);

-- name: InsertLevelUp :one
-- The record of a guided level-up (MR-040), written in the transaction that
-- saves the sheet.
INSERT INTO character_level_ups
    (campaign_id, character_id, class_key, from_level, to_level, hp_method, hp_value, choices, created_at)
VALUES (
    sqlc.arg(campaign_id)::UUID, sqlc.arg(character_id)::UUID, sqlc.arg(class_key), sqlc.arg(from_level),
    sqlc.arg(to_level), sqlc.arg(hp_method), sqlc.arg(hp_value), sqlc.arg(choices), sqlc.arg(created_at)
)
RETURNING *;

-- name: ListLevelUps :many
-- A campaign's level-ups, newest first, for the master's "O que mudou". With
-- character_id, only that character's. The character's name is the current
-- one, and its player the character's own.
SELECT l.id, l.character_id, l.class_key, l.from_level, l.to_level, l.choices, l.created_at,
       c.name AS character_name, c.player_user_id
FROM character_level_ups l
JOIN characters c ON c.id = l.character_id
WHERE l.campaign_id = sqlc.arg(campaign_id)::UUID
  AND (sqlc.narg(character_id)::UUID IS NULL OR l.character_id = sqlc.narg(character_id)::UUID)
  AND (sqlc.narg(before_created_at)::TIMESTAMPTZ IS NULL
       OR (l.created_at, l.id) < (sqlc.narg(before_created_at)::TIMESTAMPTZ, sqlc.narg(before_id)::UUID))
ORDER BY l.created_at DESC, l.id DESC
LIMIT sqlc.arg(max_rows);

-- A character's creatures (MR-037, Etapa 9). A creature is "live" until it is
-- dismissed; a dismissed one is kept (dismissed_at, dismissed_reason) so an
-- undo can bring it back.

-- name: InsertCharacterCreature :one
INSERT INTO character_creatures
    (campaign_id, character_id, monster_key, name, source, attack, summon_group_id, concentration_cast_id, hp_current, hp_max, created_at)
VALUES (
    sqlc.arg(campaign_id)::UUID, sqlc.arg(character_id)::UUID, sqlc.arg(monster_key), sqlc.arg(name), sqlc.arg(source),
    sqlc.arg(attack), sqlc.arg(summon_group_id)::UUID, sqlc.narg(concentration_cast_id)::UUID, sqlc.arg(hp_current), sqlc.arg(hp_max),
    sqlc.arg(created_at)
)
RETURNING *;

-- name: GetCharacterCreature :one
-- One creature of the campaign, live or not, with its owner's player.
SELECT sqlc.embed(cc), c.player_user_id
FROM character_creatures cc
JOIN characters c ON c.id = cc.character_id
WHERE cc.campaign_id = sqlc.arg(campaign_id)::UUID AND cc.id = sqlc.arg(id)::UUID;

-- name: GetCharacterCreatureForUpdate :one
-- The same, locking the creature's row until the transaction ends.
SELECT sqlc.embed(cc), c.player_user_id
FROM character_creatures cc
JOIN characters c ON c.id = cc.character_id
WHERE cc.campaign_id = sqlc.arg(campaign_id)::UUID AND cc.id = sqlc.arg(id)::UUID
FOR UPDATE OF cc;

-- name: ListLiveCreaturesOfCharacters :many
-- The live creatures of the given characters of the campaign, oldest first.
SELECT sqlc.embed(cc), c.player_user_id
FROM character_creatures cc
JOIN characters c ON c.id = cc.character_id
WHERE cc.campaign_id = sqlc.arg(campaign_id)::UUID
  AND cc.character_id = ANY(sqlc.arg(character_ids)::UUID[])
  AND cc.dismissed_at IS NULL
ORDER BY cc.created_at, cc.id;

-- name: ListCreaturesByIDs :many
-- The creatures of the campaign with the given IDs, live or not.
SELECT sqlc.embed(cc), c.player_user_id
FROM character_creatures cc
JOIN characters c ON c.id = cc.character_id
WHERE cc.campaign_id = sqlc.arg(campaign_id)::UUID AND cc.id = ANY(sqlc.arg(ids)::UUID[])
ORDER BY cc.created_at, cc.id;

-- name: ListLiveCreaturesOnConcentration :many
-- The live creatures of the character that last only while it concentrates.
SELECT sqlc.embed(cc), c.player_user_id
FROM character_creatures cc
JOIN characters c ON c.id = cc.character_id
WHERE cc.campaign_id = sqlc.arg(campaign_id)::UUID
  AND cc.character_id = sqlc.arg(character_id)::UUID
  AND cc.concentration_cast_id IS NOT NULL
  AND cc.dismissed_at IS NULL
ORDER BY cc.created_at, cc.id;

-- name: ListLiveFamiliars :many
-- The character's live familiars: one at a time (the SRD), so a new one
-- dismisses these.
SELECT sqlc.embed(cc), c.player_user_id
FROM character_creatures cc
JOIN characters c ON c.id = cc.character_id
WHERE cc.campaign_id = sqlc.arg(campaign_id)::UUID
  AND cc.character_id = sqlc.arg(character_id)::UUID
  AND cc.source = 'familiar'
  AND cc.dismissed_at IS NULL
ORDER BY cc.created_at, cc.id;

-- name: CountLiveCreaturesOfCharacter :one
SELECT count(*)::INT4 FROM character_creatures
WHERE campaign_id = sqlc.arg(campaign_id)::UUID AND character_id = sqlc.arg(character_id)::UUID AND dismissed_at IS NULL;

-- name: DismissCreatures :many
-- Sends the live ones of these creatures away, with the reason; a creature
-- dismissed because it was defeated is at 0 hit points. Returns the ones it
-- dismissed.
UPDATE character_creatures
SET dismissed_at = sqlc.arg(at)::TIMESTAMPTZ, dismissed_reason = sqlc.arg(reason)::TEXT,
    hp_current = CASE WHEN sqlc.arg(reason)::TEXT = 'defeated' THEN 0 ELSE hp_current END
WHERE campaign_id = sqlc.arg(campaign_id)::UUID
  AND id = ANY(sqlc.arg(ids)::UUID[])
  AND dismissed_at IS NULL
RETURNING id;

-- name: ReviveCreatures :many
-- Brings back the creatures dismissed for this reason (an undo): the ones
-- defeated that an undo healed, the ones a master's undo of a casting or of the
-- end of a concentration gives back. Returns the ones it revived.
UPDATE character_creatures
SET dismissed_at = NULL, dismissed_reason = NULL
WHERE campaign_id = sqlc.arg(campaign_id)::UUID
  AND id = ANY(sqlc.arg(ids)::UUID[])
  AND dismissed_reason = sqlc.arg(reason)::TEXT
RETURNING id;

-- name: DeleteCreatures :exec
-- Takes the creatures a casting made away for good (the master's undo of it).
DELETE FROM character_creatures
WHERE campaign_id = sqlc.arg(campaign_id)::UUID AND id = ANY(sqlc.arg(ids)::UUID[]);

-- name: SetCharacterCreatureName :exec
UPDATE character_creatures SET name = sqlc.arg(name)
WHERE campaign_id = sqlc.arg(campaign_id)::UUID AND id = sqlc.arg(id)::UUID;

-- name: SetCharacterCreatureHitPoints :exec
-- The creature's hit points: the master's correction, or a combat's write-back.
UPDATE character_creatures SET hp_current = sqlc.arg(hp_current)
WHERE campaign_id = sqlc.arg(campaign_id)::UUID AND id = sqlc.arg(id)::UUID;

-- name: SetWildShape :exec
-- A druid's Wild Shape form (MR-037): the beast and its current hit points
-- (1 or more). The caller also bumps the vitals' revision (TouchVitals).
INSERT INTO character_wild_shapes (character_id, beast, hp, updated_at)
VALUES (sqlc.arg(character_id), sqlc.arg(beast), sqlc.arg(hp), sqlc.arg(now))
ON CONFLICT (character_id) DO UPDATE SET beast = excluded.beast, hp = excluded.hp, updated_at = excluded.updated_at;

-- name: ClearWildShape :exec
-- The druid is itself again.
DELETE FROM character_wild_shapes WHERE character_id = $1;

-- name: TouchVitals :one
-- Bumps a character's vitals revision for a change made on a table of its own (the
-- Wild Shape form): the first write creates the vitals row, as UpsertVitals does
-- (hit_points_current is only for that insert).
INSERT INTO character_vitals (character_id, hit_points_current, revision, updated_at)
VALUES (sqlc.arg(character_id), sqlc.arg(hit_points_current), 1, sqlc.arg(now))
ON CONFLICT (character_id) DO UPDATE SET
    revision = character_vitals.revision + 1,
    updated_at = excluded.updated_at
RETURNING revision, updated_at;

-- name: SetFamiliarSight :one
-- "Ver pelos olhos do familiar" (MR-036): the familiar the player looks through
-- (NULL for none), whether it started in a combat and the conditions it gave the
-- combatant. The first write creates the vitals row, as TouchVitals does.
INSERT INTO character_vitals (character_id, hit_points_current, familiar_sight_creature_id, familiar_sight_in_combat, familiar_sight_conditions, revision, updated_at)
VALUES (sqlc.arg(character_id), sqlc.arg(hit_points_current), sqlc.narg(creature_id), sqlc.arg(in_combat), sqlc.arg(conditions)::TEXT[], 1, sqlc.arg(now))
ON CONFLICT (character_id) DO UPDATE SET
    familiar_sight_creature_id = excluded.familiar_sight_creature_id,
    familiar_sight_in_combat = excluded.familiar_sight_in_combat,
    familiar_sight_conditions = excluded.familiar_sight_conditions,
    revision = character_vitals.revision + 1,
    updated_at = excluded.updated_at
RETURNING revision, updated_at;

-- name: ListPartyVision :many
-- The campaign's living, active player characters, oldest first, with what the
-- fog needs to know of how each one sees (package maps): the sheet, the beast of
-- a Wild Shape form, and the familiar the player looks through, if it is still with
-- the character (MR-036, MR-037).
SELECT c.id, c.player_user_id, c.sheet, ws.beast AS wild_shape_beast,
       cc.id AS familiar_id, cc.monster_key AS familiar_monster_key
FROM characters AS c
LEFT JOIN character_wild_shapes AS ws ON ws.character_id = c.id
LEFT JOIN character_vitals AS v ON v.character_id = c.id
LEFT JOIN character_creatures AS cc ON cc.id = v.familiar_sight_creature_id AND cc.dismissed_at IS NULL
WHERE c.campaign_id = sqlc.arg(campaign_id)::UUID
  AND c.kind = 'player' AND c.status = 'active'
ORDER BY c.created_at, c.id;

-- name: ListMapCreatures :many
-- The live creatures of the given IDs that belong to a living player's character
-- of the campaign: the ones that may have a token on a map (package maps). Oldest
-- first.
SELECT cc.id, cc.character_id, cc.name, cc.monster_key, c.player_user_id
FROM character_creatures AS cc
JOIN characters AS c ON c.id = cc.character_id
WHERE cc.campaign_id = sqlc.arg(campaign_id)::UUID
  AND cc.id = ANY(sqlc.arg(ids)::UUID[])
  AND cc.dismissed_at IS NULL
  AND c.status = 'active'
ORDER BY cc.created_at, cc.id;


-- The table's own content (MR-025, RN-23, ADR-0018): campaign_content, one row
-- per entry, and campaign_content_state, the campaign's content revision.

-- name: GetContentRevision :one
-- The campaign's content revision. No row is revision 0 (no content of its own).
-- Read in the caller's transaction: it orders a content write against a sheet
-- write, and the cache of the live content is keyed by it.
SELECT revision FROM campaign_content_state WHERE campaign_id = sqlc.arg(campaign_id)::UUID;

-- name: BumpContentRevision :one
-- Every content write starts here: the revision goes up by one (the row is made
-- by the first write), and the row is held until the transaction ends, so two
-- writes of one campaign run one after the other.
INSERT INTO campaign_content_state (campaign_id, revision, updated_at)
VALUES (sqlc.arg(campaign_id)::UUID, 1, sqlc.arg(now))
ON CONFLICT (campaign_id) DO UPDATE
SET revision = campaign_content_state.revision + 1, updated_at = EXCLUDED.updated_at
RETURNING revision;

-- name: ListCampaignContent :many
-- Every entry of the campaign, archived ones included, sorted by key.
SELECT * FROM campaign_content
WHERE campaign_id = sqlc.arg(campaign_id)::UUID
ORDER BY content_key;

-- name: GetCampaignContent :one
SELECT * FROM campaign_content
WHERE campaign_id = sqlc.arg(campaign_id)::UUID AND content_key = sqlc.arg(content_key);

-- name: InsertCampaignContent :one
INSERT INTO campaign_content
    (campaign_id, content_key, kind, name_pt, data, revision, created_at, updated_at)
VALUES (
    sqlc.arg(campaign_id)::UUID, sqlc.arg(content_key), sqlc.arg(kind), sqlc.arg(name_pt), sqlc.arg(data),
    sqlc.arg(revision), sqlc.arg(now), sqlc.arg(now)
)
RETURNING *;

-- name: UpdateCampaignContent :one
-- The body and the name; the key, the kind and the archive mark stay.
UPDATE campaign_content
SET name_pt = sqlc.arg(name_pt), data = sqlc.arg(data), revision = sqlc.arg(revision), updated_at = sqlc.arg(now)
WHERE campaign_id = sqlc.arg(campaign_id)::UUID AND content_key = sqlc.arg(content_key)
RETURNING *;

-- name: SetCampaignContentArchived :one
-- archived true retires the entry, false brings it back. The revision is the
-- campaign's new one: an archive is a change too.
UPDATE campaign_content
SET archived_at = CASE WHEN sqlc.arg(archived)::BOOL THEN sqlc.arg(now)::TIMESTAMPTZ ELSE NULL END,
    revision = sqlc.arg(revision), updated_at = sqlc.arg(now)
WHERE campaign_id = sqlc.arg(campaign_id)::UUID AND content_key = sqlc.arg(content_key)
RETURNING *;

-- name: ListCampaignSheets :many
-- Every character of the campaign with the sheet: who uses which table entry,
-- and who has issues after a change. Players' characters first, then NPCs.
SELECT id, kind, name, player_user_id, sheet
FROM characters
WHERE campaign_id = sqlc.arg(campaign_id)::UUID
ORDER BY kind <> 'player', created_at, id;
