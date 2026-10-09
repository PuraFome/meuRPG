-- +goose Up
-- Reviving the dead (RN-03): the master's "Reviver" and the spell Revivify.
--
-- A dead character is never deleted (RN-03), so a revival changes it back: status
-- 'active' again, with the state it had before it died (sheet_locked_at is never
-- touched by a death). The columns below keep what the master and the spell need.
--
--   revived_at     the last time it lived again; NULL if it never did.
--   died_at        stays after a revival, as the master's history, and a new death sets it
--                  again (MarkCharacterDead). Only a dead character has to have one, so the
--                  old CHECK (dead exactly when died_at is set) becomes "dead implies died_at".
--   death_round    the round of the combat in which the master confirmed the death, and
--   death_encounter_id  that combat; both NULL for a death marked outside a combat. The ID
--                  is not a foreign key: the characters module owns this table, and a combat
--                  that is gone leaves the round as a plain history.
--   revivify_blocked  the master's switch "Revivificar não funciona nesta morte" (old age, a
--                  missing body part: SRD 5.1, Revivify). It belongs to one death, so a revival
--                  and a new death clear it.
--   revive_key, revive_hash  the idempotency key of the last ReviveCharacter (scoped to the
--                  campaign and the caller) and the hash of its request.
--
-- combatants keep, for the creatures that die in a combat, the round and the place in the
-- order they died at (the 10 rounds of Revivify run out at that place, in the tenth round),
-- and the same switch for an NPC. They are NULL while the creature lives.
--
-- The session event kind 'character_revived' is the log line "O mestre reviveu <name>".
ALTER TABLE characters
    ADD COLUMN IF NOT EXISTS revived_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS death_round INT4 NULL,
    ADD COLUMN IF NOT EXISTS death_encounter_id UUID NULL,
    ADD COLUMN IF NOT EXISTS revivify_blocked BOOL NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS revive_key TEXT NULL,
    ADD COLUMN IF NOT EXISTS revive_hash TEXT NULL;

ALTER TABLE characters DROP CONSTRAINT IF EXISTS characters_dead_since;
ALTER TABLE characters DROP CONSTRAINT IF EXISTS characters_dead_has_died_at;
ALTER TABLE characters
    ADD CONSTRAINT characters_dead_has_died_at CHECK (status <> 'dead' OR died_at IS NOT NULL);

-- Adding a column makes CockroachDB rewrite the table's TTL expression (00020) in its own
-- normal form; setting the same expression again, last, keeps a second run of the migrations
-- from leaving the table looking different (as 00122 and 00179 do).
ALTER TABLE characters SET (
    ttl_expiration_expression = 'CASE WHEN kind = ''player'' AND player_user_id IS NULL AND campaign_id IS NULL THEN created_at END'
);

ALTER TABLE combatants
    ADD COLUMN IF NOT EXISTS death_round INT4 NULL,
    ADD COLUMN IF NOT EXISTS death_order_index INT4 NULL,
    ADD COLUMN IF NOT EXISTS revivify_blocked BOOL NOT NULL DEFAULT false;

INSERT INTO session_event_kinds (kind) VALUES
    ('character_revived')
ON CONFLICT (kind) DO NOTHING;

-- +goose Down
DELETE FROM session_events WHERE kind = 'character_revived';
DELETE FROM session_event_kinds WHERE kind = 'character_revived';
ALTER TABLE combatants
    DROP COLUMN IF EXISTS revivify_blocked,
    DROP COLUMN IF EXISTS death_order_index,
    DROP COLUMN IF EXISTS death_round;
UPDATE characters SET died_at = NULL WHERE status <> 'dead' AND died_at IS NOT NULL;
ALTER TABLE characters DROP CONSTRAINT IF EXISTS characters_dead_has_died_at;
ALTER TABLE characters
    ADD CONSTRAINT characters_dead_since CHECK ((status = 'dead') = (died_at IS NOT NULL));
ALTER TABLE characters
    DROP COLUMN IF EXISTS revive_hash,
    DROP COLUMN IF EXISTS revive_key,
    DROP COLUMN IF EXISTS revivify_blocked,
    DROP COLUMN IF EXISTS death_encounter_id,
    DROP COLUMN IF EXISTS death_round,
    DROP COLUMN IF EXISTS revived_at;
