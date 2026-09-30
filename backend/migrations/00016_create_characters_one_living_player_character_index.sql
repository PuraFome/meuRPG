-- +goose Up
-- RN-03: inside a campaign, a player has at most one living character. They
-- create a new one only after the current one dies, and the dead one stays.
--
-- The application checks this first, inside the transaction, to answer with
-- a clear error; this index is what makes it true even when two requests
-- race. The predicate is status <> 'dead', not status = 'active', so a
-- character waiting for the master's approval (status 'pending', MR-024)
-- also counts as living.
--
-- Rows with a NULL player_user_id (the player deleted their account, RN-16)
-- or a NULL campaign_id (the campaign was deleted) never conflict: a unique
-- index treats NULLs as different from each other.
CREATE UNIQUE INDEX IF NOT EXISTS characters_one_living_player_character
    ON characters (campaign_id, player_user_id)
    WHERE kind = 'player' AND status <> 'dead';

-- +goose Down
DROP INDEX IF EXISTS characters_one_living_player_character;
