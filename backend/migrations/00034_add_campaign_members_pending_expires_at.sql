-- +goose Up
-- pending_expires_at is when a pending membership without a character is
-- deleted on its own (RN-15, MR-024): 30 days after the person joined.
--
-- A pending member is someone who accepted an invite with approval and has
-- not been approved yet. If they never create their character, nobody ever
-- has anything to approve or reject, so the master would have to notice and
-- remove them by hand. The deadline spares him that, and keeps a row that
-- serves no purpose from living forever (docs/privacidade.md). Migration
-- 00035 turns the deadline into row-level TTL.
--
-- The column is set only while the membership is pending AND the person has
-- no character yet:
--   - set to joined time + 30 days when a pending membership is created;
--   - cleared when the pending member creates their character: from then
--     on the master decides, and a character waiting for approval must not
--     vanish with its owner's membership;
--   - cleared when the membership becomes active.
-- NULL means "no deadline": every active membership, and a pending one whose
-- character already waits for the master. The master's list of "pending
-- without a character" is exactly the pending rows where it is not NULL.
--
-- One statement with several parts, so re-running it is safe (see 00022).
ALTER TABLE campaign_members
    ADD COLUMN IF NOT EXISTS pending_expires_at TIMESTAMPTZ,
    DROP CONSTRAINT IF EXISTS campaign_members_expiry_only_pending,
    ADD CONSTRAINT campaign_members_expiry_only_pending CHECK (pending_expires_at IS NULL OR status = 'pending');

-- +goose Down
ALTER TABLE campaign_members
    DROP CONSTRAINT IF EXISTS campaign_members_expiry_only_pending,
    DROP COLUMN IF EXISTS pending_expires_at;
