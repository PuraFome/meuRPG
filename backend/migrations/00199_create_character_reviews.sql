-- +goose Up
-- character_reviews is the master's "Pedir ajustes" on a character that waits for
-- approval (RN-15, MR-024): the reason he wrote and where the request stands. A
-- character that was never sent back has no row ("awaiting"); the row exists from the
-- request on, and it is gone as soon as the character is approved, rejected or deleted.
--
-- The reason is free text of the master about a player, so it is personal data of the
-- player (docs/privacy.md). It lives in its own table, not in a column of characters,
-- so that deleting works by itself in every case:
--   - the character is deleted (a rejection, or the account of its master): CASCADE on
--     character_id;
--   - the campaign is deleted: CASCADE on campaign_id (the character itself stays with
--     its player, without the reason);
--   - the player deletes their account: CASCADE on player_user_id (the character stays
--     with the campaign, RN-16, without the reason).
-- Approval deletes the row in the approval's transaction.
--
-- status is 'changes_requested' (the master asked, the player has not answered) or
-- 'resubmitted' (the player sent the sheet again; the reason stays for the master's
-- history until one of the cases above). reason is 1 to 500 characters, as the API.
--
-- request_key/request_hash and resubmit_key/resubmit_hash are the idempotency key of the
-- last RequestCharacterChanges and of the last ResubmitCharacter, each scoped to the
-- campaign and the caller, and the hash of the request they came with: a retry of the
-- same call returns what it did, and the same key with another request is refused.
CREATE TABLE IF NOT EXISTS character_reviews (
    character_id UUID NOT NULL PRIMARY KEY REFERENCES characters (id) ON DELETE CASCADE,
    campaign_id UUID NOT NULL REFERENCES campaigns (id) ON DELETE CASCADE,
    player_user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    status TEXT NOT NULL,
    reason TEXT NOT NULL,
    requested_at TIMESTAMPTZ NOT NULL,
    resubmitted_at TIMESTAMPTZ NULL,
    request_key TEXT NULL,
    request_hash TEXT NULL,
    resubmit_key TEXT NULL,
    resubmit_hash TEXT NULL,
    CONSTRAINT character_reviews_status_valid CHECK (status IN ('changes_requested', 'resubmitted')),
    CONSTRAINT character_reviews_reason_length CHECK (char_length(reason) BETWEEN 1 AND 500),
    CONSTRAINT character_reviews_resubmitted CHECK ((status = 'resubmitted') = (resubmitted_at IS NOT NULL))
);

-- +goose Down
DROP TABLE IF EXISTS character_reviews;
