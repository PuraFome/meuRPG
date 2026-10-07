-- +goose Up
-- characters holds every character: the players' characters and the
-- masters' NPCs (MR-003, MR-004, MR-005). kind says which: 'player', or one
-- of the NPC kinds 'enemy', 'boss', 'minion' and 'story'.
--
-- The sheet is a document, not columns. sheet is the JSON form (protojson,
-- with proto field names) of meurpg.characters.v1.CharacterSheet: the
-- player's choices (content keys such as "class:wizard", ability scores,
-- chosen skills and spells, equipment). story is the JSON form of
-- meurpg.characters.v1.CharacterStory: the descriptive text (personality,
-- appearance, backstory, allies). Nothing ever filters on what is inside
-- either document, so there is no index on them. Derived numbers (modifiers,
-- spell save DC, hit points) are never stored: the rules module computes
-- them on every read (ADR-0008). Renaming a field or an enum value in those
-- proto messages would break the stored JSON; buf breaking forbids it.
--
-- story is separate from sheet because it follows its own lock: after the
-- sheet locks (RN-01), the player edits the story only while the master
-- allows it (story_editing_allowed, below).
--
-- Ownership uses two columns, so each kind gets the right ON DELETE:
--   - player_user_id (player characters only): SET NULL. When a player
--     deletes their account, the character stays with the campaign, without
--     its owner (RN-16).
--   - master_user_id (NPCs only): CASCADE. A master's NPCs are theirs
--     (RN-04) and go with the account.
-- campaign_id is the one campaign of a player character (RN-03) and the
-- campaign an NPC was created in. SET NULL: when a campaign is deleted, its
-- player characters stay with their players (docs/privacy.md).
--
-- status is 'active', 'dead' or 'pending'. A dead character is never deleted
-- (RN-03): it changes status, never row. 'pending' is for a character that
-- waits for the master's approval (RN-15, MR-024); it is allowed here from
-- the start so that MR-024 needs no change to this constraint. The
-- character's state (draft, locked, dead, pending) is computed from status
-- and sheet_locked_at: sheet_locked_at is set when a game session starts
-- (RN-01), and only player sheets lock.
--
-- story_editing_allowed is the master's permission for the player to edit
-- the story of a locked or dead character (decided by Vinicius on
-- 29/09/2026). While a character is a draft, its player edits the story
-- anyway; the master always may. Starting a game session turns the
-- permission off, as it locks the sheet.
--
-- revision goes up by one on every change to name, sheet or story, so two
-- people editing at once cannot silently overwrite each other (the API
-- answers "aborted" to a stale revision). sheet_schema marks the version of
-- the sheet document, for a future v2 of the message.
--
-- INT4, because CockroachDB's INT is 64 bits and these are small counts
-- (the API sends revision as an int32).
--
-- The CHECKs below keep the invariants even if application code has a bug.
-- IS NOT NULL is spelled out where it matters, because a CHECK that
-- evaluates to NULL passes.
CREATE TABLE IF NOT EXISTS characters (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id UUID NULL REFERENCES campaigns (id) ON DELETE SET NULL,
    kind TEXT NOT NULL,
    player_user_id UUID NULL REFERENCES users (id) ON DELETE SET NULL,
    master_user_id UUID NULL REFERENCES users (id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'active',
    name TEXT NOT NULL,
    sheet JSONB NOT NULL,
    story JSONB NOT NULL DEFAULT '{}'::JSONB,
    story_editing_allowed BOOL NOT NULL DEFAULT false,
    sheet_schema INT4 NOT NULL DEFAULT 1,
    revision INT4 NOT NULL DEFAULT 1,
    sheet_locked_at TIMESTAMPTZ NULL,
    died_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT characters_kind_valid CHECK (kind IN ('player', 'enemy', 'boss', 'minion', 'story')),
    CONSTRAINT characters_status_valid CHECK (status IN ('active', 'dead', 'pending')),
    -- A player character never has a master owner; an NPC always has one and
    -- never has a player. player_user_id may be NULL for a player character
    -- whose player deleted their account (RN-16).
    CONSTRAINT characters_owner CHECK (
        (kind = 'player' AND master_user_id IS NULL)
        OR (kind <> 'player' AND player_user_id IS NULL AND master_user_id IS NOT NULL)
    ),
    -- Only player characters die or wait for approval; NPCs stay active.
    CONSTRAINT characters_only_players_change_status CHECK (kind = 'player' OR status = 'active'),
    -- Only player sheets lock (RN-01); the master always edits NPCs.
    CONSTRAINT characters_only_players_lock CHECK (kind = 'player' OR sheet_locked_at IS NULL),
    -- Only a player's story needs the master's permission.
    CONSTRAINT characters_only_players_story_permission CHECK (kind = 'player' OR NOT story_editing_allowed),
    CONSTRAINT characters_dead_since CHECK ((status = 'dead') = (died_at IS NOT NULL)),
    CONSTRAINT characters_name_length CHECK (char_length(name) BETWEEN 1 AND 80),
    -- The size CHECKs are the last line of defense against an oversized
    -- document, set so that no input within the API's limits can reach
    -- them: those limits count characters, and one character takes up to
    -- 4 bytes in UTF-8. A full sheet within the limits stays under about
    -- 90 KB; a story (18,240 characters at most) under about 75 KB. Both get
    -- 128 KiB.
    CONSTRAINT characters_sheet_object CHECK (jsonb_typeof(sheet) = 'object'),
    CONSTRAINT characters_sheet_size CHECK (octet_length(sheet::TEXT) <= 131072),
    CONSTRAINT characters_story_object CHECK (jsonb_typeof(story) = 'object'),
    CONSTRAINT characters_story_size CHECK (octet_length(story::TEXT) <= 131072),
    CONSTRAINT characters_sheet_schema_valid CHECK (sheet_schema >= 1),
    CONSTRAINT characters_revision_valid CHECK (revision >= 1)
);

-- +goose Down
DROP TABLE IF EXISTS characters;
