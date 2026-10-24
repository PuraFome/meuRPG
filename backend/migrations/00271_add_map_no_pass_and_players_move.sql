-- +goose Up
-- The "can't pass" layer of a map (plan B, B2): one bit a square, in the byte layout of
-- package rules/grid (layer.go) like the walls and the difficult terrain. A set square
-- stops movement, in or out of combat, but not sight or light. NULL when nothing is
-- painted; a new grid or a new image deletes it with the rest of the row.
ALTER TABLE map_layers
    ADD COLUMN IF NOT EXISTS no_pass BYTEA NULL;

-- Whether each player may move their own character's token on the map outside combat
-- (SetMapPlayersMove). Off until the master turns it on.
ALTER TABLE maps
    ADD COLUMN IF NOT EXISTS players_move_tokens BOOL NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE maps
    DROP COLUMN IF EXISTS players_move_tokens;
ALTER TABLE map_layers
    DROP COLUMN IF EXISTS no_pass;
