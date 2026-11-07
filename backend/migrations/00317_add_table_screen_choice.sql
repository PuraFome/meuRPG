-- +goose Up
-- The master's choice for the TV at the table (issue #648): follow what the table sees (tv_mirror) or one
-- pinned item (tv_pin, a TableScreenKind: 1 idle, 2 map, 3 image, 4 scene, 5 combat). tv_choice_revision
-- counts the master's writes of the choice, so a stale write is refused. The choice stays when its item ends.
-- A flag and two small numbers; no personal data.
--
-- Production is live: the columns only add, with every row as it was (mirror on, pin idle, revision 0),
-- so running the migration twice is safe.
ALTER TABLE game_sessions
    ADD COLUMN IF NOT EXISTS tv_mirror BOOLEAN NOT NULL DEFAULT true,
    ADD COLUMN IF NOT EXISTS tv_pin SMALLINT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS tv_choice_revision INT NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE game_sessions
    DROP COLUMN IF EXISTS tv_choice_revision,
    DROP COLUMN IF EXISTS tv_pin,
    DROP COLUMN IF EXISTS tv_mirror;
