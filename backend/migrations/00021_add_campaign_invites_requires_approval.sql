-- +goose Up
-- requires_approval says whether whoever accepts the invite joins the
-- campaign at once (false, as before) or as a pending member, whose
-- character the master must approve first (true: RN-15, MR-024). The master
-- chooses it per invite ("Exigir aprovação do mestre").
--
-- DEFAULT false keeps every existing invite working as it did. ADD COLUMN IF
-- NOT EXISTS makes the statement safe to run twice.
ALTER TABLE campaign_invites ADD COLUMN IF NOT EXISTS requires_approval BOOL NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE campaign_invites DROP COLUMN IF EXISTS requires_approval;
