-- +goose Up
-- Whether the window to attune a magic item is open for a character (SRD 5.1: attuning takes a
-- short rest). The master's short or long rest opens it; one attunement or ending by the
-- owner spends it. A character without a vitals row has it closed.
-- Production is live: the column only adds, with every row as it was (false), so running the
-- migration twice is safe.
ALTER TABLE character_vitals
    ADD COLUMN IF NOT EXISTS attunement_open BOOLEAN NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE character_vitals
    DROP COLUMN IF EXISTS attunement_open;
