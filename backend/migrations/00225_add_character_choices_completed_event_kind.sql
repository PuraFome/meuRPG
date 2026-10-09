-- +goose Up
-- The session event of a player completing choices of a sheet that is locked for them
-- (PM-05, CompleteCharacterChoices): a `character_choices_completed` event is written
-- in the same transaction, so the master's history says who completed what. It holds
-- the character's ID and a count, never a name or a choice. A new kind is one row of
-- the table the foreign key on session_events.kind points at (00168).
INSERT INTO session_event_kinds (kind) VALUES ('character_choices_completed')
ON CONFLICT (kind) DO NOTHING;

-- +goose Down
DELETE FROM session_events WHERE kind = 'character_choices_completed';
DELETE FROM session_event_kinds WHERE kind = 'character_choices_completed';
