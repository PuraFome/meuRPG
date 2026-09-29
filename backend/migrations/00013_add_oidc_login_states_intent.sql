-- +goose Up
-- intent_kind and intent_data remember what the user asked to finish right
-- after signing in, such as accepting a campaign invite: the app posts the
-- intent and its payload to POST /auth/login, and the callback completes it
-- once the session exists. Both are NULL for a plain sign-in.
--
-- intent_kind names the handler (today only campaign_invite). intent_data is
-- what that handler prepared from the payload at login start: for an invite,
-- the token's SHA-256, never the token itself (docs/privacidade.md). Like the
-- rest of the row, it lives at most 10 minutes and the callback deletes it.
--
-- One statement with several parts, so re-running it is safe: ADD COLUMN IF
-- NOT EXISTS skips a column that exists, and the named CHECK is dropped, if
-- present, and added again (see 00007).
ALTER TABLE oidc_login_states
    ADD COLUMN IF NOT EXISTS intent_kind TEXT NULL,
    ADD COLUMN IF NOT EXISTS intent_data BYTEA NULL,
    DROP CONSTRAINT IF EXISTS oidc_login_states_intent,
    ADD CONSTRAINT oidc_login_states_intent CHECK (
        (intent_kind IS NULL AND intent_data IS NULL)
        OR (
            -- IS NOT NULL on both, because a CHECK that evaluates to NULL
            -- passes: without them, a data with no kind would get through.
            intent_kind IS NOT NULL AND intent_data IS NOT NULL
            AND char_length(intent_kind) BETWEEN 1 AND 32
            AND octet_length(intent_data) <= 256
        )
    );

-- +goose Down
ALTER TABLE oidc_login_states
    DROP CONSTRAINT IF EXISTS oidc_login_states_intent,
    DROP COLUMN IF EXISTS intent_kind,
    DROP COLUMN IF EXISTS intent_data;
