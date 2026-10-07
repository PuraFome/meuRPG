-- +goose Up
-- One row per create_key (which carries its owner's ID): the unique index is what makes the
-- idempotency hold when two calls with the same key race. Rows without a key are left out of the
-- index. See 00178.
CREATE UNIQUE INDEX IF NOT EXISTS campaigns_create_key_idx ON campaigns (create_key) WHERE create_key IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS maps_create_key_idx ON maps (create_key) WHERE create_key IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS scene_actions_create_key_idx ON scene_actions (create_key) WHERE create_key IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS scene_clues_create_key_idx ON scene_clues (create_key) WHERE create_key IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS player_notes_create_key_idx ON player_notes (create_key) WHERE create_key IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS game_sessions_create_key_idx ON game_sessions (create_key) WHERE create_key IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS puzzles_create_key_idx ON puzzles (create_key) WHERE create_key IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS planned_milestones_create_key_idx ON planned_milestones (create_key) WHERE create_key IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS campaign_content_create_key_idx ON campaign_content (create_key) WHERE create_key IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS character_creatures_create_key_idx ON character_creatures (create_key) WHERE create_key IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS campaigns_create_key_idx;
DROP INDEX IF EXISTS maps_create_key_idx;
DROP INDEX IF EXISTS scene_actions_create_key_idx;
DROP INDEX IF EXISTS scene_clues_create_key_idx;
DROP INDEX IF EXISTS player_notes_create_key_idx;
DROP INDEX IF EXISTS game_sessions_create_key_idx;
DROP INDEX IF EXISTS puzzles_create_key_idx;
DROP INDEX IF EXISTS planned_milestones_create_key_idx;
DROP INDEX IF EXISTS campaign_content_create_key_idx;
DROP INDEX IF EXISTS character_creatures_create_key_idx;
