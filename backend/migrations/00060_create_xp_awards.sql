-- +goose Up
-- xp_awards are the master's gifts of experience to the party (MR-016, RN-09):
-- one row per "Dar XP" or "Registrar marco". It is the XP history the whole
-- campaign reads (question 50), and, like session_events, it is never
-- rewritten or deleted (ADR-0007): undoing the last award sets undone_at and
-- undone_by, and the XP goes back through the sheets, not through this table.
--
-- mode says where the XP came from: 'enemies' (the defeated NPCs of an ended
-- encounter), 'gold' (1 XP per gold piece, RN-09), 'manual' (the master's own
-- number, any time) or 'milestone' (no XP: it marks the characters "pode subir
-- de nível"). reason is what the master wrote, 1 to 120 characters; it is
-- fiction, shown to every member of the campaign, and never goes to a
-- session_events payload.
--
-- encounter_id is the combat an 'enemies' award is about (SET NULL when the
-- encounter is deleted with its session); gold is the gold of a 'gold' award.
-- total_xp is what was split: the sum of the defeated NPCs' XP, the gold, or the
-- typed amount; 0 for a milestone. Each character's share is in
-- xp_award_shares.
--
-- idempotency_key is the UUID the app sent with the award: sending it again
-- finds this row and changes nothing. undo_key is the same for the undo.
-- given_by and undone_by are users (SET NULL when the account is deleted).
-- Deleting the campaign deletes its awards.
CREATE TABLE IF NOT EXISTS xp_awards (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id UUID NOT NULL REFERENCES campaigns (id) ON DELETE CASCADE,
    given_by UUID NULL REFERENCES users (id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL,
    mode TEXT NOT NULL,
    reason TEXT NOT NULL,
    encounter_id UUID NULL REFERENCES encounters (id) ON DELETE SET NULL,
    gold INT4 NULL,
    total_xp INT4 NOT NULL,
    idempotency_key UUID NOT NULL,
    undone_at TIMESTAMPTZ NULL,
    undone_by UUID NULL REFERENCES users (id) ON DELETE SET NULL,
    undo_key UUID NULL,
    CONSTRAINT xp_awards_mode_valid CHECK (mode IN ('enemies', 'gold', 'manual', 'milestone')),
    CONSTRAINT xp_awards_reason_length CHECK (char_length(reason) BETWEEN 1 AND 120),
    CONSTRAINT xp_awards_total_valid CHECK (
        (mode = 'milestone' AND total_xp = 0) OR (mode <> 'milestone' AND total_xp BETWEEN 1 AND 1000000)
    ),
    CONSTRAINT xp_awards_gold_valid CHECK (
        (mode = 'gold' AND gold IS NOT NULL AND gold BETWEEN 1 AND 1000000) OR (mode <> 'gold' AND gold IS NULL)
    ),
    CONSTRAINT xp_awards_undone_valid CHECK ((undone_at IS NULL) = (undo_key IS NULL))
);

-- +goose Down
DROP TABLE IF EXISTS xp_awards;
