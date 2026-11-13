-- +goose Up
-- feature_state holds the marks of a combatant's features that last a turn, a round or a combat ("once per turn"), as ONE JSON
-- object {"<feature key>": {"s": "turn"|"round"|"combat", "at": "<mark>", "n": <uses, at least 1>, "c": "<choice>"}} ("c" only
-- when there is one). An entry is live only while its mark matches now (the turn "<round>:<combatant on turn>", the round
-- number, or always for a combat), so nothing resets it: a stale entry reads as empty and is dropped on the next write. The
-- column replaces sneak_attack_turn, colossus_slayer_turn (00215) and table_once_turns (00264): they stay in the table, copied
-- below, and nothing reads or writes them any more. No personal data.
--
-- The column has a constant default and the statement adds only what is missing (IF NOT EXISTS). The check is its own named
-- constraint, dropped first, so the migration can run twice (an inline CHECK on ADD COLUMN IF NOT EXISTS adds a second
-- constraint on the rerun in CockroachDB).
ALTER TABLE combatants
    ADD COLUMN IF NOT EXISTS feature_state JSONB NOT NULL DEFAULT '{}';
ALTER TABLE combatants DROP CONSTRAINT IF EXISTS combatants_feature_state_valid;
ALTER TABLE combatants ADD CONSTRAINT combatants_feature_state_valid
    CHECK (jsonb_typeof(feature_state) = 'object' AND octet_length(feature_state::text) <= 8192);

-- The marks the old columns hold become entries of scope turn with one use: the table's features under their own key,
-- Sneak Attack under 'sneak_attack' and Colossus Slayer under 'colossus_slayer'. Only rows with a mark and no state yet
-- are touched, so a second run changes nothing.
UPDATE combatants
SET feature_state = (
    SELECT COALESCE(jsonb_object_agg(m.k, jsonb_build_object('s', 'turn', 'at', m.v, 'n', 1)), '{}'::jsonb)
    FROM (
        SELECT t.key AS k, t.value AS v
        FROM jsonb_each_text(CASE WHEN jsonb_typeof(combatants.table_once_turns) = 'object' THEN combatants.table_once_turns ELSE '{}'::jsonb END) AS t
        UNION ALL
        SELECT 'sneak_attack', combatants.sneak_attack_turn WHERE combatants.sneak_attack_turn IS NOT NULL
        UNION ALL
        SELECT 'colossus_slayer', combatants.colossus_slayer_turn WHERE combatants.colossus_slayer_turn IS NOT NULL
    ) AS m
)
WHERE feature_state = '{}'::jsonb
  AND (table_once_turns <> '{}'::jsonb OR sneak_attack_turn IS NOT NULL OR colossus_slayer_turn IS NOT NULL);

-- A character used a feature that declares a use (UseFeature, `feature_used`): the payload holds the combatant, the round, the
-- feature's key, the choice's key, the targets, what it healed and gave, the effect rows it made and what the undo puts back
-- (the economy, the feature state, the hit points). IDs, keys and numbers only. The row is secret when a hidden combatant is in it.
INSERT INTO session_event_kinds (kind) VALUES
    ('feature_used')
ON CONFLICT (kind) DO NOTHING;

-- +goose Down
DELETE FROM session_events WHERE kind = 'feature_used';
DELETE FROM session_event_kinds WHERE kind = 'feature_used';
ALTER TABLE combatants DROP CONSTRAINT IF EXISTS combatants_feature_state_valid;
ALTER TABLE combatants
    DROP COLUMN IF EXISTS feature_state;
