-- +goose Up
-- Reserved characters and the links that hand them to players (MR-049).
--
-- A reserved character is a player character the master made (or imported) for a
-- player to take: kind 'player', no owner (player_user_id NULL) and
-- reserved = true. It is invisible to every player until it is claimed (RN-10): the
-- queries that list "the party" leave reserved characters out, and a player's own
-- reads find nothing, since the character has no owner. A character whose player
-- deleted their account has no owner either (RN-16), but it is not reserved: it
-- stays visible as it always did.
--
-- claimed_at is when a player took the character through a link, and stays while
-- they own it. It is what lets the master give the character back to the reserve
-- ("Devolver à reserva") and what the master's list uses to keep a claimed character
-- next to the reserved ones. It does not depend on the link row, which is deleted
-- 30 days after it ends.
--
-- The CHECK keeps the shape even if application code has a bug: a reserved character
-- is a living player character of the master's, with no owner.
--
-- One statement with several parts, so running it twice is safe: ADD COLUMN IF NOT
-- EXISTS skips a column that exists, and the named CHECK is dropped, if present, and
-- added again (see 00022).
ALTER TABLE characters
    ADD COLUMN IF NOT EXISTS reserved BOOL NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS claimed_at TIMESTAMPTZ NULL,
    DROP CONSTRAINT IF EXISTS characters_reserved_shape,
    ADD CONSTRAINT characters_reserved_shape CHECK (
        NOT reserved OR (kind = 'player' AND player_user_id IS NULL AND status = 'active' AND claimed_at IS NULL)
    );

-- Adding a column makes CockroachDB rewrite the table's TTL expression (00020) in its
-- own normal form; setting the same expression again, last, keeps a second run from
-- leaving the table looking different (TestMigrationsAreSafeToRerun), as 00179 does.
ALTER TABLE characters SET (
    ttl_expiration_expression = 'CASE WHEN kind = ''player'' AND player_user_id IS NULL AND campaign_id IS NULL THEN created_at END'
);

-- claim_links holds the links a master sends so that a player can take a reserved
-- character (package characters, ClaimCharacter).
--
-- The link is https://<app>/claim#t=<token>. The token is 32 random bytes that the
-- server returns once, to the master; the database keeps only its SHA-256
-- (token_hash), so reading this table does not let anyone take a character. The token
-- sits after the #, which browsers never send to a server, and the app posts it in a
-- request body (ADR-0009, docs/privacy.md), exactly like an invite.
--
-- A link works while used_at and revoked_at are NULL and expires_at is in the future.
-- It lives 1, 7 or 30 days (the CHECK keeps the 30) and once. A character has at most
-- one live link (claim_links_one_live_per_character): making another revokes the old
-- one in the same transaction. A used link keeps used_by, the player who took the
-- character; their deleted account sets it to NULL.
--
-- Rows are deleted by CockroachDB's row-level TTL 30 days after the link ended (used,
-- revoked, or expired, whichever applies; docs/privacy.md). Queries still filter on
-- the times: the TTL job runs once a day. Deleting the campaign, the character or the
-- account that made the link deletes it too.
CREATE TABLE IF NOT EXISTS claim_links (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id UUID NOT NULL REFERENCES campaigns (id) ON DELETE CASCADE,
    character_id UUID NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    token_hash BYTEA NOT NULL,
    created_by UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ NULL,
    used_at TIMESTAMPTZ NULL,
    used_by UUID NULL REFERENCES users (id) ON DELETE SET NULL,
    CONSTRAINT claim_links_token_hash_key UNIQUE (token_hash),
    CONSTRAINT claim_links_token_hash_length CHECK (octet_length(token_hash) = 32),
    CONSTRAINT claim_links_lifetime CHECK (expires_at > created_at AND expires_at <= created_at + INTERVAL '30 days'),
    CONSTRAINT claim_links_used_or_revoked CHECK (used_at IS NULL OR revoked_at IS NULL)
) WITH (ttl_expiration_expression = 'COALESCE(used_at, revoked_at, expires_at) + INTERVAL ''30 days''');

-- At most one live link per character, even if two CreateClaimLink calls race.
CREATE UNIQUE INDEX IF NOT EXISTS claim_links_one_live_per_character
    ON claim_links (character_id) WHERE revoked_at IS NULL AND used_at IS NULL;

-- The latest link of a character (the master's list, RevokeClaimLink).
CREATE INDEX IF NOT EXISTS claim_links_character_id_idx ON claim_links (character_id, created_at DESC);

-- claim_sign_ins keeps, for ten minutes, the claim link a person opened while signed
-- out and then signed in with (sign-in intent character_claim). Signing in never
-- claims: this row only lets the page come back to the card without the link's token,
-- which is gone from the address bar. Like the login state that carried it, it holds
-- only the token's SHA-256. One row per person: a newer link replaces the older one.
-- A used row is deleted by the claim; the TTL clears the rest.
CREATE TABLE IF NOT EXISTS claim_sign_ins (
    user_id UUID PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    token_hash BYTEA NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT claim_sign_ins_token_hash_length CHECK (octet_length(token_hash) = 32)
) WITH (ttl_expiration_expression = 'expires_at');

-- +goose Down
DROP TABLE IF EXISTS claim_sign_ins;
DROP TABLE IF EXISTS claim_links;
ALTER TABLE characters
    DROP CONSTRAINT IF EXISTS characters_reserved_shape,
    DROP COLUMN IF EXISTS claimed_at,
    DROP COLUMN IF EXISTS reserved;
