-- +goose Up
-- The attacks and what follows them are session events too (ADR-0007, MR-012,
-- MR-014): the attack roll, the damage roll, the master's apply or discard,
-- a standard action, an NPC's hit points by hand, and the undo that takes one
-- back (a compensating row: the history keeps both). Their payloads hold IDs
-- and numbers only, never a name. The CHECK lists every kind of the history
-- (the vitals correction, the 6.3 combat kinds and these): dropped, if
-- present, and added again, so re-running this migration is safe.
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
        'encounter_ended',
        'attack_rolled',
        'damage_rolled',
        'damage_applied',
        'damage_discarded',
        'action_taken',
        'hit_points_adjusted',
        'action_undone'
    ));

-- +goose Down
DELETE FROM session_events WHERE kind IN (
    'attack_rolled', 'damage_rolled', 'damage_applied', 'damage_discarded',
    'action_taken', 'hit_points_adjusted', 'action_undone'
);

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
