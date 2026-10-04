-- +goose Up
-- The combatant of a creature: dismissing a creature takes it out of its
-- combat, and the ON DELETE CASCADE of combatants.creature_id looks it up.
CREATE INDEX IF NOT EXISTS combatants_creature_id_idx
    ON combatants (creature_id)
    WHERE creature_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS combatants_creature_id_idx;
