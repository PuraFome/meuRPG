-- +goose Up
-- map_vision_memory is what each player saw of a map with the fog of war on
-- (MR-036, Etapa 9, D6): the squares their character (or the group, with "Visão
-- do grupo") has seen, so that the player keeps them, darkened, after moving
-- away. One row per map and player. seen is a packed bitmap in the byte layout
-- of package rules/grid (layer.go): one bit a square, row-major, the least
-- significant bit first, ceil(squares/8) bytes (at most 10 KB for a 200 x 400
-- map). A wall seen next to a seen square counts as seen.
--
-- It only grows: a write that changes what someone sees (a token placed or
-- moved, a carried light, a wall or light painted, a Luz point, the fog's
-- settings) adds the squares now seen. It holds squares and never creatures,
-- so a remembered square shows no NPC. The master clears it with "Esquecer o
-- que foi visto" (ForgetMapVision), and changing the map's grid or image clears
-- it too, since a bitmap only fits the grid it was made on.
--
-- epoch is the map's vision_epoch (00107) the bitmap was built for. Every clear
-- (a new grid or image, "Esquecer o que foi visto") bumps the map's epoch, and a
-- row with an older epoch reads as empty and is overwritten: so a refresh that was
-- running while the clear happened can never bring the old bitmap back.
--
-- Deleting the map or the player deletes the row. What is stored is where a
-- player has been looking in a game: it is personal data about play, listed in
-- docs/privacy.md.
CREATE TABLE IF NOT EXISTS map_vision_memory (
    map_id UUID NOT NULL REFERENCES maps (id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    seen BYTEA NOT NULL,
    epoch INT4 NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT map_vision_memory_pkey PRIMARY KEY (map_id, user_id)
);

-- +goose Down
DROP TABLE IF EXISTS map_vision_memory;
