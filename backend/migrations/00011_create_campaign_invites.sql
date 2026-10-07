-- +goose Up
-- campaign_invites holds the invite links a master shares so that players
-- can join a campaign (MR-002, RN-07).
--
-- The link carries a random 32-byte token in its fragment (/convite#t=...);
-- the database keeps only its SHA-256 (token_hash), so reading this table
-- does not let anyone join a campaign.
--
-- An invite works while all of these hold: revoked_at is NULL, use_count <
-- max_uses and expires_at is in the future. Defaults (1 use, 7 days) and
-- limits live in the application while RN-07 is open; the CHECKs keep the
-- invariants: use_count never passes max_uses, even under concurrent
-- AcceptInvite calls, and no invite lives longer than 30 days.
--
-- Rows are deleted by CockroachDB's row-level TTL 30 days after the invite
-- expires (docs/privacy.md). Queries still filter on expires_at: the TTL
-- job runs once a day. Deleting the campaign, or the account that created
-- the invite, deletes it too. There is no index on created_by, for the same
-- reason as campaigns.created_by.
CREATE TABLE IF NOT EXISTS campaign_invites (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id UUID NOT NULL REFERENCES campaigns (id) ON DELETE CASCADE,
    token_hash BYTEA NOT NULL,
    created_by UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    -- INT4, because CockroachDB's INT is 64 bits and these are small counts.
    max_uses INT4 NOT NULL,
    use_count INT4 NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ NULL,
    CONSTRAINT campaign_invites_token_hash_key UNIQUE (token_hash),
    CONSTRAINT campaign_invites_token_hash_length CHECK (octet_length(token_hash) = 32),
    CONSTRAINT campaign_invites_uses CHECK (max_uses >= 1 AND use_count >= 0 AND use_count <= max_uses),
    CONSTRAINT campaign_invites_lifetime CHECK (expires_at > created_at AND expires_at <= created_at + INTERVAL '30 days')
) WITH (ttl_expiration_expression = 'expires_at + INTERVAL ''30 days''');

-- +goose Down
DROP TABLE IF EXISTS campaign_invites;
