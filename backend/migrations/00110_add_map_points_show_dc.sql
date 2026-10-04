-- +goose Up
-- show_dc is the master's switch "Mostrar a CD aos jogadores" on a SCENE point
-- (MR-015, Etapa 8, question 52: "deixar as duas opções, para o mestre
-- decidir"). Off by default: the players never see a DC or the pass or fail of
-- a roll (RN-20). On, they see the DC of each action that has one, and whether
-- their own rolls passed. Only a scene has the switch: the API keeps it false
-- on any other kind and clears it when the point stops being a scene.
--
-- One statement, as 00067, so re-running it is safe.
ALTER TABLE map_points
    ADD COLUMN IF NOT EXISTS show_dc BOOL NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE map_points
    DROP COLUMN IF EXISTS show_dc;
