-- +goose Up
-- campaign_members says who belongs to a campaign and with which role
-- (RN-05). Roles are per campaign, so the same account can be the master of
-- one campaign and a player in another; users has no role at all.
--
-- Every authorization check reads this table (package authz, ADR-0011): a
-- row here is what lets someone see a campaign. The primary key
-- (campaign_id, user_id) answers both "is this user a member?" and "who are
-- the members?"; the index in 00010 answers "which campaigns is this user
-- in?".
--
-- role is 'master' or 'player' (the app says "mestre" and "jogador").
-- Deleting the campaign or the account deletes the membership.
CREATE TABLE IF NOT EXISTS campaign_members (
    campaign_id UUID NOT NULL REFERENCES campaigns (id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role TEXT NOT NULL,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (campaign_id, user_id),
    CONSTRAINT campaign_members_role_valid CHECK (role IN ('master', 'player'))
);

-- +goose Down
DROP TABLE IF EXISTS campaign_members;
