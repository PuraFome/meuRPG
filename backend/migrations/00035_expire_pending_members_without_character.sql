-- +goose Up
-- A pending membership without a character is deleted by CockroachDB's
-- row-level TTL job, on its own, 30 days after the person joined (RN-15,
-- MR-024, docs/privacy.md). The person loses nothing else: no account,
-- no other campaign. A new invite brings them back.
--
-- Same pattern as migration 00020. The expiration expression is the column
-- from 00034, and NULL means "never expires", so only the rows that are
-- pending without a character have a deadline. The job runs once a day
-- (CockroachDB's default), so a membership may outlive its deadline by up
-- to a day.
--
-- First, the rows that existed before the column: every pending membership
-- with no player character in its campaign gets joined_at + 30 days. This is
-- a separate migration from the ADD COLUMN because CockroachDB does not
-- allow writing to a column in the transaction that adds it. It is safe to
-- run twice: it only touches rows still without a deadline and without a
-- character, and a deadline is never set on a row that has a character.
UPDATE campaign_members AS m
SET pending_expires_at = m.joined_at + INTERVAL '30 days'
WHERE m.status = 'pending'
  AND m.pending_expires_at IS NULL
  AND NOT EXISTS (
      SELECT 1 FROM characters AS c
      WHERE c.campaign_id = m.campaign_id AND c.player_user_id = m.user_id AND c.kind = 'player'
  );

-- ALTER TABLE ... SET only sets the table's storage parameters, so running
-- it twice leaves the table as it was. The Down turns row-level TTL off.
ALTER TABLE campaign_members SET (
    ttl_expiration_expression = 'pending_expires_at'
);

-- +goose Down
ALTER TABLE campaign_members RESET (ttl);
