-- +goose Up
-- A combatant's reaction records which reaction was spent and in which round (SRD 5.1, Combat: "Reactions": you can
-- take one reaction between turns, and it comes back at the start of your turn).
--   - reaction_kind is the reaction that was spent: the key of a reaction window (shield, uncanny_dodge, ...),
--     "opportunity" for an opportunity attack, or the key of the feature whose action is a reaction.
--   - reaction_round is the round of the combat when it was spent.
-- Both are NULL while reaction_used is false: the start of the combatant's turn clears them with the flag, and so
-- does the undo that gives the reaction back.
--
-- Production is live: the columns only add and allow NULL, so running the migration twice is safe.
ALTER TABLE combatants ADD COLUMN IF NOT EXISTS reaction_kind TEXT NULL;
ALTER TABLE combatants ADD COLUMN IF NOT EXISTS reaction_round INT4 NULL;

-- +goose Down
ALTER TABLE combatants DROP COLUMN IF EXISTS reaction_round;
ALTER TABLE combatants DROP COLUMN IF EXISTS reaction_kind;
