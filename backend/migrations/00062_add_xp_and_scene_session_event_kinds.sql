-- +goose Up
-- The XP awards and the RP scenes are session events too (ADR-0007, Etapa 7):
-- an XP award and its undo, a milestone, a scene opened or closed and a scene
-- check rolled. Their payloads hold IDs and numbers only, never a name or the
-- master's words. The CHECK lists every kind of the history (see 00058), the
-- six new ones of the whole Etapa 7 together: dropped, if present, and added
-- again, so re-running this migration is safe.
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
        'conditions_set',
        'xp_awarded',
        'xp_award_undone',
        'milestone_marked',
        'scene_opened',
        'scene_closed',
        'scene_check_rolled'
    ));

-- +goose Down
DELETE FROM session_events WHERE kind IN (
    'xp_awarded', 'xp_award_undone', 'milestone_marked', 'scene_opened', 'scene_closed', 'scene_check_rolled'
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
        'action_undone',
        'spell_cast',
        'reaction_used',
        'reaction_declined',
        'death_save_rolled',
        'death_confirmed',
        'conditions_set'
    ));
