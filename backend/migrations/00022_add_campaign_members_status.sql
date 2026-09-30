-- +goose Up
-- status says whether a membership is in force (RN-15, MR-024):
--   - 'active': a member, as every membership was before this migration.
--   - 'pending': someone who accepted an invite that requires approval
--     (campaign_invites.requires_approval). They may only work on the one
--     character they create while they wait (package authz lists exactly
--     which calls); for everything else they are not a member. The master's
--     ApproveCharacter makes the row 'active'; RejectCharacter deletes it.
--
-- DEFAULT 'active' keeps every existing member a member.
--
-- Only a player can be pending: a campaign's master is always active
-- (campaign_members_only_players_pending). The CHECKs spell out IS NOT NULL
-- only where needed: status is NOT NULL, so both evaluate to true or false.
--
-- One statement with several parts, so re-running it is safe: ADD COLUMN IF
-- NOT EXISTS skips a column that exists, and each named CHECK is dropped, if
-- present, and added again (see 00007).
ALTER TABLE campaign_members
    ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'active',
    DROP CONSTRAINT IF EXISTS campaign_members_status_valid,
    ADD CONSTRAINT campaign_members_status_valid CHECK (status IN ('active', 'pending')),
    DROP CONSTRAINT IF EXISTS campaign_members_only_players_pending,
    ADD CONSTRAINT campaign_members_only_players_pending CHECK (role = 'player' OR status = 'active');

-- +goose Down
ALTER TABLE campaign_members
    DROP CONSTRAINT IF EXISTS campaign_members_only_players_pending,
    DROP CONSTRAINT IF EXISTS campaign_members_status_valid,
    DROP COLUMN IF EXISTS status;
