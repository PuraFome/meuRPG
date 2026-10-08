-- +goose Up
-- taken is what a settled damage of a spell cost its target once temporary hit
-- points had absorbed their share (what a concentrating target's Constitution
-- save is set from, RN-22). A spell with two damage types opens one pending
-- damage per type, and the target gets one concentration save for the whole
-- cast: the sum of the taken of its pending damages. NULL until the damage lands,
-- and again after an undo.
ALTER TABLE pending_damages
    ADD COLUMN IF NOT EXISTS taken INT4;

-- +goose Down
ALTER TABLE pending_damages
    DROP COLUMN IF EXISTS taken;
