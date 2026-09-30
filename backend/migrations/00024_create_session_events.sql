-- +goose Up
-- session_events is the game session's history (ADR-0007): every change
-- made at the table becomes one row, which is never updated. Etapa 5 writes
-- one kind, 'character_vitals_adjusted': the master's correction of a
-- character's vitals (RN-02). Damage, healing, spells and XP come with
-- combat (Etapa 6), each as a new kind in the CHECK below.
--
-- A row is written in the same transaction as the change it records, so
-- the history and the state never disagree. seq numbers a session's events
-- from 1, in the order they happened: the next one is max + 1, read inside
-- the transaction while the session's row is locked (FOR UPDATE), and
-- UNIQUE (game_session_id, seq) is the backstop.
--
-- idempotency_key is the UUID the app sends with a change. A retry with the
-- same key finds this row and changes nothing (UNIQUE per session). It is
-- NULL for events without one; NULLs never collide in a UNIQUE constraint.
--
-- payload is small JSON with the numbers before and after the change: no
-- free text, no names. actor_user_id is who made the change (SET NULL when
-- the account is deleted); character_id is the character it is about (SET
-- NULL if the character is deleted, which keeps the history). There is no
-- other personal data. Deleting the campaign deletes its sessions and,
-- with them, their events.
CREATE TABLE IF NOT EXISTS session_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    game_session_id UUID NOT NULL REFERENCES game_sessions (id) ON DELETE CASCADE,
    seq INT4 NOT NULL,
    kind TEXT NOT NULL,
    actor_user_id UUID NULL REFERENCES users (id) ON DELETE SET NULL,
    character_id UUID NULL REFERENCES characters (id) ON DELETE SET NULL,
    payload JSONB NOT NULL,
    idempotency_key UUID NULL,
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT session_events_game_session_id_seq_key UNIQUE (game_session_id, seq),
    CONSTRAINT session_events_game_session_id_idempotency_key_key UNIQUE (game_session_id, idempotency_key),
    CONSTRAINT session_events_seq_valid CHECK (seq >= 1),
    CONSTRAINT session_events_kind_valid CHECK (kind IN ('character_vitals_adjusted')),
    CONSTRAINT session_events_payload_valid CHECK (
        jsonb_typeof(payload) = 'object' AND octet_length(payload::TEXT) <= 4096
    )
);

-- +goose Down
DROP TABLE IF EXISTS session_events;
