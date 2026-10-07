-- +goose Up
-- create_hash is the hash of a whole CreateCharacter request, kept next to
-- create_key (00122: a UUID, unique in the campaign) so a retry with the same key and another
-- request is refused (the audit of 07/10/2026, F7). NULL for a character made without a key and
-- for the NPCs of "Criar NPC" and of the combat, which have no request to compare. Opaque, not
-- personal data.
ALTER TABLE characters
    ADD COLUMN IF NOT EXISTS create_hash TEXT NULL;

-- Adding a column makes CockroachDB rewrite the table's TTL expression (00020) in its own normal
-- form; setting the same expression again, last, keeps a second run of the migrations from
-- leaving the table looking different (TestMigrationsAreSafeToRerun), as 00122 does.
ALTER TABLE characters SET (
    ttl_expiration_expression = 'CASE WHEN kind = ''player'' AND player_user_id IS NULL AND campaign_id IS NULL THEN created_at END'
);

-- +goose Down
ALTER TABLE characters DROP COLUMN IF EXISTS create_hash;
