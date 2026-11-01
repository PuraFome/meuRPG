# Stored session events

One folder per kind of `session_event_kinds`, one file per shape of payload the code has written.
`stored_events_test.go` decodes each with the readers of today (the undo's walk, the combat log, the highlights, the session summary), with no database.

| File | What it is | Rewritten? |
| --- | --- | --- |
| `today.json`, `today-<what>.json` | The payload as the writer's Go type marshals it now, every field set (`TestTodaysPayloadsAreWhatTheWritersWrite`) | `go test ./internal/play -run TestTodaysPayloads -update`; read the diff |
| `external-<what>.json` | The payload of a writer in another module (progression, maps, characters) or a map literal of play, copied by hand from its code | By hand, when that writer changes |
| `v<n>-<what>.json` | An older shape: a field that was added later is missing | Never |

The older shapes come from the struct of the commit that first wrote the kind (`git show <commit>:backend/internal/play/<file>`):

| Fixtures | Commit |
| --- | --- |
| `attack_rolled`, `damage_rolled`, `damage_applied`, `damage_discarded`, `hit_points_adjusted`, `action_taken`, `action_undone`, `combatant_hidden_set`, `combatant_moved/v1-whole-feet` | `1c217360` (#61, the first `actionEvent`); the move before the tenths of a foot, jumps and `from` came with `9002d08d` (#104) |
| `trap_triggered/v1-first-firing` | `6722da65` |
| `hit_dice_spent/v1-before-song-of-rest`, `rest_taken/v1-original` | `6cd0bef0` |
| `scene_check_rolled/v1-before-dc-shown` | `28f56579` |
| `character_vitals_adjusted/v1-before-resources` | `724227f5` |
| `stage_changed/v1-before-hidden` | `a63181c65` (the stage before the hidden NPCs: no `hidden`, no `revealed`) |
| `scene_opened/v1-before-audience`, `scene_closed/v1-before-audience`, `scene_check_rolled/v2-before-audience` | `4cb338c77` (the scene before it could be open to only some players: no `players`, no `audience`) |
| `spell_cast/v1-before-slot-counted` | `e4aa5fdcf` (#712; an NPC's cast stored its slot without saying it counted it: no `slot_counted`) |

No JSON key of a play payload has been renamed since the first commit; only fields were added.
A fixture whose key today's type no longer reads fails the test; if that is on purpose, list it in `retiredKeys` with the reason.

A new kind needs a line in `actionEventKinds`, `otherWriters` or `externalKinds` and its fixture; the test says which is missing.
