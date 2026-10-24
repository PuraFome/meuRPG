-- +goose Up
-- An NPC on the stage can be hidden from the players (MR-031, RN-10): the master puts it there while
-- preparing, or turns it between "Os jogadores veem" and "Escondida dos jogadores" while the scene
-- runs. A hidden NPC is still one of the 4 the stage holds, but it never reaches a player: not the
-- open scene, not the stream, not its portrait. A hidden NPC does not speak: hiding clears the mark,
-- and giving it the floor shows it first.
--
-- Production is live: the column only adds, with every row visible as before, so running the
-- migration twice is safe.
ALTER TABLE stage_npcs ADD COLUMN IF NOT EXISTS hidden BOOL NOT NULL DEFAULT false;

ALTER TABLE stage_npcs DROP CONSTRAINT IF EXISTS stage_npcs_hidden_does_not_speak;
ALTER TABLE stage_npcs ADD CONSTRAINT stage_npcs_hidden_does_not_speak CHECK (NOT (hidden AND speaking));

-- +goose Down
ALTER TABLE stage_npcs DROP CONSTRAINT IF EXISTS stage_npcs_hidden_does_not_speak;
ALTER TABLE stage_npcs DROP COLUMN IF EXISTS hidden;
