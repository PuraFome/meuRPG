-- +goose Up
-- The Inspiration of 00332 moves between characters only when the master agrees (RN-43), and the master can give back one that
-- a roll spent for nothing. Two tables hold what waits for the master's answer.
--
-- favor_passes is a player's ask to pass the Inspiration of their character to another player character. One row for each ask:
--   - from_character_id is the giver (the one that holds it), to_character_id the receiver; asked_by_user_id is the giver's owner.
--   - game_session_id is the open session the ask was made in: the master answers at the table, and the session's end closes the
--     pending ask as 'canceled'.
--   - status: 'pending' until it ends; 'accepted' (the master accepted: the Inspiration moved), 'refused' (the master refused: it
--     stays with the giver) or 'canceled' (the giver gave the ask up, or the session ended).
--   - ask_key is the idempotency key of the ask; closed_key the one of the call that closed it (accept, refuse or cancel), so a
--     retry finds the same row. Both are unique in the campaign.
-- A giver has at most one pending ask. The rows go with the campaign, the session and the characters.
--
-- favor_refund_prompts is the question the master gets when a roll spent the Inspiration and it added nothing over every source of
-- advantage (some of them the player may not read): one row for each such roll.
--   - character_id is the roller, roll_label_pt the roll's name as the log reads it.
--   - hidden_sources is a JSON list of the advantage sources the roller did not read (the master's view of why it added nothing).
--   - status: 'open' until the master answers; 'returned' (the Inspiration was given back), 'kept' (it stays spent: the master
--     chose so, or the session ended without an answer).
--   - answer_key is the idempotency key of the answer.
--   - roll_key is the idempotency key the spend was made under, that is the request's: the roll's event is stamped with it (favor_roll) and
--     so is its held roll (the hold keeps the request's key), because the key the event is written under is not always the request's
--     (a reaction window's replay writes none, and the answer to a die's question writes its own). The roll's undo and the forgetting of
--     its held roll close the prompt as 'kept', because the Inspiration went back with them.
-- The rows go with the campaign, the session and the character.
--
-- Production is live: the migration only adds, and every statement may run twice.
CREATE TABLE IF NOT EXISTS favor_passes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id UUID NOT NULL REFERENCES campaigns (id) ON DELETE CASCADE,
    game_session_id UUID NOT NULL REFERENCES game_sessions (id) ON DELETE CASCADE,
    from_character_id UUID NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    to_character_id UUID NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    asked_by_user_id UUID NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    ask_key TEXT NOT NULL,
    closed_key TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    closed_at TIMESTAMPTZ NULL,
    CONSTRAINT favor_passes_status_valid CHECK (status IN ('pending', 'accepted', 'refused', 'canceled')),
    CONSTRAINT favor_passes_not_self CHECK (from_character_id <> to_character_id),
    CONSTRAINT favor_passes_closed_valid CHECK ((status = 'pending') = (closed_at IS NULL)),
    CONSTRAINT favor_passes_ask_key_unique UNIQUE (campaign_id, ask_key),
    CONSTRAINT favor_passes_closed_key_unique UNIQUE (campaign_id, closed_key)
);
CREATE UNIQUE INDEX IF NOT EXISTS favor_passes_one_pending_idx ON favor_passes (from_character_id) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS favor_passes_campaign_pending_idx ON favor_passes (campaign_id, created_at) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS favor_passes_session_idx ON favor_passes (game_session_id);
CREATE INDEX IF NOT EXISTS favor_passes_to_character_idx ON favor_passes (to_character_id);

CREATE TABLE IF NOT EXISTS favor_refund_prompts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id UUID NOT NULL REFERENCES campaigns (id) ON DELETE CASCADE,
    game_session_id UUID NOT NULL REFERENCES game_sessions (id) ON DELETE CASCADE,
    character_id UUID NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    roll_label_pt TEXT NOT NULL DEFAULT '',
    hidden_sources JSONB NOT NULL DEFAULT '[]',
    status TEXT NOT NULL DEFAULT 'open',
    answer_key TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    answered_at TIMESTAMPTZ NULL,
    roll_key TEXT NOT NULL DEFAULT '',
    CONSTRAINT favor_refund_prompts_status_valid CHECK (status IN ('open', 'returned', 'kept')),
    CONSTRAINT favor_refund_prompts_answered_valid CHECK ((status = 'open') = (answered_at IS NULL)),
    CONSTRAINT favor_refund_prompts_answer_key_unique UNIQUE (campaign_id, answer_key)
);
CREATE INDEX IF NOT EXISTS favor_refund_prompts_campaign_open_idx ON favor_refund_prompts (campaign_id, created_at) WHERE status = 'open';
CREATE INDEX IF NOT EXISTS favor_refund_prompts_session_idx ON favor_refund_prompts (game_session_id);
CREATE INDEX IF NOT EXISTS favor_refund_prompts_character_idx ON favor_refund_prompts (character_id);
CREATE INDEX IF NOT EXISTS favor_refund_prompts_roll_idx ON favor_refund_prompts (campaign_id, character_id, roll_key) WHERE status = 'open';

-- The events of the pass and of the refund: the ask, the pass (favor_passed, already there since 00332), the refusal, the
-- cancel, the prompt, the refund and the prompt left as it was.
INSERT INTO session_event_kinds (kind) VALUES
    ('favor_pass_asked'),
    ('favor_passed'),
    ('favor_pass_refused'),
    ('favor_pass_canceled'),
    ('favor_refund_prompted'),
    ('favor_refunded'),
    ('favor_refund_kept')
ON CONFLICT (kind) DO NOTHING;

-- +goose Down
DELETE FROM session_events WHERE kind IN ('favor_pass_asked', 'favor_pass_refused', 'favor_pass_canceled', 'favor_refund_prompted', 'favor_refunded', 'favor_refund_kept');
DELETE FROM session_event_kinds WHERE kind IN ('favor_pass_asked', 'favor_pass_refused', 'favor_pass_canceled', 'favor_refund_prompted', 'favor_refunded', 'favor_refund_kept');
DROP TABLE IF EXISTS favor_refund_prompts;
DROP TABLE IF EXISTS favor_passes;
