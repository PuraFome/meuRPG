-- +goose Up
-- In a combat without a grid (MR-025, RN-25) the master offers an opportunity
-- attack by hand: nobody left a square, so the offer has none (both columns
-- NULL; a combat on a grid always sets both). A withdrawn offer (the master took
-- it back before it was answered) is a state of its own. The named CHECKs are
-- dropped, if present, and added again, so re-running this migration is safe.
ALTER TABLE opportunity_offers
    ALTER COLUMN left_col DROP NOT NULL,
    ALTER COLUMN left_row DROP NOT NULL;

ALTER TABLE opportunity_offers
    DROP CONSTRAINT IF EXISTS opportunity_offers_square_valid,
    ADD CONSTRAINT opportunity_offers_square_valid CHECK (
        (left_col IS NULL AND left_row IS NULL) OR (left_col >= 0 AND left_row >= 0)
    ),
    DROP CONSTRAINT IF EXISTS opportunity_offers_state_valid,
    ADD CONSTRAINT opportunity_offers_state_valid CHECK (state IN ('pending', 'attacked', 'declined', 'skipped', 'withdrawn'));

-- +goose Down
-- Down is a development tool: the offers the master made by hand have no square,
-- so they cannot go back into the old columns and are deleted; a withdrawn offer
-- becomes the closest old state, a skipped one.
UPDATE opportunity_offers SET state = 'skipped' WHERE state = 'withdrawn';
DELETE FROM opportunity_offers WHERE left_col IS NULL OR left_row IS NULL;

ALTER TABLE opportunity_offers
    DROP CONSTRAINT IF EXISTS opportunity_offers_square_valid,
    DROP CONSTRAINT IF EXISTS opportunity_offers_state_valid,
    ADD CONSTRAINT opportunity_offers_state_valid CHECK (state IN ('pending', 'attacked', 'declined', 'skipped')),
    ADD CONSTRAINT opportunity_offers_square_valid CHECK (left_col >= 0 AND left_row >= 0);

ALTER TABLE opportunity_offers
    ALTER COLUMN left_col SET NOT NULL,
    ALTER COLUMN left_row SET NOT NULL;
