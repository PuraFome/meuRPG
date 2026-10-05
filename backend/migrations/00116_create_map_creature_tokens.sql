-- +goose Up
-- map_creature_tokens are the creatures of the characters standing on a map
-- (MR-037, Etapa 9, D7): the master places one like any token. A table of its
-- own, not rows of map_tokens, because map_tokens is keyed by (map_id,
-- character_id) and a character's creatures are many, and its own token must
-- stay one. At most one token per creature per map.
--
-- A creature's token is a party token (D6): there is no hidden flag, and no
-- player is ever kept from it, as with a player character's. x_bp and y_bp work
-- as in map_tokens. The creature is a live creature of the map's campaign (the
-- API checks it through the characters module); deleting the map or the
-- creature deletes the token, and a dismissed creature's token is not listed.
-- No personal data: IDs and a position.
CREATE TABLE IF NOT EXISTS map_creature_tokens (
    map_id UUID NOT NULL REFERENCES maps (id) ON DELETE CASCADE,
    creature_id UUID NOT NULL REFERENCES character_creatures (id) ON DELETE CASCADE,
    x_bp INT4 NOT NULL,
    y_bp INT4 NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT map_creature_tokens_pkey PRIMARY KEY (map_id, creature_id),
    CONSTRAINT map_creature_tokens_position_valid CHECK (x_bp BETWEEN 0 AND 10000 AND y_bp BETWEEN 0 AND 10000)
);

-- +goose Down
DROP TABLE IF EXISTS map_creature_tokens;
