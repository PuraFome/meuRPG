# Point kinds: traps, treasure and carried light

`map_points.kind` accepts battle, sub-map, RP scene, `trap`, `treasure` and `light`.
This page covers the last three and the light a character carries (MR-034 to MR-036, MR-041, RN-10).
The code is in `spec.go` (the kinds' data), `pointwrites.go` (the master's writes to points, with `applyspec.go` for a kind's data), `traps.go` (revealing a trap, treasure found), `traps_play.go` (traps in play) and `carriedlight.go`.

## Storage

Each new kind has its own columns, `NULL` on the other kinds; CHECK constraints guarantee a kind never carries another's data.
Changing a point's kind deletes the old kind's data and requires the new one's.

- **Trap.** The definition is in `trap` (JSONB, in the format of the API's `TrapSpec`, without the state) and the state in `trap_state` (armed, triggered, disarmed), because live play changes the state and because "triggered" decides who sees the trap.
- **Treasure.** The value in gp (an integer, 0 to 1,000,000; the description is the point's), when it was found, the session open at that time, and `treasure_converted_award_id`.
- **Light.** The origin preset (or nothing, if custom) and the two radii in feet (0 to 120, multiples of 5, not both 0). A light point cannot be revealed.
- **No foreign key on two IDs.** `treasure_session_id` and `treasure_converted_award_id` are plain UUIDs: `map_points` and `game_sessions` already reference each other (the open scene), and a reference to `game_sessions` or `xp_awards` would close a cycle the test databases' table copy (`platform/dbtest`) cannot build. Nothing is lost, because a session and an award are only deleted along with the campaign, which deletes the map points too.

## Traps

The server validates a trap on creation and change with the same rules as the `effects/traps.json` loader:

- DCs from 1 to 30 (notice may be 0, "none"), area from 1 to 4, trigger `enter` or `manual`;
- an attack with bonus 0 to 20 and 1 to 10 attacks;
- damage of 1 to 20 dice of d4 to d12, or a number from 1 to 100, in at most 4 damage parts, and at most 4 conditions;
- a damage type and a condition that exist;
- a save that on failure does damage or gives a condition, and half damage only where there is damage.

Creating from a preset **copies** the numbers: changing the preset later touches no trap on a map.
An edit that does not send the state (`TRAP_STATE_UNSPECIFIED`) keeps the current one; only a new trap is born armed.

**A trap that triggered stays public forever.**
`trap_triggered_at` is stored the first time the state becomes `triggered` and, except for the trigger's "Desfazer" (`RestoreTrap`, which returns the trap as it was), nothing clears it.
It, not the current state, decides "everyone sees" in `seesPoint`, in the point count and in `ListPointRevealsOfCampaign`.
Disarming it later shows "Desarmada" to whoever sees it, and re-arming it does not hide it.
An undone trigger makes the trap hidden again if it was hidden before.

**Who knows a trap (RN-10).**
The table `map_point_reveals(point_id, character_id, how, at)` says which characters know it; `how` is `noticed` or `searched` (written in play, see Traps in play in [Architecture](../play/traps.md#traps-in-play)) or `master` (`RevealTrap`, 1 to 50 player characters or all, writes `trap_revealed` with a session open).

- `viewerOf` reads, for each player, the traps revealed to their characters (`knownTraps`) and `seesPoint` decides. Revealing to all is the `revealed_at` the point already had.
- A trap only some characters know is announced only to their players (`map_changed` through `PublishToUsers`) when it changes, is revealed, hidden or deleted. The players are read in the transaction, before the `DELETE` removes the `map_point_reveals` rows. A triggered trap, or one revealed to all, everyone sees.
- The response to a player carries only name, description, position, area and state. DCs, trigger, effect and preset never leave the master.
- "Quem notaria" (each character's passive Perception, with −5 in dim light) is computed by the server (`GetTrapNoticers`). The session page asks again for each open trap card when a token or combatant moves (`token_moved`, `combatant_moved`), one read at a time, so "Perto da armadilha" and "Longe da armadilha" follow the tokens.

## Treasure found

`MarkTreasureFound(characters)` stores `treasure_found_at`, the open session (`LiveSession.OpenSessionID`, which locks the session row like `AppendEvent`) and the finders (`map_treasure_finders`).
From then on anyone who sees the map sees the treasure with its description, value and finders.
Marking again swaps the finders and keeps the time.

- The events `treasure_found`, `treasure_unfound` (and `trap_revealed`) are written only with a session open and carry IDs and the value in gp, never a name or text.
- A **found** treasure cannot be deleted, nor the map that holds it (`MapBlocked` `TREASURE_FOUND`: unmark first), so the session summary never loses it silently.
- Once `treasure_converted_award_id` is filled, the value, the mark, the kind and deleting the point or map are locked (`MapBlocked` `TREASURE_CONVERTED`). The XP award's transaction fills and clears that field ([Progression](../progression.md#voltar-à-cidade-mr-041)).
- Hidden treasure generated by the master's tool is described in [Treasure](treasure.md).

## The light a character carries

`map_tokens.carried_light` stores a preset's key (`light:torch`, from `ListLightPresets`).
`SetCarriedLight` is the player's for their own character and the master's for anyone.
A player on an NPC gets `not_found`, on another player's character `permission_denied`, and on a hidden token `not_found` (like having no token) with no notice.
`MapToken` carries the key only to the master and the owner; what others see of it comes through the fog ([Fog](fog.md)).
