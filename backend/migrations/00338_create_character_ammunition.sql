-- +goose Up
-- The ammunition a player character holds: one row for each kind (ammo_key, a catalog key such as equipment:arrow)
-- the character has had.
-- Each attack with an ammunition weapon spends one piece.
-- spent_last_combat counts the pieces fired in the combat last_encounter_id.
-- recovered says the half that can be picked up after that combat was taken back.
-- The row goes with the character, and a sheet's save never touches it.
CREATE TABLE IF NOT EXISTS character_ammunition (
    character_id UUID NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    ammo_key TEXT NOT NULL,
    quantity INT NOT NULL DEFAULT 0,
    spent_last_combat INT NOT NULL DEFAULT 0,
    last_encounter_id UUID NULL,
    recovered BOOLEAN NOT NULL DEFAULT false,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (character_id, ammo_key),
    CONSTRAINT character_ammunition_quantity_valid CHECK (quantity >= 0),
    CONSTRAINT character_ammunition_spent_valid CHECK (spent_last_combat >= 0)
);

-- The charges and the ammunition spent are session events, so the key of a retry finds what it did.
INSERT INTO session_event_kinds (kind) VALUES
    ('item_charge_spent'),
    ('item_charges_restored'),
    ('item_destroyed'),
    ('ammunition_spent'),
    ('ammunition_recovered')
ON CONFLICT (kind) DO NOTHING;

-- +goose Down
DELETE FROM session_events WHERE kind IN ('item_charge_spent', 'item_charges_restored', 'item_destroyed', 'ammunition_spent', 'ammunition_recovered');
DELETE FROM session_event_kinds WHERE kind IN ('item_charge_spent', 'item_charges_restored', 'item_destroyed', 'ammunition_spent', 'ammunition_recovered');
DROP TABLE IF EXISTS character_ammunition;
