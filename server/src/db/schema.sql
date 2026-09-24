-- meuRPG backend schema (CockroachDB / PostgreSQL compatible).
-- Idempotent: safe to re-run (CREATE TABLE IF NOT EXISTS), so `npm run migrate`
-- can be executed multiple times without erroring.

-- CockroachDB ships pgcrypto, which guarantees gen_random_uuid() is available.
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

-- Bearer sessions: sha256(token) -> user, so sessions survive process
-- restarts and can be revoked per-token on logout.
CREATE TABLE IF NOT EXISTS auth_sessions (
  token_hash text PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL
);

-- Characters mirror the Character interface (src/app/core/models/character.ts).
-- Flexible fields (attributes, skills, inventory, quotes, sheet, minion) are
-- stored as JSONB, with scalar metadata in dedicated columns.
CREATE TABLE IF NOT EXISTS characters (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  master_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
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

-- Frequently queried by type (master dashboard filters) and by owner
-- (every read is scoped to the logged-in user).
CREATE INDEX IF NOT EXISTS idx_characters_type ON characters (type);

-- Reusable share-link tokens: a visitor redeems one to create a player
-- character owned by themselves and co-visible to the token's creator.
CREATE TABLE IF NOT EXISTS character_join_tokens (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  character_id uuid REFERENCES characters(id) ON DELETE SET NULL,
  created_by uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token text UNIQUE NOT NULL,
  type text NOT NULL DEFAULT 'player',
  created_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL
);

-- Expiry filtering powers token cleanup sweeps.
CREATE INDEX IF NOT EXISTS idx_character_join_tokens_expires_at ON character_join_tokens (expires_at);

-- ---------------------------------------------------------------------------
-- Live-table migrations (additive + idempotent). CREATE TABLE IF NOT EXISTS
-- above only covers fresh databases, so these ALTERs bring pre-existing
-- installs to the owner-aware shape without destroying already-owned rows.
-- The user explicitly chose to discard legacy ownerless characters.
-- ---------------------------------------------------------------------------
ALTER TABLE characters ADD COLUMN IF NOT EXISTS user_id uuid REFERENCES users(id) ON DELETE CASCADE;
DELETE FROM characters WHERE user_id IS NULL;
ALTER TABLE characters ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE character_join_tokens ADD COLUMN IF NOT EXISTS created_by uuid REFERENCES users(id) ON DELETE CASCADE;
DELETE FROM character_join_tokens WHERE created_by IS NULL;
ALTER TABLE character_join_tokens ALTER COLUMN created_by SET NOT NULL;
CREATE INDEX IF NOT EXISTS idx_characters_user_id ON characters (user_id);

-- Visitor role + shared ownership: a character created through a join link is
-- owned by the invited player (`user_id`) but co-visible to the inviter
-- (`master_user_id`), so both lists show it.
ALTER TABLE users ADD COLUMN IF NOT EXISTS role text NOT NULL DEFAULT 'master';
ALTER TABLE characters ADD COLUMN IF NOT EXISTS master_user_id uuid REFERENCES users(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_characters_master_user_id ON characters (master_user_id);

