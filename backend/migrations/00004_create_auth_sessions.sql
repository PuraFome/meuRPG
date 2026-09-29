-- +goose Up
-- auth_sessions holds the server side of each sign-in session (ADR-0002).
--
-- The browser keeps a random 32-byte token in the __Host-meurpg_session
-- cookie; the database keeps only its SHA-256 (token_hash), so reading this
-- table does not let anyone sign in. Revoking a session is deleting its row.
--
-- Sessions end at expires_at, at most 30 days after created_at, and are never
-- extended (NIST SP 800-63B-4, AAL1). Queries must still filter on
-- expires_at: the row-level TTL job below deletes expired rows only once a
-- day.
--
-- auth_time is the provider's auth_time claim, when it sends one. It is kept
-- for auditing only; expires_at never depends on it.
CREATE TABLE IF NOT EXISTS auth_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    token_hash BYTEA NOT NULL,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    auth_time TIMESTAMPTZ NULL,
    -- Every authenticated request looks a session up by token_hash. INCLUDE
    -- (STORING, in CockroachDB's words) lets that lookup read only this index.
    -- The index on user_id is in 00006: sqlc reads these files with the
    -- PostgreSQL parser, which has no inline INDEX inside CREATE TABLE.
    CONSTRAINT auth_sessions_token_hash_key UNIQUE (token_hash) INCLUDE (user_id, created_at, expires_at),
    CONSTRAINT auth_sessions_token_hash_length CHECK (octet_length(token_hash) = 32),
    CONSTRAINT auth_sessions_lifetime CHECK (expires_at > created_at AND expires_at <= created_at + INTERVAL '30 days')
) WITH (ttl_expiration_expression = 'expires_at');

-- +goose Down
DROP TABLE IF EXISTS auth_sessions;
