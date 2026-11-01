-- +goose Up
-- character_creatures.stat_block is the numbers of a table creature whose entry has formulas, worked out
-- for its owner when the creature is created (rules.ResolvedCreature as JSON). It is a snapshot, like
-- hp_max: a later level of the owner or an edit of the entry's formulas does not change it. NULL for an
-- SRD creature and for a table creature with no formula, whose numbers are read from the content.
ALTER TABLE character_creatures ADD COLUMN IF NOT EXISTS stat_block JSONB NULL;

-- +goose Down
ALTER TABLE character_creatures DROP COLUMN IF EXISTS stat_block;
