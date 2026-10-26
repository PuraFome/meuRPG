-- +goose Up
-- campaign_exports gets a fifth state, 'pending' (MR-050): the export is asked for and its
-- task is not running yet. The task that writes the zip is delivered at least once, so it
-- claims the row with a conditional update (pending to running) and a duplicate delivery
-- finds nothing to claim. The pending row counts as running for the page, for the one
-- export per campaign and for the stale rule.
ALTER TABLE campaign_exports DROP CONSTRAINT IF EXISTS campaign_exports_state_valid;
ALTER TABLE campaign_exports ADD CONSTRAINT campaign_exports_state_valid
    CHECK (state IN ('pending', 'running', 'done', 'failed', 'canceled'));

-- +goose Down
UPDATE campaign_exports SET state = 'running' WHERE state = 'pending';
ALTER TABLE campaign_exports DROP CONSTRAINT IF EXISTS campaign_exports_state_valid;
ALTER TABLE campaign_exports ADD CONSTRAINT campaign_exports_state_valid
    CHECK (state IN ('running', 'done', 'failed', 'canceled'));
