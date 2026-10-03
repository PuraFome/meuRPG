-- +goose Up
-- The spells, the reactions, the death saves and the conditions are session
-- events too (ADR-0007, MR-014, RN-03, RN-22): a spell cast, an Escudo used or
-- declined, a death save, the master confirming a death, and the conditions or
-- the concentration marked. Their payloads hold IDs and numbers only, never a
-- name. The CHECK lists every kind of the history (see 00051): dropped, if
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
        'action_undone',
        'spell_cast',
        'reaction_used',
        'reaction_declined',
        'death_save_rolled',
        'death_confirmed',
        'conditions_set'
    ));

-- +goose Down
DELETE FROM session_events WHERE kind IN (
    'spell_cast', 'reaction_used', 'reaction_declined', 'death_save_rolled', 'death_confirmed', 'conditions_set'
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
        'encounter_ended',
        'attack_rolled',
        'damage_rolled',
        'damage_applied',
        'damage_discarded',
        'action_taken',
        'hit_points_adjusted',
        'action_undone'
    ));
