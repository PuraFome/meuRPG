-- +goose Up
-- oidc_login_states remembers a sign-in that has started but not finished:
-- one row per visit to /auth/login, deleted when the provider sends the
-- browser back to /auth/callback. It lives at most 10 minutes (expires_at).
--
-- state_hash is the SHA-256 of the "state" sent to the provider; the state
-- itself also goes into a short-lived cookie, which ties the row to the
-- browser that started the sign-in. code_verifier (PKCE) and nonce must be
-- read back as they are, so they are stored in the clear, for minutes only.
--
-- Abandoned rows are deleted by the row-level TTL job, which runs every hour
-- here instead of the default once a day.
CREATE TABLE IF NOT EXISTS oidc_login_states (
    state_hash BYTEA PRIMARY KEY,
    code_verifier TEXT NOT NULL,
    nonce TEXT NOT NULL,
    return_to TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT oidc_login_states_state_hash_length CHECK (octet_length(state_hash) = 32),
    CONSTRAINT oidc_login_states_lifetime CHECK (expires_at > created_at AND expires_at <= created_at + INTERVAL '10 minutes')
) WITH (ttl_expiration_expression = 'expires_at', ttl_job_cron = '@hourly');

-- +goose Down
DROP TABLE IF EXISTS oidc_login_states;
