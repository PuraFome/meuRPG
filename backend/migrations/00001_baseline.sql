-- Baseline: the schema as the NestJS app (server/src/db/schema.sql) leaves it
-- today, written as its final shape.
--
-- Every statement uses IF NOT EXISTS, so on the existing production database,
-- where these objects already exist, this migration changes nothing and only
-- records version 1 in goose_db_version. On a fresh database it creates
-- everything. The legacy backfill statements from schema.sql (DELETE ...
-- WHERE user_id IS NULL, ALTER ... SET NOT NULL, the maps_kind_check swap)
-- are intentionally left out: they only fixed up older installs.
--
-- From here on, change the schema only by adding new numbered migrations;
-- never edit a migration that has already run in production.

-- +goose Up

-- gen_random_uuid() is built into CockroachDB (and PostgreSQL 13+); the
-- extension line is kept so the baseline matches schema.sql exactly.
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS users (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  google_sub text UNIQUE NOT NULL,
  email text,
  name text,
  role text NOT NULL DEFAULT 'master',
  consent_at timestamptz,
  created_at timestamptz DEFAULT now()
);

-- Short-lived OAuth handshake state: PKCE verifier + nonce + state secret,
-- keyed by an id embedded in the OAuth `state` parameter.
CREATE TABLE IF NOT EXISTS oauth_handshakes (
  id text PRIMARY KEY,
  state_secret text NOT NULL,
  nonce text NOT NULL,
  code_verifier text NOT NULL,
  expires_at timestamptz NOT NULL
);

-- Bearer sessions: sha256(token) -> user, so sessions survive restarts and
-- can be revoked one by one on logout.
CREATE TABLE IF NOT EXISTS auth_sessions (
  token_hash text PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL
);

-- A character is owned by user_id. When it was created through a join link,
-- master_user_id is the inviting master, who can see it too.
-- Flexible fields are JSONB; scalar metadata has its own columns.
CREATE TABLE IF NOT EXISTS characters (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  master_user_id uuid REFERENCES users (id) ON DELETE SET NULL,
  type text NOT NULL CHECK (type IN ('npc', 'player', 'boss', 'minion')),
  name text NOT NULL,
  email text,
  description text NOT NULL DEFAULT '',
  image_url text,
  history text,
  master_notes text,
  attributes jsonb NOT NULL DEFAULT '{}',
  skills jsonb NOT NULL DEFAULT '[]',
  inventory jsonb NOT NULL DEFAULT '[]',
  quotes jsonb NOT NULL DEFAULT '[]',
  sheet jsonb,
  minion jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_characters_type ON characters (type);
CREATE INDEX IF NOT EXISTS idx_characters_user_id ON characters (user_id);
CREATE INDEX IF NOT EXISTS idx_characters_master_user_id ON characters (master_user_id);

-- Reusable share-link tokens: a visitor redeems one to create a player
-- character owned by themselves and visible to the token's creator.
CREATE TABLE IF NOT EXISTS character_join_tokens (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  character_id uuid REFERENCES characters (id) ON DELETE SET NULL,
  created_by uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  token text UNIQUE NOT NULL,
  type text NOT NULL DEFAULT 'player',
  created_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL
);

-- Expiry filtering powers token cleanup sweeps.
CREATE INDEX IF NOT EXISTS idx_character_join_tokens_expires_at ON character_join_tokens (expires_at);

-- Maps: structured fields are JSONB. background_image holds a base64 data
-- URL, so it can be large.
CREATE TABLE IF NOT EXISTS maps (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  name text NOT NULL,
  description text,
  kind text NOT NULL DEFAULT 'world',
  background_image text,
  width integer NOT NULL DEFAULT 1024,
  height integer NOT NULL DEFAULT 768,
  grid jsonb NOT NULL DEFAULT '{}',
  fog_of_war jsonb NOT NULL DEFAULT '{}',
  layers jsonb NOT NULL DEFAULT '[]',
  markers jsonb NOT NULL DEFAULT '[]',
  submaps jsonb NOT NULL DEFAULT '[]',
  dungeon jsonb NOT NULL DEFAULT '{}',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  -- Named explicitly: it is the name schema.sql uses when it widens the
  -- check, so future migrations can refer to it on every database.
  CONSTRAINT maps_kind_check CHECK (kind IN ('world', 'city', 'dungeon', 'local'))
);

CREATE INDEX IF NOT EXISTS idx_maps_user_id ON maps (user_id);

-- +goose Down

-- Reverse dependency order: tables that reference others go first.
-- Dropping a table also drops its indexes and constraints.
DROP TABLE IF EXISTS maps;
DROP TABLE IF EXISTS character_join_tokens;
DROP TABLE IF EXISTS characters;
DROP TABLE IF EXISTS auth_sessions;
DROP TABLE IF EXISTS oauth_handshakes;
DROP TABLE IF EXISTS users;
