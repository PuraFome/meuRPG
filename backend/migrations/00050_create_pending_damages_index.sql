-- +goose Up
-- An encounter's pending damage (the ones still open when a turn ends), and
-- the ON DELETE CASCADE when the encounter is deleted.
CREATE INDEX IF NOT EXISTS pending_damages_encounter_id_idx
    ON pending_damages (encounter_id);

-- +goose Down
DROP INDEX IF EXISTS pending_damages_encounter_id_idx;
