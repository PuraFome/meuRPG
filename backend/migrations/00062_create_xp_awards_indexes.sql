-- +goose Up
-- A campaign's awards, newest first: the history (ListXPAwards) and the one
-- the master may undo (the newest not undone). It also serves the ON DELETE
-- CASCADE when the campaign is deleted.
CREATE INDEX IF NOT EXISTS xp_awards_campaign_id_created_at_idx
    ON xp_awards (campaign_id, created_at DESC, id DESC);

-- A retry of the same award finds it by the app's key.
CREATE UNIQUE INDEX IF NOT EXISTS xp_awards_campaign_id_idempotency_key_idx
    ON xp_awards (campaign_id, idempotency_key);

-- One 'enemies' award per encounter, unless the master undid it. The service
-- checks first, to answer with a clear error; this index is what makes it
-- true when two awards race.
CREATE UNIQUE INDEX IF NOT EXISTS xp_awards_one_per_encounter
    ON xp_awards (encounter_id)
    WHERE mode = 'enemies' AND undone_at IS NULL;

-- Which awards marked a character, for the "pode subir de nível" tag, and the
-- ON DELETE CASCADE when a character is deleted.
CREATE INDEX IF NOT EXISTS xp_award_shares_character_id_idx
    ON xp_award_shares (character_id);

-- +goose Down
DROP INDEX IF EXISTS xp_award_shares_character_id_idx;
DROP INDEX IF EXISTS xp_awards_one_per_encounter;
DROP INDEX IF EXISTS xp_awards_campaign_id_idempotency_key_idx;
DROP INDEX IF EXISTS xp_awards_campaign_id_created_at_idx;
