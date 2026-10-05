-- +goose Up
-- A druid in Wild Shape (MR-037, Etapa 9, D7): the form is part of the
-- character's vitals as far as the API goes (CharacterVitals.wild_shape), because
-- it lasts from one session to the next as the hit points do (the master ends it:
-- the app does not count its hours). It has a table of its own, not columns of
-- character_vitals, for a reason of locking: a combat's write changes the
-- vitals row (a slot spent, hit points) and then reads the character's sheet
-- through another connection, and that read now needs the form to give the
-- beast's armor class; if the form were on the vitals row, the read would wait for
-- the write of the very transaction that asked for it. Changing the form also
-- bumps character_vitals.revision (the API's "newer wins"), which the code does
-- in the same transaction.
--
-- One row per character in a beast form, none in its own shape. beast is the
-- SRD beast's content key ("monster:wolf") and hp the beast's current hit points,
-- a pool of their own (the maximum is never stored: it is the stat block's). A
-- beast at 0 hit points is no longer the form, so hp is 1 or more. The character's
-- own hit points (character_vitals.hit_points_current) wait untouched and take the
-- damage left over when the beast falls. Deleting the character deletes the row.
-- IDs, a key and a number only.
CREATE TABLE IF NOT EXISTS character_wild_shapes (
    character_id UUID PRIMARY KEY REFERENCES characters (id) ON DELETE CASCADE,
    beast TEXT NOT NULL,
    hp INT4 NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT character_wild_shapes_beast_length CHECK (char_length(beast) BETWEEN 1 AND 100),
    CONSTRAINT character_wild_shapes_hp_valid CHECK (hp >= 1)
);

-- +goose Down
DROP TABLE IF EXISTS character_wild_shapes;
