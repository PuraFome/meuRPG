-- +goose Up
-- campaigns is a D&D campaign: the table that groups sessions, characters
-- and maps (MR-001). Who may do what in it lives in campaign_members.
--
-- xp_mode is how the campaign awards experience (RN-09): for defeated
-- enemies, for gold, or by milestones. It is text with a CHECK rather than
-- an ENUM type, because adding a value to a CHECK is a plain migration.
--
-- created_by is the account that created the campaign, today always its
-- master. Deleting that account deletes the campaign, and with it the
-- memberships and invites (docs/privacidade.md, "Excluir a conta"). There is
-- no index on created_by: only an account deletion looks campaigns up by it,
-- and scanning a small table then is fine.
CREATE TABLE IF NOT EXISTS campaigns (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    xp_mode TEXT NOT NULL,
    created_by UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT campaigns_name_length CHECK (char_length(name) BETWEEN 1 AND 80),
    CONSTRAINT campaigns_xp_mode_valid CHECK (xp_mode IN ('enemies', 'gold', 'milestones'))
);

-- +goose Down
DROP TABLE IF EXISTS campaigns;
