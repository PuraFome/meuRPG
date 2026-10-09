-- +goose Up
CREATE INDEX IF NOT EXISTS combat_effects_encounter_id_idx ON combat_effects (encounter_id);
CREATE INDEX IF NOT EXISTS combat_effects_group_id_idx ON combat_effects (group_id);
CREATE INDEX IF NOT EXISTS combat_effects_caster_id_idx ON combat_effects (caster_id);
CREATE INDEX IF NOT EXISTS combat_effect_targets_combatant_id_idx ON combat_effect_targets (combatant_id);
CREATE INDEX IF NOT EXISTS combat_effect_saves_encounter_id_idx ON combat_effect_saves (encounter_id);
CREATE INDEX IF NOT EXISTS combat_effect_saves_effect_id_idx ON combat_effect_saves (effect_id);
CREATE UNIQUE INDEX IF NOT EXISTS combat_effect_saves_turn_idx ON combat_effect_saves (effect_id, combatant_id, round, phase);

-- +goose Down
DROP INDEX IF EXISTS combat_effect_saves_turn_idx;
DROP INDEX IF EXISTS combat_effect_saves_effect_id_idx;
DROP INDEX IF EXISTS combat_effect_saves_encounter_id_idx;
DROP INDEX IF EXISTS combat_effect_targets_combatant_id_idx;
DROP INDEX IF EXISTS combat_effects_caster_id_idx;
DROP INDEX IF EXISTS combat_effects_group_id_idx;
DROP INDEX IF EXISTS combat_effects_encounter_id_idx;
