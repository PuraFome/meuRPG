-- +goose Up
-- The ON DELETE SET NULL of character_vitals.familiar_sight_creature_id looks
-- the sights of a deleted creature up by it.
CREATE INDEX IF NOT EXISTS character_vitals_familiar_sight_creature_id_idx
    ON character_vitals (familiar_sight_creature_id)
    WHERE familiar_sight_creature_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS character_vitals_familiar_sight_creature_id_idx;
