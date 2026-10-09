-- name: CharacterIsInOpenCombat :one
-- Whether the character is a combatant (dismissed or not) of the campaign's combat that
-- is not ended. A character cannot go back to the reserve (MR-049) while one is.
SELECT EXISTS (
    SELECT 1 FROM combatants AS cb
    JOIN encounters AS e ON e.id = cb.encounter_id
    JOIN game_sessions AS gs ON gs.id = e.game_session_id
    WHERE gs.campaign_id = sqlc.arg(campaign_id)::UUID
      AND e.status <> 'ended'
      AND cb.character_id = sqlc.arg(character_id)::UUID
);
