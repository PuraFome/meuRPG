-- +goose Up
CREATE INDEX IF NOT EXISTS combatant_states_group_id_idx ON combatant_states (group_id);

-- +goose Down
DROP INDEX IF EXISTS combatant_states_group_id_idx;
