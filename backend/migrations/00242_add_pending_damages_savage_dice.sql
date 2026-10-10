-- +goose Up
-- savage_dice is how many of the extra_dice of a critical hit come from the
-- half-orc's Savage Attacks (SRD 5.1: one more weapon die on a melee critical),
-- 0 or 1. The rest of extra_dice is the barbarian's Brutal Critical. The damage
-- sheet names each source apart.
ALTER TABLE pending_damages
    ADD COLUMN IF NOT EXISTS savage_dice INT4 NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE pending_damages
    DROP COLUMN IF EXISTS savage_dice;
