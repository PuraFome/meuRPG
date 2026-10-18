-- +goose Up
-- table_once_turns says, for each once-per-turn extra of the table's features (extra_damage), the turn
-- ("<round>:<combatant on turn>") in which it last added its damage, as JSON {"<feature key>": "<turn>"}.
-- sneak_attack_turn and colossus_slayer_turn (00215) are the same marks for those two SRD features.
ALTER TABLE combatants
    ADD COLUMN IF NOT EXISTS table_once_turns JSONB NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE combatants
    DROP COLUMN IF EXISTS table_once_turns;
