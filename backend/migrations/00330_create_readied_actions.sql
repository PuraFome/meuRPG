-- +goose Up
-- A readied action (SRD 5.1, "Ready"): the combatant spends its action on its turn to act later, as a reaction, when
-- a trigger it names happens. One row for each readied action:
--   - trigger_text is the player's own words (1 to 200 characters); the master judges when it happens.
--   - kind is what is held: 'attack' (action_key is the attack's key), 'spell' (spell_cast_id names the held cast),
--     'move' (move_ft is the speed it may move) or 'other' (action_key is the standard action's key).
--   - action_name_pt is the held action's name when it was readied, so every view names it as the log line does.
--   - target_id is the creature the player named, if any; it is forgotten when that combatant leaves.
--   - status: 'waiting' until it ends; 'fired' (the master fired it and the reaction ran it), 'ignored' (the reactor let
--     it pass), 'lost' (the readier's next turn started first), 'broken' (the concentration of a held spell ended),
--     'canceled' (the owner gave it up on their own turn). A readier that is defeated loses it then: its turns are
--     skipped, so the start of its next turn would never come.
-- A combatant has at most one waiting row. The rows go with the combat and with the combatant.
-- Production is live: the migration only adds, and every statement may run twice.
CREATE TABLE IF NOT EXISTS readied_actions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    encounter_id UUID NOT NULL REFERENCES encounters (id) ON DELETE CASCADE,
    combatant_id UUID NOT NULL REFERENCES combatants (id) ON DELETE CASCADE,
    round INT4 NOT NULL,
    trigger_text TEXT NOT NULL,
    kind TEXT NOT NULL,
    action_key TEXT NULL,
    action_name_pt TEXT NOT NULL DEFAULT '',
    spell_cast_id TEXT NULL,
    move_ft INT4 NOT NULL DEFAULT 0,
    target_id UUID NULL REFERENCES combatants (id) ON DELETE SET NULL,
    status TEXT NOT NULL DEFAULT 'waiting',
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT readied_actions_trigger_valid CHECK (char_length(trigger_text) BETWEEN 1 AND 200),
    CONSTRAINT readied_actions_kind_valid CHECK (kind IN ('attack', 'spell', 'move', 'other')),
    CONSTRAINT readied_actions_status_valid CHECK (status IN ('waiting', 'fired', 'ignored', 'lost', 'broken', 'canceled')),
    CONSTRAINT readied_actions_action_key_valid CHECK (kind NOT IN ('attack', 'other') OR action_key IS NOT NULL),
    CONSTRAINT readied_actions_round_move_valid CHECK (round >= 0 AND move_ft >= 0)
);

CREATE INDEX IF NOT EXISTS readied_actions_encounter_id_idx ON readied_actions (encounter_id);
CREATE INDEX IF NOT EXISTS readied_actions_combatant_id_idx ON readied_actions (combatant_id);
CREATE INDEX IF NOT EXISTS readied_actions_target_id_idx ON readied_actions (target_id);
CREATE UNIQUE INDEX IF NOT EXISTS readied_actions_one_waiting_idx ON readied_actions (combatant_id) WHERE status = 'waiting';

-- The master fires a readied action in a reaction window of its own kind: the list of 00308 plus 'readied'.
-- Dropped first by name, so the migration can run twice; it only widens the CHECK, so every row stays valid.
ALTER TABLE reaction_windows
    DROP CONSTRAINT IF EXISTS reaction_windows_kind_valid,
    ADD CONSTRAINT reaction_windows_kind_valid CHECK (kind IN (
        'shield', 'uncanny_dodge', 'hellish_rebuke', 'counterspell', 'cutting_words',
        'deflect_missiles', 'feather_fall', 'concentration_save', 'master_check', 'effect_save', 'contest',
        'maneuver_reduce', 'protection', 'legendary_resistance', 'parry', 'slow_fall', 'readied'
    ));

-- The events of a readied action: it was readied (action_readied: the row, the trigger and the turn's economy before,
-- which the master's undo puts back), fired (readied_fired) and lost or given up (readied_lost).
INSERT INTO session_event_kinds (kind) VALUES
    ('action_readied'),
    ('readied_fired'),
    ('readied_lost')
ON CONFLICT (kind) DO NOTHING;

-- +goose Down
DELETE FROM session_events WHERE kind IN ('action_readied', 'readied_fired', 'readied_lost');
DELETE FROM session_event_kinds WHERE kind IN ('action_readied', 'readied_fired', 'readied_lost');
DELETE FROM reaction_windows WHERE kind = 'readied';
ALTER TABLE reaction_windows
    DROP CONSTRAINT IF EXISTS reaction_windows_kind_valid,
    ADD CONSTRAINT reaction_windows_kind_valid CHECK (kind IN (
        'shield', 'uncanny_dodge', 'hellish_rebuke', 'counterspell', 'cutting_words',
        'deflect_missiles', 'feather_fall', 'concentration_save', 'master_check', 'effect_save', 'contest',
        'maneuver_reduce', 'protection', 'legendary_resistance', 'parry', 'slow_fall'
    ));
DROP TABLE IF EXISTS readied_actions;
