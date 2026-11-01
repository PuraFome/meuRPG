-- +goose Up
-- creature_spent keeps what a creature of a stat block has spent in this combat, as JSON
-- {"<key>": N}: only what was spent, a key that is absent is 0. Named keys: "legendary_actions"
-- (legendary actions taken since the creature's last turn, cleared when its own turn starts) and
-- "legendary_resistance" (uses spent in this combat). The namespaces "use:<action key>" and
-- "recharge:<action key>" are reserved for the N/day and Recharge actions. Keys, numbers and
-- nothing else; no personal data. A new combatant starts with '{}'. Its text is at most 4 KiB.
--
-- Production is live: the column only adds, with every row as it was ('{}'), so running the
-- migration twice is safe.
ALTER TABLE combatants
    ADD COLUMN IF NOT EXISTS creature_spent JSONB NOT NULL DEFAULT '{}',
    DROP CONSTRAINT IF EXISTS combatants_creature_spent_valid,
    ADD CONSTRAINT combatants_creature_spent_valid CHECK (jsonb_typeof(creature_spent) = 'object' AND octet_length(creature_spent::TEXT) <= 4096);

-- +goose Down
ALTER TABLE combatants
    DROP CONSTRAINT IF EXISTS combatants_creature_spent_valid,
    DROP COLUMN IF EXISTS creature_spent;
