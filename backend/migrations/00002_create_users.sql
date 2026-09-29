-- +goose Up
-- users is the account itself. It holds no personal data: how a person signs
-- in lives in other tables (user_identities today; the player sign-in of
-- ADR-0009 later), so an account can have more than one way in.
--
-- Every table that points at users declares ON DELETE explicitly, so that
-- deleting an account (docs/privacidade.md) removes everything tied to it.
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS users;
