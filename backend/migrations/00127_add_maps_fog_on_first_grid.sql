-- +goose Up
-- fog_on_first_grid says that the table's rule "névoa nos mapas novos" (RN-24) was
-- on when the map was created and has not been applied yet. The fog is made of the
-- grid's squares, so a map without a grid cannot have it (SetMapFog refuses it, and
-- the fog code reads it as off): the rule is applied when the master sets the map's
-- first grid, which turns the fog on and clears this flag. Switching the fog by hand
-- (SetMapFog) clears it too: the master decided.
--
-- One statement, so re-running it is safe (see 00067).
ALTER TABLE maps
    ADD COLUMN IF NOT EXISTS fog_on_first_grid BOOL NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE maps
    DROP COLUMN IF EXISTS fog_on_first_grid;
