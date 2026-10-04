-- +goose Up
-- The combatants that joined a combat before 00110 get what the new columns
-- would have held: the tenths of a foot already walked (movement_used_ft times
-- ten), and 'party' for a player's character. Both statements only touch a row
-- that still holds the column's default, so re-running this is safe. A combat
-- that was running keeps its movement; its jump limits stay 0 (no jump) until
-- the combatants are added again.
UPDATE combatants
SET movement_used_dft = movement_used_ft * 10
WHERE movement_used_dft = 0 AND movement_used_ft > 0;

UPDATE combatants
SET side = 'party'
WHERE kind = 'player' AND side = 'enemy';

-- +goose Down
-- Nothing to undo: the values are valid with or without the columns.
SELECT 1;
