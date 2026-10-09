-- +goose Up
-- revivify_requests is a Revivify cast outside a combat (SRD 5.1, Revivify): the spell reaches
-- back one minute, the app does not count time outside a combat, and so the master answers
-- whether the creature died less than a minute ago. Nothing is spent until he says it did.
--
-- status is 'pending' (waiting for the master), 'confirmed' (less than a minute: the slot is
-- spent, the target lives with 1 hit point) or 'denied' (more: nothing is spent). The request
-- belongs to the open game session and goes with it. caster_character_id and
-- target_character_id are characters of the campaign; a character deleted takes the request
-- with it.
--
-- create_key/create_hash are the idempotency key of the cast (scoped to the campaign and the
-- caller) and the hash of its request; answer_key/answer_hash those of the master's answer.
CREATE TABLE IF NOT EXISTS revivify_requests (
    id UUID NOT NULL PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id UUID NOT NULL REFERENCES campaigns (id) ON DELETE CASCADE,
    game_session_id UUID NOT NULL REFERENCES game_sessions (id) ON DELETE CASCADE,
    caster_character_id UUID NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    target_character_id UUID NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    requested_by_user_id UUID NULL REFERENCES users (id) ON DELETE SET NULL,
    slot_level INT4 NOT NULL,
    slot_pact BOOL NOT NULL DEFAULT false,
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL,
    answered_at TIMESTAMPTZ NULL,
    create_key TEXT NULL,
    create_hash TEXT NULL,
    answer_key TEXT NULL,
    answer_hash TEXT NULL,
    CONSTRAINT revivify_requests_status_valid CHECK (status IN ('pending', 'confirmed', 'denied')),
    CONSTRAINT revivify_requests_slot_valid CHECK (slot_level BETWEEN 3 AND 9),
    CONSTRAINT revivify_requests_answered CHECK ((status = 'pending') = (answered_at IS NULL))
);

CREATE UNIQUE INDEX IF NOT EXISTS revivify_requests_create_key_idx
    ON revivify_requests (campaign_id, create_key) WHERE create_key IS NOT NULL;
CREATE INDEX IF NOT EXISTS revivify_requests_session_idx
    ON revivify_requests (game_session_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS revivify_requests;
