-- +goose Up
-- A player character with neither a player nor a campaign is deleted by
-- CockroachDB's row-level TTL job, on its own (docs/privacidade.md).
--
-- A player character stays when its player deletes their account (RN-16:
-- it stays with the campaign's master) and when its campaign is deleted (it
-- stays with its player). When both are gone, nobody can reach it any more,
-- through any RPC, yet it still holds the free text its player wrote. Such a
-- row is kept for no purpose, so it must go.
--
-- The expiration expression is NULL, meaning "never expires", for every
-- other row. For an orphan it is created_at, a time that is always in the
-- past, so the row is expired from the moment it is orphaned and the next
-- run of the TTL job (once a day, CockroachDB's default) deletes it. Its
-- master's notes went with the campaign already (character_master_notes
-- cascades from campaigns).
--
-- ALTER TABLE ... SET only sets the table's storage parameters, so running
-- it twice leaves the table as it was. The Down turns row-level TTL off.
--
-- Migrations 00021 and later are for MR-024 (invites with approval).
ALTER TABLE characters SET (
    ttl_expiration_expression = 'CASE WHEN kind = ''player'' AND player_user_id IS NULL AND campaign_id IS NULL THEN created_at END'
);

-- +goose Down
ALTER TABLE characters RESET (ttl);
