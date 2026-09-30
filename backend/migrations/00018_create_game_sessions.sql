-- +goose Up
-- game_sessions records when each game session of a campaign started and
-- ended (PlayService.StartGameSession and EndGameSession). Starting a
-- session also locks, in the same transaction, the sheets of the campaign's
-- living player characters that are still unlocked (RN-01).
--
-- session_number counts the campaign's sessions from 1. The UNIQUE
-- constraint keeps two concurrent starts from taking the same number, and
-- its index also lists a campaign's sessions. A session is open while
-- ended_at is NULL; 00019 allows at most one open session per campaign.
--
-- There is no personal data here, only IDs and times. Deleting the campaign
-- deletes its sessions.
CREATE TABLE IF NOT EXISTS game_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id UUID NOT NULL REFERENCES campaigns (id) ON DELETE CASCADE,
    session_number INT4 NOT NULL,
    started_at TIMESTAMPTZ NOT NULL,
    ended_at TIMESTAMPTZ NULL,
    CONSTRAINT game_sessions_campaign_id_session_number_key UNIQUE (campaign_id, session_number),
    CONSTRAINT game_sessions_session_number_valid CHECK (session_number >= 1),
    CONSTRAINT game_sessions_ends_after_start CHECK (ended_at IS NULL OR ended_at >= started_at)
);

-- +goose Down
DROP TABLE IF EXISTS game_sessions;
