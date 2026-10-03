-- +goose Up
-- resources_used holds the uses spent of a character's class and race
-- resources (Retomar o fôlego, Surto de ação, Fúria...; RN-02, Etapa 6), as a
-- JSON object {resource key: uses}, like spell_slots_used for the slots. The
-- totals come from the sheet (package rules derives them, like the slots) and
-- the service clamps the stored values to them on every read. The keys are the
-- sheet's resource keys, never a name. There is no rest in the app yet: the
-- master corrects the uses (AdjustCharacterVitals).
--
-- One statement with several parts, so re-running it is safe (see 00036).
ALTER TABLE character_vitals
    ADD COLUMN IF NOT EXISTS resources_used JSONB NOT NULL DEFAULT '{}',
    DROP CONSTRAINT IF EXISTS character_vitals_resources_used_valid,
    ADD CONSTRAINT character_vitals_resources_used_valid CHECK (jsonb_typeof(resources_used) = 'object');

-- +goose Down
ALTER TABLE character_vitals
    DROP CONSTRAINT IF EXISTS character_vitals_resources_used_valid,
    DROP COLUMN IF EXISTS resources_used;
