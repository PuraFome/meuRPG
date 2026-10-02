-- +goose Up
-- map_tokens are the characters standing on a map (MR-012): at most one
-- token per character per map, so (map_id, character_id) is the primary
-- key, which also lists a map's tokens.
--
-- The character is one of the map's campaign's living characters, a
-- player's or an NPC; the API checks that through the characters module.
-- x_bp and y_bp work as in map_points. hidden is the master's switch
-- (RN-10): a player's character starts visible, an NPC hidden (question 31
-- for Samuel), and the server never sends a hidden token to a player.
--
-- Deleting the map or the character deletes the token. There is no index
-- by character_id: only deleting a character looks tokens up that way, and
-- characters are deleted almost never (a rejected pending character, which
-- never has a token, or an account deletion), as for
-- campaigns.created_by. No personal data: IDs, a position and a flag.
CREATE TABLE IF NOT EXISTS map_tokens (
    map_id UUID NOT NULL REFERENCES maps (id) ON DELETE CASCADE,
    character_id UUID NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    x_bp INT4 NOT NULL,
    y_bp INT4 NOT NULL,
    hidden BOOL NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT map_tokens_pkey PRIMARY KEY (map_id, character_id),
    CONSTRAINT map_tokens_position_valid CHECK (x_bp BETWEEN 0 AND 10000 AND y_bp BETWEEN 0 AND 10000)
);

-- +goose Down
DROP TABLE IF EXISTS map_tokens;
