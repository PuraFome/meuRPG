-- +goose Up
-- A scene action can be hidden from the players (MR-015, RN-10): the master prepares it, or turns it
-- between "Os jogadores veem" and "Escondida" while the scene runs. A hidden action never reaches a
-- player (not its name, DC, attempts or rolls), and a player's roll on it is refused as an action that
-- does not exist.
--
-- Production is live: the column only adds, with every row visible as before, so running the
-- migration twice is safe.
ALTER TABLE scene_actions ADD COLUMN IF NOT EXISTS hidden BOOL NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE scene_actions DROP COLUMN IF EXISTS hidden;
