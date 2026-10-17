-- +goose Up
-- The open questions of a combatant, and of a combat (00255 added the columns): the one question at
-- a time rule and the combat's view read the holds that wait for an answer, by combat and roller.
-- Alone in its migration, as CONTRIBUTING asks of an index.
CREATE INDEX IF NOT EXISTS inspiration_holds_open_combat_idx ON inspiration_holds (encounter_id, combatant_id) WHERE answer_key IS NULL;

-- +goose Down
DROP INDEX IF EXISTS inspiration_holds_open_combat_idx;
