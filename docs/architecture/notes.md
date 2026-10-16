# Notes module: player notes

Each player writes private notes in the campaign and reads, in the same list, the clues the master revealed to them (MR-030, MR-029).
The `notes` module owns the table `player_notes` (see [Data](../data/README.md)) and the `NotesService` (`proto/meurpg/notes/v1/notes.proto`); the code is in `backend/internal/notes`.
**The master does not read the notes, and no other player does either.**
All the module's SQL filters by author, so another person's note is never found.

## Who may call what

| Call | Master | Active player | Non-member | Anonymous | Pending member (RN-15) |
| --- | --- | --- | --- | --- | --- |
| `ListNotes`, `CreateNote`, `UpdateNote`, `DeleteNote`, `ListNoteScenes` | `not_found` | Yes, on their own notes | `not_found` | `unauthenticated` | `not_found` |

The master gets `not_found`, not `permission_denied`, on purpose: they have no notes, and the answer is the same as for a note that does not exist.
Another player's note, a clue's ID and an ID that does not exist are also `not_found`.
`TestNotesAuthorizationMatrix` calls each method as each role and fails if a new method is not in its table.

## Behaviour

- **The list** (`ListNotes`) holds the player's notes and the clues revealed to them, newest first (a note by its last write, a clue by its arrival), with an optional filter by scene. A clue comes as `NOTE_KIND_CLUE`: read only, no editing or deleting, and it **does not count** toward the note limit. The response carries `note_count` and `max_notes`. The list is not paged: at most 300 notes plus the clues.
- **The limits.** A note's text is 1 to 2,000 characters (multiple lines allowed). Each player has at most 300 notes per campaign (`resource_exhausted`, checked in the `INSERT`'s transaction).
- **The scene label** can only name a **discovered** scene (see RP scenes in [Architecture](play/scenes-and-stage.md)). An undiscovered scene and one that does not exist are refused the same way (`invalid_argument`, same message), so the player never learns it is there. `ListNoteScenes` is the picker: only the discovered scenes, by today's name. A note's or clue's label shows the scene's name only while the scene is discovered; a clue of an undiscovered scene arrives without a label.
- **Deleting the account** deletes the notes and the received clues (`ON DELETE CASCADE`). No call lets an active member leave a campaign or be removed from it; a pending member the master refuses or removes takes their data along.
- **Free text stays out of logs and events.** A note's text never enters a log or a session event (see [Privacy](../privacy.md)).

## How it connects to `maps`

The discovered scenes and the received clues are `maps` tables.
`notes` declares the `Scenes` interface (`DiscoveredScenes`, `ReceivedClues`), implemented by `maps.SessionMaps` and wired in `cmd/api`, with the simple types in `notes/link`.
Neither module imports the other.
