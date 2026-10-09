package play

// eventCharacterChoicesCompleted is the session event a player's completion of the
// choices of a locked sheet writes (CharacterService.CompleteCharacterChoices, in
// package characters); it is listed here with the other kinds of session_events so
// TestSessionEventKindsMatchTheTable sees the whole table. Nothing in the combat log
// reads it.
const eventCharacterChoicesCompleted = "character_choices_completed"
