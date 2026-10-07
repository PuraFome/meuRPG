-- +goose Up
-- create_key is the idempotency key of the create RPCs that had none (the audit of 07/10/2026,
-- F7): the owner's ID (the user's, the campaign's, or the campaign's and the user's), a colon
-- and the 1 to 64 characters the app chose once for the action, so a key is unique in its owner.
-- A retry with the same key finds the row the first call made instead of making a second.
-- create_hash is the hash of the whole request, minus the key: a retry with the same key and
-- another request is refused. Both are NULL for a row made without a key (a call that sent
-- none, which is still allowed) and for every row made before. Both are opaque, not personal
-- data. map_points has them since 00163.
ALTER TABLE campaigns ADD COLUMN IF NOT EXISTS create_key TEXT NULL;
ALTER TABLE campaigns ADD COLUMN IF NOT EXISTS create_hash TEXT NULL;
ALTER TABLE maps ADD COLUMN IF NOT EXISTS create_key TEXT NULL;
ALTER TABLE maps ADD COLUMN IF NOT EXISTS create_hash TEXT NULL;
ALTER TABLE scene_actions ADD COLUMN IF NOT EXISTS create_key TEXT NULL;
ALTER TABLE scene_actions ADD COLUMN IF NOT EXISTS create_hash TEXT NULL;
ALTER TABLE scene_clues ADD COLUMN IF NOT EXISTS create_key TEXT NULL;
ALTER TABLE scene_clues ADD COLUMN IF NOT EXISTS create_hash TEXT NULL;
ALTER TABLE player_notes ADD COLUMN IF NOT EXISTS create_key TEXT NULL;
ALTER TABLE player_notes ADD COLUMN IF NOT EXISTS create_hash TEXT NULL;
ALTER TABLE game_sessions ADD COLUMN IF NOT EXISTS create_key TEXT NULL;
ALTER TABLE game_sessions ADD COLUMN IF NOT EXISTS create_hash TEXT NULL;
ALTER TABLE puzzles ADD COLUMN IF NOT EXISTS create_key TEXT NULL;
ALTER TABLE puzzles ADD COLUMN IF NOT EXISTS create_hash TEXT NULL;
ALTER TABLE planned_milestones ADD COLUMN IF NOT EXISTS create_key TEXT NULL;
ALTER TABLE planned_milestones ADD COLUMN IF NOT EXISTS create_hash TEXT NULL;
ALTER TABLE campaign_content ADD COLUMN IF NOT EXISTS create_key TEXT NULL;
ALTER TABLE campaign_content ADD COLUMN IF NOT EXISTS create_hash TEXT NULL;
ALTER TABLE character_creatures ADD COLUMN IF NOT EXISTS create_key TEXT NULL;
ALTER TABLE character_creatures ADD COLUMN IF NOT EXISTS create_hash TEXT NULL;

-- +goose Down
ALTER TABLE campaigns DROP COLUMN IF EXISTS create_hash;
ALTER TABLE campaigns DROP COLUMN IF EXISTS create_key;
ALTER TABLE maps DROP COLUMN IF EXISTS create_hash;
ALTER TABLE maps DROP COLUMN IF EXISTS create_key;
ALTER TABLE scene_actions DROP COLUMN IF EXISTS create_hash;
ALTER TABLE scene_actions DROP COLUMN IF EXISTS create_key;
ALTER TABLE scene_clues DROP COLUMN IF EXISTS create_hash;
ALTER TABLE scene_clues DROP COLUMN IF EXISTS create_key;
ALTER TABLE player_notes DROP COLUMN IF EXISTS create_hash;
ALTER TABLE player_notes DROP COLUMN IF EXISTS create_key;
ALTER TABLE game_sessions DROP COLUMN IF EXISTS create_hash;
ALTER TABLE game_sessions DROP COLUMN IF EXISTS create_key;
ALTER TABLE puzzles DROP COLUMN IF EXISTS create_hash;
ALTER TABLE puzzles DROP COLUMN IF EXISTS create_key;
ALTER TABLE planned_milestones DROP COLUMN IF EXISTS create_hash;
ALTER TABLE planned_milestones DROP COLUMN IF EXISTS create_key;
ALTER TABLE campaign_content DROP COLUMN IF EXISTS create_hash;
ALTER TABLE campaign_content DROP COLUMN IF EXISTS create_key;
ALTER TABLE character_creatures DROP COLUMN IF EXISTS create_hash;
ALTER TABLE character_creatures DROP COLUMN IF EXISTS create_key;
