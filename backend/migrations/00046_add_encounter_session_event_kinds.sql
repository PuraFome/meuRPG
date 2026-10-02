-- +goose Up
-- The combat's changes are session_events too (ADR-0007): one kind for each
-- thing the master or a player does to an encounter (MR-013). Their
-- payloads hold IDs and numbers only, never a name. The CHECK grows with
-- every kind of the history: dropped, if present, and added again, so
-- re-running this migration is safe.
ALTER TABLE session_events
    DROP CONSTRAINT IF EXISTS session_events_kind_valid,
    ADD CONSTRAINT session_events_kind_valid CHECK (kind IN (
        'character_vitals_adjusted',
        'encounter_started',
        'initiative_submitted',
        'initiative_order_set',
        'combat_begun',
        'turn_ended',
        'combatant_moved',
        'combatant_hidden_set',
        'combatants_added',
        'combatant_removed',
        'encounter_ended'
    ));

-- +goose Down
DELETE FROM session_events WHERE kind <> 'character_vitals_adjusted';

ALTER TABLE session_events
    DROP CONSTRAINT IF EXISTS session_events_kind_valid,
    ADD CONSTRAINT session_events_kind_valid CHECK (kind IN ('character_vitals_adjusted'));
