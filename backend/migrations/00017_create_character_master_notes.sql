-- +goose Up
-- character_master_notes holds the master's private notes about a character
-- (RN-11). They live in their own table, reached only by the master-only
-- RPCs (CharacterService.GetMasterNotes and UpdateMasterNotes): the queries
-- that build a character for a player never touch this table, so the notes
-- cannot leak into a player's response.
--
-- The primary key is per campaign, so a character used in more than one
-- campaign (an NPC reused by its master, MR-022) gets separate notes in each.
-- Empty notes are not stored: saving empty notes deletes the row.
--
-- Deleting the campaign or the character deletes the notes. Characters are
-- deleted only with their owner's account (NPCs, CASCADE), so the delete
-- through character_id scans this small table instead of using an index, as
-- for campaigns.created_by.
CREATE TABLE IF NOT EXISTS character_master_notes (
    campaign_id UUID NOT NULL REFERENCES campaigns (id) ON DELETE CASCADE,
    character_id UUID NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    notes TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (campaign_id, character_id),
    CONSTRAINT character_master_notes_length CHECK (char_length(notes) BETWEEN 1 AND 20000)
);

-- +goose Down
DROP TABLE IF EXISTS character_master_notes;
