-- +goose Up
-- player_notes holds the private notes a player writes during the campaign
-- (MR-030, Etapa 8, D6, question 60). They are free text, 1 to 2,000
-- characters, and they belong to their author alone: nobody else ever reads
-- one, not even the campaign's master (RN-20). At most 300 per player per
-- campaign (the API checks, inside the transaction that inserts one); the
-- clues the master reveals are not notes and do not count (they live in
-- scene_clue_reveals).
--
-- scene_point_id is the optional scene tag, a point the group discovered (the
-- API checks, scene_discoveries); deleting the point drops the tag and keeps
-- the note (SET NULL). Deleting the account or the campaign deletes the notes
-- (CASCADE).
CREATE TABLE IF NOT EXISTS player_notes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id UUID NOT NULL REFERENCES campaigns (id) ON DELETE CASCADE,
    author_user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    text TEXT NOT NULL,
    scene_point_id UUID NULL REFERENCES map_points (id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT player_notes_text_length CHECK (char_length(text) BETWEEN 1 AND 2000)
);

-- +goose Down
DROP TABLE IF EXISTS player_notes;
