-- +goose Up
-- combatants are who fights in an encounter (MR-013): the party's player
-- characters and copies of NPCs ("Goblin 1", "Goblin 2"; RN-19: each copy
-- has its own initiative). A combatant belongs to one encounter and stands
-- for one character; several copies of the same NPC share its character_id.
--
-- label is what the table calls it, 1 to 40 characters. kind is 'player' or
-- 'npc'. user_id is the player of a player combatant (SET NULL when the
-- account is deleted: the character stays with the master, RN-16). hidden is
-- the master's switch (RN-10, RN-20): the server never sends a hidden
-- combatant to a player. A new NPC starts hidden (question 31).
--
-- Initiative (RN-19): initiative is the total, d20 + initiative_bonus;
-- initiative_face is the d20 (typed from a physical die, or rolled by the
-- app); both NULL until rolled. order_index is the combatant's place in the
-- turn order, from 0, kept in step with the initiatives by the service.
-- tie_ordered says the master decided the order among combatants tied on
-- the same total and bonus (SetInitiativeOrder).
--
-- Position (RN-21): grid_col and grid_row are a square of the encounter's
-- grid, from 0; both NULL while the combatant is not placed. speed_ft is
-- the walking speed copied when the combatant joins (sheets of players are
-- locked during a session; the master's edit of an NPC does not move a
-- fight already going). movement_used_ft counts the feet walked this turn;
-- dashed (the Dash action) doubles the speed for the turn.
--
-- The turn's economy: action_used, bonus_action_used, reaction_used.
--
-- Only an NPC has hit points here (hp_current, hp_max, hp_temp): a player
-- character's live in character_vitals, one source of truth (RN-02, RN-04:
-- combat never changes the NPC's sheet). defeated, the death saves and the
-- condition labels (RN-22) come with the actions of the next slice.
-- concentration_spell is the spell label the master or the player marks.
--
-- Deleting the encounter, or the character, deletes the combatant. No
-- personal data besides user_id: fiction, numbers and flags.
CREATE TABLE IF NOT EXISTS combatants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    encounter_id UUID NOT NULL REFERENCES encounters (id) ON DELETE CASCADE,
    character_id UUID NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    user_id UUID NULL REFERENCES users (id) ON DELETE SET NULL,
    label TEXT NOT NULL,
    kind TEXT NOT NULL,
    hidden BOOL NOT NULL,
    initiative INT4 NULL,
    initiative_bonus INT4 NOT NULL,
    initiative_face INT4 NULL,
    tie_ordered BOOL NOT NULL DEFAULT false,
    order_index INT4 NOT NULL,
    grid_col INT4 NULL,
    grid_row INT4 NULL,
    speed_ft INT4 NOT NULL,
    movement_used_ft INT4 NOT NULL DEFAULT 0,
    dashed BOOL NOT NULL DEFAULT false,
    action_used BOOL NOT NULL DEFAULT false,
    bonus_action_used BOOL NOT NULL DEFAULT false,
    reaction_used BOOL NOT NULL DEFAULT false,
    hp_current INT4 NULL,
    hp_max INT4 NULL,
    hp_temp INT4 NULL,
    defeated BOOL NOT NULL DEFAULT false,
    death_successes INT4 NOT NULL DEFAULT 0,
    death_failures INT4 NOT NULL DEFAULT 0,
    conditions TEXT[] NOT NULL DEFAULT '{}',
    concentration_spell TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT combatants_label_length CHECK (char_length(label) BETWEEN 1 AND 40),
    CONSTRAINT combatants_kind_valid CHECK (kind IN ('player', 'npc')),
    CONSTRAINT combatants_initiative_valid CHECK (
        (initiative IS NULL) = (initiative_face IS NULL) AND (initiative_face IS NULL OR initiative_face BETWEEN 1 AND 20)
    ),
    CONSTRAINT combatants_initiative_bonus_valid CHECK (initiative_bonus BETWEEN -20 AND 40),
    CONSTRAINT combatants_order_index_valid CHECK (order_index >= 0),
    CONSTRAINT combatants_position_valid CHECK (
        (grid_col IS NULL) = (grid_row IS NULL) AND (grid_col IS NULL OR (grid_col >= 0 AND grid_row >= 0))
    ),
    CONSTRAINT combatants_speed_valid CHECK (speed_ft BETWEEN 0 AND 600),
    CONSTRAINT combatants_movement_used_valid CHECK (movement_used_ft >= 0),
    CONSTRAINT combatants_hit_points_valid CHECK (
        (kind = 'player' AND hp_current IS NULL AND hp_max IS NULL AND hp_temp IS NULL)
        OR (
            kind = 'npc' AND hp_max IS NOT NULL AND hp_current IS NOT NULL AND hp_temp IS NOT NULL
            AND hp_max >= 1 AND hp_current BETWEEN 0 AND hp_max AND hp_temp >= 0
        )
    ),
    CONSTRAINT combatants_death_saves_valid CHECK (death_successes BETWEEN 0 AND 3 AND death_failures BETWEEN 0 AND 3),
    CONSTRAINT combatants_conditions_valid CHECK (COALESCE(array_length(conditions, 1), 0) <= 20),
    CONSTRAINT combatants_concentration_length CHECK (concentration_spell IS NULL OR char_length(concentration_spell) BETWEEN 1 AND 80)
);

-- +goose Down
DROP TABLE IF EXISTS combatants;
