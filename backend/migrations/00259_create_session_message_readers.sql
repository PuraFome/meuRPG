-- +goose Up
-- session_message_readers lists the players who read a line that is not for everyone
-- (session_messages.for_everyone = false). The master and the author read the line without a row.
-- A row goes with the line (CASCADE) and with the reader's account (CASCADE).
CREATE TABLE IF NOT EXISTS session_message_readers (
    message_id UUID NOT NULL REFERENCES session_messages (id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    PRIMARY KEY (message_id, user_id)
);

-- +goose Down
DROP TABLE IF EXISTS session_message_readers;
