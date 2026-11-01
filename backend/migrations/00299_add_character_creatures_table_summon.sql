-- +goose Up
-- A creature of a table spell's summon (rules.TableSummon) has source 'table' and the spell's key in
-- summon_key ('spell:<slug>@mesa'): what a new casting replaces (the same spell's creatures) and what a
-- concentration spell's creatures are tied to. It is NULL for every other source, and only they have one.
-- One statement with several parts, so re-running it is safe (see 00104).
ALTER TABLE character_creatures
    ADD COLUMN IF NOT EXISTS summon_key TEXT NULL,
    DROP CONSTRAINT IF EXISTS character_creatures_source_valid,
    ADD CONSTRAINT character_creatures_source_valid CHECK (source IN ('familiar', 'animate_dead', 'conjure_animals', 'master', 'table')),
    DROP CONSTRAINT IF EXISTS character_creatures_summon_key_valid,
    ADD CONSTRAINT character_creatures_summon_key_valid CHECK (
        (source = 'table') = (summon_key IS NOT NULL) AND (summon_key IS NULL OR char_length(summon_key) BETWEEN 1 AND 100)
    );

-- +goose Down
DELETE FROM character_creatures WHERE source = 'table';
ALTER TABLE character_creatures
    DROP CONSTRAINT IF EXISTS character_creatures_summon_key_valid,
    DROP CONSTRAINT IF EXISTS character_creatures_source_valid,
    ADD CONSTRAINT character_creatures_source_valid CHECK (source IN ('familiar', 'animate_dead', 'conjure_animals', 'master')),
    DROP COLUMN IF EXISTS summon_key;
