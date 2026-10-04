-- +goose Up
-- An opportunity offer is the right a combatant gets when an enemy leaves its
-- reach (MR-034, RN-21, question 69): the mover's move lands at once, and each
-- reactor that could hit it is offered one attack, which its controller takes,
-- declines, or the master skips (the player is offline). The mover's turn waits
-- while one is 'pending'.
--
-- mover_id and reactor_id are combatants of the encounter; left_col and left_row
-- are the square of the move's line where the mover was last inside the
-- reactor's reach (it goes back there if the attack drops it to 0 hit points).
-- move_id is shared by the offers one move made: the master's undo of that move
-- deletes them. state is 'pending', 'attacked' (the reactor attacked, and
-- attack_pending_id is the damage that attack opened, while it still exists),
-- 'declined' or 'skipped' (the master passed over it, or ended the mover's
-- turn). answered_at is when it left 'pending'.
--
-- Deleting the encounter or either combatant deletes the row. No personal data:
-- IDs, squares and a state.
CREATE TABLE IF NOT EXISTS opportunity_offers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    encounter_id UUID NOT NULL REFERENCES encounters (id) ON DELETE CASCADE,
    move_id UUID NOT NULL,
    mover_id UUID NOT NULL REFERENCES combatants (id) ON DELETE CASCADE,
    reactor_id UUID NOT NULL REFERENCES combatants (id) ON DELETE CASCADE,
    left_col INT4 NOT NULL,
    left_row INT4 NOT NULL,
    state TEXT NOT NULL DEFAULT 'pending',
    attack_pending_id UUID NULL REFERENCES pending_damages (id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL,
    answered_at TIMESTAMPTZ NULL,
    CONSTRAINT opportunity_offers_state_valid CHECK (state IN ('pending', 'attacked', 'declined', 'skipped')),
    CONSTRAINT opportunity_offers_square_valid CHECK (left_col >= 0 AND left_row >= 0),
    CONSTRAINT opportunity_offers_distinct CHECK (mover_id <> reactor_id)
);

-- +goose Down
DROP TABLE IF EXISTS opportunity_offers;
