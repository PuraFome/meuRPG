-- +goose Up
-- The charges a character's magic items have spent (SRD 5.1: wands, staffs and rings hold charges that come back
-- at dawn). One row for each entry of the sheet's magic_items (entry_id, "m<n>") that spent a charge or was lost;
-- the maximum comes from the catalog, never from here. The row goes with the character, and the sheet's save
-- deletes the rows of entries it removed, so a new entry that takes a freed id starts full.
CREATE TABLE IF NOT EXISTS character_item_charges (
    character_id UUID NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    entry_id TEXT NOT NULL,
    charges_used INT NOT NULL DEFAULT 0,
    destroyed BOOLEAN NOT NULL DEFAULT false,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (character_id, entry_id),
    CONSTRAINT character_item_charges_used_valid CHECK (charges_used >= 0)
);

-- +goose Down
DROP TABLE IF EXISTS character_item_charges;
