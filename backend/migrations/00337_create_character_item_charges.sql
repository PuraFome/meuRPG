-- +goose Up
-- The charges a character's magic items have spent (SRD 5.1: wands, staffs and rings hold charges that come back
-- at dawn). One row for each entry of the sheet's magic_items (entry_id, "m<n>") that spent a charge, was set by the
-- master (an item whose charges the SRD rolls has no row until then) or was lost;
-- the maximum comes from the catalog, never from here. The row goes with the character, and the sheet's save
-- deletes the rows of entries it removed, so a new entry that takes a freed id starts full.
CREATE TABLE IF NOT EXISTS character_item_charges (
    character_id UUID NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    entry_id TEXT NOT NULL,
    charges_used INT NOT NULL DEFAULT 0,
    lost TEXT NOT NULL DEFAULT '',
    dawns_waited INT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (character_id, entry_id),
    CONSTRAINT character_item_charges_used_valid CHECK (charges_used >= 0),
    CONSTRAINT character_item_charges_lost_valid CHECK (lost IN ('', 'gone', 'plain', 'partly')),
    CONSTRAINT character_item_charges_dawns_valid CHECK (dawns_waited >= 0)
);

-- +goose Down
DROP TABLE IF EXISTS character_item_charges;
