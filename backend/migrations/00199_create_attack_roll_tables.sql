-- +goose Up
-- The tables of the attack rolls (advantage and disadvantage, the damage extras and
-- the states of a combatant). They hold IDs, keys, numbers and the short reasons a
-- player or the master writes; none of it goes to session_events (docs/privacy.md).
--
-- roll_mode_requests are a player's request for a better mode than the one the
-- server suggested for an attack (SRD 5.1, "Advantage and Disadvantage": the master
-- decides). combatant_id is who wants to attack, target_id the target and attack_key
-- the attack. suggested_mode, requested_mode and decided_mode are 'normal',
-- 'advantage' or 'disadvantage' (decided_mode is NULL while 'pending'). status is
-- 'pending' (waits for the master), 'answered' (decided_mode is the mode the attack
-- rolls with) or 'closed' (taken back, used by the attack, or the turn ended). reason
-- is the player's, 1 to 120 characters, fiction: the screen warns "é ficção; não
-- escreva dados reais de pessoas". Deleting the encounter or a combatant deletes it.
CREATE TABLE IF NOT EXISTS roll_mode_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    encounter_id UUID NOT NULL REFERENCES encounters (id) ON DELETE CASCADE,
    combatant_id UUID NOT NULL REFERENCES combatants (id) ON DELETE CASCADE,
    target_id UUID NOT NULL REFERENCES combatants (id) ON DELETE CASCADE,
    attack_key TEXT NOT NULL,
    suggested_mode TEXT NOT NULL,
    requested_mode TEXT NOT NULL,
    decided_mode TEXT NULL,
    status TEXT NOT NULL,
    reason TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    answered_at TIMESTAMPTZ NULL,
    CONSTRAINT roll_mode_requests_status_valid CHECK (status IN ('pending', 'answered', 'closed')),
    CONSTRAINT roll_mode_requests_modes_valid CHECK (
        suggested_mode IN ('normal', 'advantage', 'disadvantage')
        AND requested_mode IN ('normal', 'advantage', 'disadvantage')
        AND (decided_mode IS NULL OR decided_mode IN ('normal', 'advantage', 'disadvantage'))
    ),
    CONSTRAINT roll_mode_requests_reason_length CHECK (char_length(reason) BETWEEN 1 AND 120),
    CONSTRAINT roll_mode_requests_key_length CHECK (char_length(attack_key) BETWEEN 1 AND 100)
);

CREATE INDEX IF NOT EXISTS roll_mode_requests_encounter_id_idx ON roll_mode_requests (encounter_id);
CREATE INDEX IF NOT EXISTS roll_mode_requests_combatant_id_idx ON roll_mode_requests (combatant_id);
CREATE INDEX IF NOT EXISTS roll_mode_requests_target_id_idx ON roll_mode_requests (target_id);

-- combat_reasons are the short texts of a combat's log lines: why a roll's mode is not
-- the server's suggestion (kind 'roll_mode'), and why the master took an extra out of a
-- damage (kind 'part_removed'). The session event holds the row's ID, never the text.
-- reason is 1 to 120 characters, fiction. Deleting the encounter deletes it.
CREATE TABLE IF NOT EXISTS combat_reasons (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    encounter_id UUID NOT NULL REFERENCES encounters (id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    reason TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT combat_reasons_kind_valid CHECK (kind IN ('roll_mode', 'part_removed')),
    CONSTRAINT combat_reasons_reason_length CHECK (char_length(reason) BETWEEN 1 AND 120)
);

CREATE INDEX IF NOT EXISTS combat_reasons_encounter_id_idx ON combat_reasons (encounter_id);

-- combatant_states are the states a combatant is in for a while: 'rage',
-- 'hunters_mark_target' (the target of a ranger's Hunter's Mark, source_id the
-- ranger), 'dodging' and 'reckless'. ends_combatant_id, ends_phase ('start_of_turn' or
-- 'end_of_turn') and ends_round say when it ends: at that moment of that combatant's
-- turn, in that round when ends_round is set (a rage lasts 10 rounds); all NULL when
-- it ends with something else (a concentration). started_round is the round it began.
-- amount is the number the state carries (a rage's damage bonus), 0 for the others.
-- Deleting the encounter or a combatant deletes it.
CREATE TABLE IF NOT EXISTS combatant_states (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    encounter_id UUID NOT NULL REFERENCES encounters (id) ON DELETE CASCADE,
    combatant_id UUID NOT NULL REFERENCES combatants (id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    source_id UUID NULL REFERENCES combatants (id) ON DELETE CASCADE,
    ends_combatant_id UUID NULL REFERENCES combatants (id) ON DELETE CASCADE,
    ends_phase TEXT NULL,
    ends_round INT4 NULL,
    started_round INT4 NOT NULL,
    amount INT4 NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT combatant_states_kind_valid CHECK (kind IN ('rage', 'hunters_mark_target', 'dodging', 'reckless')),
    CONSTRAINT combatant_states_phase_valid CHECK (ends_phase IS NULL OR ends_phase IN ('start_of_turn', 'end_of_turn')),
    CONSTRAINT combatant_states_rounds_valid CHECK (started_round >= 0 AND (ends_round IS NULL OR ends_round >= started_round))
);

CREATE INDEX IF NOT EXISTS combatant_states_encounter_id_idx ON combatant_states (encounter_id);
CREATE INDEX IF NOT EXISTS combatant_states_combatant_id_idx ON combatant_states (combatant_id);
CREATE INDEX IF NOT EXISTS combatant_states_source_id_idx ON combatant_states (source_id);
CREATE INDEX IF NOT EXISTS combatant_states_ends_combatant_id_idx ON combatant_states (ends_combatant_id);

-- +goose Down
DROP TABLE IF EXISTS combatant_states;
DROP TABLE IF EXISTS combat_reasons;
DROP TABLE IF EXISTS roll_mode_requests;
