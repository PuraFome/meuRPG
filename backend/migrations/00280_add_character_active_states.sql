-- +goose Up
-- The temporary states that are on for a character (a table feature's `state` effect: a stance,
-- a form), as a JSON list of {"key": "<state key>", "rounds_left": N} in the order they began.
-- The key is the state's feature key, "#" and the state's name; rounds_left is how many rounds
-- of the owner's turns are left (absent: it lasts until it ends). A combat starts a state and
-- counts its rounds down; a rest, an incapacity, the end of the combat or the master end it.
-- Keys, numbers and nothing else; no personal data.
--
-- It is a column of the character, not of character_vitals, because the sheet's numbers
-- (armor class, speed, attacks) depend on it and every read of the sheet already reads the
-- character; the vitals view and revision follow it (the code bumps the vitals revision in
-- the same transaction, as it does for the Wild Shape form).
--
-- Production is live: the column only adds, with every row as it was ('[]'), so running the
-- migration twice is safe.
ALTER TABLE characters
    ADD COLUMN IF NOT EXISTS active_states JSONB NOT NULL DEFAULT '[]',
    DROP CONSTRAINT IF EXISTS characters_active_states_valid,
    ADD CONSTRAINT characters_active_states_valid CHECK (jsonb_typeof(active_states) = 'array' AND jsonb_array_length(active_states) <= 32);

-- +goose Down
ALTER TABLE characters
    DROP CONSTRAINT IF EXISTS characters_active_states_valid,
    DROP COLUMN IF EXISTS active_states;
