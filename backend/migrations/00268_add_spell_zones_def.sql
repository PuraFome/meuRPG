-- +goose Up
-- The definition of a zone the table's own spell keeps (the master's spell, not one of the SRD's four, whose rows are
-- the code's): shape, size, when it catches, whether it follows the caster, kept at cast time as JSON so that a zone
-- outlives an edit of the spell. NULL for the SRD's.
ALTER TABLE spell_zones ADD COLUMN IF NOT EXISTS def JSONB NULL;

-- +goose Down
ALTER TABLE spell_zones DROP COLUMN IF EXISTS def;
