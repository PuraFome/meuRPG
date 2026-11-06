-- +goose Up
-- The free object interaction of a combatant's turn (SRD 5.1, "Other Activity on Your Turn"): drawing or stowing
-- one weapon is free once a turn; a second one in the same turn, or a shield, costs the action. The first weapon
-- swap of the turn sets the flag; ResetCombatantTurn clears it at the start of the combatant's next turn.
-- A flag and nothing else; no personal data.
--
-- Production is live: the column only adds, with every row as it was (false), so running the migration twice is safe.
ALTER TABLE combatants
    ADD COLUMN IF NOT EXISTS object_interaction_used BOOLEAN NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE combatants
    DROP COLUMN IF EXISTS object_interaction_used;
