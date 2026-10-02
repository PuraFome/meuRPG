-- +goose Up
-- A session has at most one encounter that is not ended. The service checks
-- first, inside the transaction, to answer with a clear error; this index is
-- what makes it true when two starts race (as game_sessions_one_open_per_campaign).
CREATE UNIQUE INDEX IF NOT EXISTS encounters_one_open_per_session
    ON encounters (game_session_id)
    WHERE status <> 'ended';

-- A session's encounters, newest first (GetEncounter reads the latest), and
-- the ON DELETE CASCADE when a session is deleted.
CREATE INDEX IF NOT EXISTS encounters_game_session_id_created_at_idx
    ON encounters (game_session_id, created_at);

-- An encounter's combatants in turn order, and the ON DELETE CASCADE when it
-- is deleted. combatants has no index by character_id or user_id: only
-- deleting a character or an account looks them up that way, which happens
-- almost never.
CREATE INDEX IF NOT EXISTS combatants_encounter_id_order_index_idx
    ON combatants (encounter_id, order_index);

-- +goose Down
DROP INDEX IF EXISTS combatants_encounter_id_order_index_idx;
DROP INDEX IF EXISTS encounters_game_session_id_created_at_idx;
DROP INDEX IF EXISTS encounters_one_open_per_session;
