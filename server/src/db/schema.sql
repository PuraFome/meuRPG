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
