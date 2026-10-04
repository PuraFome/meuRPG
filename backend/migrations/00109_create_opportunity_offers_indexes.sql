-- +goose Up
-- The offers that wait in a combat (the view, the turn that waits, the master's
-- skip).
CREATE INDEX IF NOT EXISTS opportunity_offers_encounter_id_pending_idx
    ON opportunity_offers (encounter_id)
    WHERE state = 'pending';

-- The offers of one move (its undo) and the offers on one mover or reactor (the
-- foreign keys' ON DELETE CASCADE).
CREATE INDEX IF NOT EXISTS opportunity_offers_move_id_idx
    ON opportunity_offers (move_id);

CREATE INDEX IF NOT EXISTS opportunity_offers_mover_id_idx
    ON opportunity_offers (mover_id);

CREATE INDEX IF NOT EXISTS opportunity_offers_reactor_id_idx
    ON opportunity_offers (reactor_id);

-- The offer a pending damage belongs to (the 0 hit points rule) and the foreign
-- key's ON DELETE SET NULL.
CREATE INDEX IF NOT EXISTS opportunity_offers_attack_pending_id_idx
    ON opportunity_offers (attack_pending_id)
    WHERE attack_pending_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS opportunity_offers_attack_pending_id_idx;
DROP INDEX IF EXISTS opportunity_offers_reactor_id_idx;
DROP INDEX IF EXISTS opportunity_offers_mover_id_idx;
DROP INDEX IF EXISTS opportunity_offers_move_id_idx;
DROP INDEX IF EXISTS opportunity_offers_encounter_id_pending_idx;
