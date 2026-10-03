-- +goose Up
-- The Etapa 8 session events (ADR-0007): a clue the master revealed to
-- players (clue_revealed, MR-029) and the NPCs on the stage changing
-- (stage_changed, MR-031). Their payloads hold IDs only, never a name nor the
-- clue's text. The CHECK lists every kind of the history (see 00063): dropped,
-- if present, and added again, so re-running this migration is safe.
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
        'scene_check_rolled',
        'clue_revealed',
        'stage_changed'
    ));

-- +goose Down
DELETE FROM session_events WHERE kind IN ('clue_revealed', 'stage_changed');

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
