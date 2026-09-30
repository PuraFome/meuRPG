-- +goose Up
-- character_vitals holds a player character's live numbers, which last from
-- one game session to the next (RN-02): current and temporary hit points,
-- spell slots used per spell level, pact magic slots used, and hit dice
-- used. The name is "vitals", not "state", because a character's state
-- already means its lifecycle (draft, locked, dead, pending).
--
-- The maximums are never stored. Package rules derives them from the sheet
-- on every read (maximum hit points, slots per spell level, pact slots, hit
-- dice), and the service clamps these values to them, so a sheet that
-- changes level never leaves a value above its new maximum.
--
-- A missing row means "fresh": current hit points at the maximum, no
-- temporary hit points, nothing used. The master corrects the values during
-- an open game session (PlayService.AdjustCharacterVitals), and each
-- correction also writes a session_events row, in the same transaction.
--
-- spell_slots_used[k] (counting from 1, as SQL arrays do) is how many slots
-- of spell level k are used, for at most 9 levels. revision goes up by one
-- on every change, so the app can tell a newer value from an older one.
--
-- Only player characters get a row; NPCs' numbers in a fight come with the
-- combatants (Etapa 6). Deleting the character deletes the row. There is no
-- personal data here, only numbers.
CREATE TABLE IF NOT EXISTS character_vitals (
    character_id UUID PRIMARY KEY REFERENCES characters (id) ON DELETE CASCADE,
    hit_points_current INT4 NOT NULL,
    hit_points_temporary INT4 NOT NULL DEFAULT 0,
    spell_slots_used INT4[] NOT NULL DEFAULT '{}',
    pact_slots_used INT4 NOT NULL DEFAULT 0,
    hit_dice_used INT4 NOT NULL DEFAULT 0,
    revision INT4 NOT NULL DEFAULT 1,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT character_vitals_hit_points_valid CHECK (hit_points_current >= 0 AND hit_points_temporary >= 0),
    CONSTRAINT character_vitals_spell_slots_valid CHECK (
        COALESCE(array_length(spell_slots_used, 1), 0) <= 9 AND 0 <= ALL (spell_slots_used)
    ),
    CONSTRAINT character_vitals_used_valid CHECK (pact_slots_used >= 0 AND hit_dice_used >= 0),
    CONSTRAINT character_vitals_revision_valid CHECK (revision >= 1)
);

-- +goose Down
DROP TABLE IF EXISTS character_vitals;
