-- +goose Up
-- hit_dice_used_by_die holds the hit dice a character has spent, by die size, as a
-- JSON object {die size: count}, such as {"10": 2, "6": 1} for two d10 and a d6
-- (SRD 5.1, "Multiclassing": hit dice of different sizes are kept apart, a short
-- rest spends them by size and a long rest gives them back by size). NULL means the
-- row was written before the sizes were kept: hit_dice_used, a count with no sizes,
-- is then read as spent dice, the largest first, and the next write stores the
-- object. hit_dice_used keeps the sum, so a count and its sizes never disagree. The
-- maximums are never stored (they come from the sheet, like the slots), and the
-- service clamps what is stored to them on every read.
--
-- spell_slots_created[k] (counting from 1) is how many spell slots of level k
-- Flexible Casting created (SRD 5.1, Sorcerer): they add to the sheet's slots and
-- vanish on a long rest, the way spell_slots_used goes back to nothing.
--
-- One statement with several parts, so re-running it is safe (see 00036).
ALTER TABLE character_vitals
    ADD COLUMN IF NOT EXISTS hit_dice_used_by_die JSONB NULL,
    DROP CONSTRAINT IF EXISTS character_vitals_hit_dice_used_by_die_valid,
    ADD CONSTRAINT character_vitals_hit_dice_used_by_die_valid CHECK (
        hit_dice_used_by_die IS NULL OR jsonb_typeof(hit_dice_used_by_die) = 'object'
    ),
    ADD COLUMN IF NOT EXISTS spell_slots_created INT4[] NOT NULL DEFAULT '{}',
    DROP CONSTRAINT IF EXISTS character_vitals_spell_slots_created_valid,
    ADD CONSTRAINT character_vitals_spell_slots_created_valid CHECK (
        COALESCE(array_length(spell_slots_created, 1), 0) <= 9 AND 0 <= ALL (spell_slots_created)
    );

-- +goose Down
ALTER TABLE character_vitals
    DROP CONSTRAINT IF EXISTS character_vitals_spell_slots_created_valid,
    DROP COLUMN IF EXISTS spell_slots_created,
    DROP CONSTRAINT IF EXISTS character_vitals_hit_dice_used_by_die_valid,
    DROP COLUMN IF EXISTS hit_dice_used_by_die;
