-- +goose Up
-- combatants.melee_hit_turn is the turn ("<round>:<combatant on turn>") in which the
-- combatant last hit with a melee weapon attack. A table maneuver that starts a grapple
-- after a hit reads it (applies "grapple", economy bonus_action). Adds only; running it
-- twice is safe.
ALTER TABLE combatants
    ADD COLUMN IF NOT EXISTS melee_hit_turn TEXT NULL;

-- +goose Down
ALTER TABLE combatants DROP COLUMN IF EXISTS melee_hit_turn;
