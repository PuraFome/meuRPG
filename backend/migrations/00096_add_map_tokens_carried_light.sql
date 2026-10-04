-- +goose Up
-- carried_light is the light a character carries on a map (MR-036, Etapa 9, D6):
-- the key of a light preset ("light:torch", "light:hooded-lantern"), or NULL for
-- none. It moves with the token. The player sets it for their own character and
-- the master for anyone. Only the master and the character's own player read it
-- (the other players learn of it only through what they see, slice 9.4). The API
-- checks the key against the rules content.
--
-- One statement, as 00067, so re-running it is safe.
ALTER TABLE map_tokens
    ADD COLUMN IF NOT EXISTS carried_light TEXT NULL,
    DROP CONSTRAINT IF EXISTS map_tokens_carried_light_length,
    ADD CONSTRAINT map_tokens_carried_light_length CHECK (carried_light IS NULL OR char_length(carried_light) BETWEEN 1 AND 60);

-- +goose Down
ALTER TABLE map_tokens
    DROP CONSTRAINT IF EXISTS map_tokens_carried_light_length,
    DROP COLUMN IF EXISTS carried_light;
