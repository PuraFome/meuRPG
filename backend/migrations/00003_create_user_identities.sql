-- +goose Up
-- user_identities links an OpenID Connect account to a MeuRPG account.
--
-- (issuer, subject) is how OpenID Connect identifies a person: "sub" is
-- unique and never reassigned only within one issuer. E-mail is not an
-- identifier; it can change and be reused.
--
-- email is optional and only a security contact (incident notices, data
-- subject requests). It is filled only when the provider says it is
-- verified, it is refreshed on every sign-in, and it is never shown to other
-- users. Name and photo are never stored (docs/privacy.md).
--
-- UNIQUE (user_id, issuer): an account has at most one identity per
-- provider, so two Google accounts are never merged into one (ADR-0009).
CREATE TABLE IF NOT EXISTS user_identities (
    issuer TEXT NOT NULL,
    subject TEXT NOT NULL,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    email TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (issuer, subject),
    CONSTRAINT user_identities_user_id_issuer_key UNIQUE (user_id, issuer),
    -- OpenID Connect Core 1.0, section 2: sub is at most 255 ASCII characters.
    CONSTRAINT user_identities_subject_length CHECK (length(subject) BETWEEN 1 AND 255)
);

-- +goose Down
DROP TABLE IF EXISTS user_identities;
