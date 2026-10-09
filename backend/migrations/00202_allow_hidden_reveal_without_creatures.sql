-- +goose Up
-- Under the table rule "Perguntar a cada vez" every area spell a player casts holds the
-- turn, so that the wait itself says nothing of whether a hidden creature was in the
-- area (RN-10): a question may have no creature to reveal, and the master answers it
-- with one tap.
ALTER TABLE hidden_reveals DROP CONSTRAINT IF EXISTS hidden_reveals_combatants_valid;
ALTER TABLE hidden_reveals ADD CONSTRAINT hidden_reveals_combatants_valid CHECK (cardinality(combatant_ids) BETWEEN 0 AND 40);

-- +goose Down
ALTER TABLE hidden_reveals DROP CONSTRAINT IF EXISTS hidden_reveals_combatants_valid;
ALTER TABLE hidden_reveals ADD CONSTRAINT hidden_reveals_combatants_valid CHECK (cardinality(combatant_ids) BETWEEN 1 AND 40);
