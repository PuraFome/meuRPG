-- +goose Up
-- xp_award_treasures records the treasures a "Voltar à cidade" award converted
-- into XP (MR-041, Etapa 9, D8): one row per treasure with its worth at that
-- moment. map_points.treasure_converted_award_id is the lock (a treasure is
-- converted once, and the link is cleared when the award is undone); this
-- table is the history, so an undone award still shows what it converted: the
-- history is never rewritten (ADR-0007).
--
-- point_id has no foreign key on purpose: after an undo the treasure is free
-- again, and the master may delete it (after unmarking it, as any found treasure),
-- but the award's history keeps the line.
-- Deleting the award (its campaign) deletes its rows. No personal data: IDs and
-- numbers.
CREATE TABLE IF NOT EXISTS xp_award_treasures (
    award_id UUID NOT NULL REFERENCES xp_awards (id) ON DELETE CASCADE,
    point_id UUID NOT NULL,
    value_po INT4 NOT NULL,
    CONSTRAINT xp_award_treasures_pkey PRIMARY KEY (award_id, point_id),
    CONSTRAINT xp_award_treasures_value_valid CHECK (value_po BETWEEN 0 AND 1000000)
);

-- +goose Down
DROP TABLE IF EXISTS xp_award_treasures;
