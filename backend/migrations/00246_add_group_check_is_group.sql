-- +goose Up
-- A request for a check may ask only some characters, and each roll may be judged alone.
-- is_group says whether the request has a group verdict (SRD 5.1, "Working Together":
-- at least half of the characters asked pass). The requests that exist are all group
-- checks, so the default is true: the code before this column keeps working.
ALTER TABLE group_checks ADD COLUMN IF NOT EXISTS is_group BOOLEAN NOT NULL DEFAULT true;

-- +goose Down
ALTER TABLE group_checks DROP COLUMN IF EXISTS is_group;
