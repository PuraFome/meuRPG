-- +goose Up
-- character_ability_rolls keeps the six sets of "4d6, drop the lowest" the
-- server made for a player's next new character in a campaign (MR-025, RN-24),
-- so asking again returns the same sets: nobody rerolls by reloading the page.
-- One row per player and campaign. CreateCharacter deletes the row when the
-- sheet it creates uses the sets, so the next character gets new ones.
--
-- sets is the JSON list of six sets, each the four dice of the roll, such as
-- [[6,5,5,2],[5,5,4,1],...] (the lowest die of a set is the one dropped, and
-- the result of a set is the other three added). source says who rolled the
-- dice: 'app' (the server's random source) or 'typed' (the player's own
-- dice, typed once, in a campaign with physical dice, RN-18). Only numbers.
--
-- Deleting the campaign or the user deletes the rows.
CREATE TABLE IF NOT EXISTS character_ability_rolls (
    campaign_id UUID NOT NULL REFERENCES campaigns (id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    sets JSONB NOT NULL,
    source TEXT NOT NULL,
    rolled_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (campaign_id, user_id),
    CONSTRAINT character_ability_rolls_source_valid CHECK (source IN ('app', 'typed'))
);

-- +goose Down
DROP TABLE IF EXISTS character_ability_rolls;
