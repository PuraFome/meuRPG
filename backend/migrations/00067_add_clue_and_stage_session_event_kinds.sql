-- +goose Up
-- The stage and the clues are session events too (ADR-0007, MR-031, MR-029,
-- Etapa 8): the master put an NPC on the stage, took it off, changed the
-- speaker or cleared the stage (stage_changed), and revealed a clue
-- (clue_revealed). Their payloads hold IDs only, never a name nor the master's
-- words. The CHECK lists every kind of the history (see 00063): dropped, if
-- present, and added again, so re-running this migration is safe. Etapa 8's
-- slices each add their kind; this migration carries both.
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
