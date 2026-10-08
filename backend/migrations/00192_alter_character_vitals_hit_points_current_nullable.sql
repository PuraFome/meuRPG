-- +goose Up
-- A character whose hit points were never set has full hit points, whatever its
-- maximum becomes: the column is NULL until the master or a combat sets the
-- value, so a first write of something else (temporary hit points, a slot, the
-- Wild Shape form, the familiar's eyes) does not pin it to the maximum of that day.
ALTER TABLE character_vitals
    ALTER COLUMN hit_points_current DROP NOT NULL;

-- +goose Down
-- The value is cut to the maximum when read, so the largest one means "full".
UPDATE character_vitals SET hit_points_current = 2147483647 WHERE hit_points_current IS NULL;
ALTER TABLE character_vitals
    ALTER COLUMN hit_points_current SET NOT NULL;
