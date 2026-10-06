-- +goose Up
-- create_key is the idempotency key of "Criar NPC" (CharacterService.
-- CreateNpcFromCreature, MR-042): a UUID the app sends once per dialog. A retry
-- with the same key finds the NPC the first call made instead of creating a
-- second one. NULL for every other character (the other creates have no key).
-- It is an opaque ID, not personal data.
ALTER TABLE characters
    ADD COLUMN IF NOT EXISTS create_key UUID NULL;

-- Adding a column makes CockroachDB rewrite the table's TTL expression (00020)
-- in its own normal form, so a second run of the migrations, which sets the
-- expression of 00020 again and finds the column there, would leave the table
-- looking different (TestMigrationsAreSafeToRerun). Setting the same expression
-- here, last, gives the same table on every run.
ALTER TABLE characters SET (
    ttl_expiration_expression = 'CASE WHEN kind = ''player'' AND player_user_id IS NULL AND campaign_id IS NULL THEN created_at END'
);

-- +goose Down
ALTER TABLE characters DROP COLUMN IF EXISTS create_key;
