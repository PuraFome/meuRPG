-- +goose Up
-- session_messages is the conversation of a game session (MR-051, RN-36): the master narrates and
-- speaks as an NPC, the players speak as their character or out of character. A line belongs to
-- the session (CASCADE with it and with the campaign), is never edited and has no deletion.
--
-- speaker_kind says who speaks: 'narrator' (the master), 'npc' (the master as an NPC of the
-- campaign), 'character' (a player as their own character) or 'out_of_character' (a player as
-- themselves, shown by the display name of the author). speaker_ref is the character of 'npc' and
-- 'character' (SET NULL if it is deleted); speaker_name and speaker_image_id keep the name and the
-- portrait it had when the line was sent, so the line still reads the same if the NPC is renamed or
-- deleted. The API checks the combinations; the CHECKs below are the backstop.
--
-- author_user_id is who sent the line. SET NULL when the account is erased: the line stays and the
-- author reads "Jogador removido" (docs/privacy.md).
--
-- for_everyone says every member of the campaign reads the line. When it is false the readers are the
-- rows of session_message_readers (00259), plus the master and the author, who always read it. It is a
-- column, not "no rows in session_message_readers", because erasing the account of the last reader
-- must not turn a line for chosen readers into a line for everyone.
--
-- image_id is the gallery image the master put in the line (the readers may open it, RN-36). create_key
-- and create_hash are the idempotency key of SendSessionMessage and the hash of its request.
-- audio_made_at is when the voice of the line was made (the file is in the blob store); it counts the
-- day's voices against the server-wide cap, and is NULL until a reader asked to hear the line.
CREATE TABLE IF NOT EXISTS session_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id UUID NOT NULL REFERENCES campaigns (id) ON DELETE CASCADE,
    game_session_id UUID NOT NULL REFERENCES game_sessions (id) ON DELETE CASCADE,
    author_user_id UUID NULL REFERENCES users (id) ON DELETE SET NULL,
    speaker_kind TEXT NOT NULL,
    speaker_ref UUID NULL REFERENCES characters (id) ON DELETE SET NULL,
    speaker_name TEXT NULL,
    speaker_image_id UUID NULL REFERENCES gallery_images (id) ON DELETE SET NULL,
    text TEXT NOT NULL,
    image_id UUID NULL REFERENCES gallery_images (id) ON DELETE SET NULL,
    for_everyone BOOL NOT NULL DEFAULT true,
    create_key TEXT NULL,
    create_hash TEXT NULL,
    audio_made_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT session_messages_speaker_kind_valid CHECK (speaker_kind IN ('narrator', 'npc', 'character', 'out_of_character')),
    CONSTRAINT session_messages_text_length CHECK (char_length(text) BETWEEN 1 AND 2000)
);

-- +goose Down
DROP TABLE IF EXISTS session_messages;
