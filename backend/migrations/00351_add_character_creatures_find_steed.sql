-- +goose Up
-- The steed of Find Steed (Convocar Montaria), the character's creature of the source 'find_steed'. creature_type is the
-- type the player picked for it ('celestial', 'fey' or 'fiend'; '' for every other creature, whose stat block says its
-- own). bonded marks the character's one steed: dismissing or losing it keeps the row, and a new casting brings the same
-- row back at full hit points, so a character has at most one bonded creature (the unique index of 00352). The reasons a creature
-- is dismissed gain 'disappeared' (the steed fell to 0 hit points) and 'dismissed' (its owner sent it away and kept the
-- bond). One statement with several parts, so re-running it is safe (see 00104); 'table' and every older value stay.
ALTER TABLE character_creatures
    ADD COLUMN IF NOT EXISTS creature_type TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS bonded BOOLEAN NOT NULL DEFAULT false,
    DROP CONSTRAINT IF EXISTS character_creatures_source_valid,
    ADD CONSTRAINT character_creatures_source_valid CHECK (source IN ('familiar', 'animate_dead', 'conjure_animals', 'master', 'table', 'find_steed')),
    DROP CONSTRAINT IF EXISTS character_creatures_dismissed_valid,
    ADD CONSTRAINT character_creatures_dismissed_valid CHECK (
        (dismissed_at IS NULL) = (dismissed_reason IS NULL)
        AND (dismissed_reason IS NULL OR dismissed_reason IN ('owner', 'master', 'defeated', 'concentration', 'replaced', 'undone', 'disappeared', 'dismissed'))
    );

-- A statement of its own, after the two above, so that a second run drops and adds the constraints in the order of the first.
ALTER TABLE character_creatures
    DROP CONSTRAINT IF EXISTS character_creatures_creature_type_valid,
    ADD CONSTRAINT character_creatures_creature_type_valid CHECK (creature_type IN ('', 'celestial', 'fey', 'fiend'));

-- +goose Down
DELETE FROM character_creatures WHERE source = 'find_steed' OR dismissed_reason IN ('disappeared', 'dismissed');
ALTER TABLE character_creatures
    DROP CONSTRAINT IF EXISTS character_creatures_dismissed_valid,
    ADD CONSTRAINT character_creatures_dismissed_valid CHECK (
        (dismissed_at IS NULL) = (dismissed_reason IS NULL)
        AND (dismissed_reason IS NULL OR dismissed_reason IN ('owner', 'master', 'defeated', 'concentration', 'replaced', 'undone'))
    ),
    DROP CONSTRAINT IF EXISTS character_creatures_source_valid,
    ADD CONSTRAINT character_creatures_source_valid CHECK (source IN ('familiar', 'animate_dead', 'conjure_animals', 'master', 'table')),
    DROP CONSTRAINT IF EXISTS character_creatures_creature_type_valid,
    DROP COLUMN IF EXISTS bonded,
    DROP COLUMN IF EXISTS creature_type;
