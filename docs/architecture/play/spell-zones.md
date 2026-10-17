# Spell zones

The areas that Spirit Guardians, Moonbeam, Cloudkill and Web keep on the map (RN-37).
The code is `zones_*.go` in `backend/internal/play`; the rules are in [Rules](../../product/rules.md#rn-37-areas-that-spells-keep-on-the-map).

## The model

A zone is a row of `spell_zones` ([data](../../data/combat.md#spell_zones)): the combat, the caster (`owner_id`), the spell, the squares it covers now, and what a trigger needs (the save and the damage as the cast made them).
It is not a lasting effect (RN-22).
A lasting effect belongs to one target and has its own clock; a zone has no single target and carries geometry.
Its life has three points, each a single call: the cast makes it (`createZone`), the end of a concentration deletes it (`endZonesOf`, called by `endConcentrationEffects`, so every way a concentration ends reaches it, `stopConcentrating` and `endConcentrationAtZero` included), and the start of the caster's turn deletes it when its round is out (`endZonesOutOfTime`).
The foreign keys delete it with the combat or the caster.

`zoneDefs` (`zones_catalog.go`) is the table of the four spells: kind (damage or restrain), shape and size, how the cast places it (`AreaPlacement`), how it moves, the save, whether it spares the caster's side, and whether the cast still hits the creatures inside.
Another spell with a lingering area is another row, plus its entry in the catalogue of lasting effects if it also puts an effect on its targets.
`placementOf`, `areaOf` and the area fields of the turn options read the row first, so the preview, the cast and the options lay the area out as the zone does.
A cube is centred on its point (`Area.FromPoint`, an even side puts the extra square on the larger side).

## The cast

`castFlow.affect` hands a placed cast of a zone spell to `affectZone`.
Moonbeam, Spirit Guardians and Cloudkill hit nobody at the cast; Web still runs `hitTarget` and `applyLastingSpell`, so the creatures inside get the effect they always got.
`createZone` keeps `plan.origin` and `plan.squares` (what `planArea` worked out, walls included), the caster's DC, the damage at the cast's slot level and the end round (`SpellEffectRounds`).
The cast keeps its place in the event as before (`placed`, `area_col`, `area_row`); the cast's `cast_id` is on the zone, and the undo of the cast deletes the zone with that id (`castEffectsBack`).
Without a grid (`isTheatre`) a spell is not placed, so there is no zone.

## When a zone looks

| Where | What it does |
| --- | --- |
| `zonesAfterSteps` (`zones_hooks.go`) | Called with the creatures that changed square: `moveInTx` (the mover and the creature it dragged; this also covers the master's move and a placement), `castFlow.teleport` (Misty Step's landing, `combat_teleport.go`), `pusher.enterZones` (a push by a cast or by the Open Hand technique) and the shove outcome (`shovePushes`). A step has where the creature stood and the squares it went through (the straight line for a walk, only the end for a jump, a push or a forced move). A zone catches a step that comes into it: one square inside whose predecessor is outside |
| `zonesAtTurnStart` | Called at the end of `startOfTurnEffects`: ends the zones whose round is out, moves the zones that drift, then catches every creature that starts its turn inside |
| `followCaster` | Part of `zonesAfterSteps`: when the creature that moved owns a zone centred on it, the zone is placed on its square and the creatures the new area covers and the old one did not are caught as if they had come in |

The undo of a move, of a push or of a shove puts the creature back and re-evaluates nothing.

## Catching a creature

`catchZone` (`zones_trigger.go`) is the whole of it, in this order:

1. The checks: the creature is on the map and standing; it is not spared (Spirit Guardians leaves the caster and the caster's `side` alone); the zone has not caught it on this turn (`hit_marks`, below). The web also skips a creature it holds or asks a save of already (`heldByWeb`).
2. The turn is written first: `hit_marks[creature] = "<round>:<first combatant of the turn>"`. At a turn start the mark is built from the turn that starts; in the middle of a change it is built from the encounter's current turn.
3. The saving throw: `zoneSave` calls `spellSave` with a `link.Spell` made of the zone's save, so it is the cast's save (modes, auto-fail, Lucky, effect dice).
4. The effect: `dealZoneDamage` rolls the dice and halves them on a pass, applies resistance, and inserts an effect pending damage (`InsertEffectPendingDamage`, `effect_source_key` the spell), landed by `landDamage` (an NPC or creature takes it, a character's waits for the master); `afterDamage` then opens the concentration save as for any damage. The web instead adds `effect:web-restrained` (`restrainInWeb`) on a failure.
5. One event, `spell_zone_triggered`, with a `lastingEvent` (change `saved` or `failed`, reason `zone_enter` or `zone_start`, the roll, the DC and the damage).

The events a zone leaves (`spell_zone_triggered`, `spell_zone_moved`, and the `lasting_effect_ended` of a zone that timed out) are held on the change (`combatTx.zoneOut`) and written by `record` right after the change's own event, so the log reads the move and then what it brought.
They are registered as effect lines in `logHandlers` and `effectLogKinds`; `COMBAT_LOG_EFFECT_CHANGE_ZONE_MOVED` is the one new change.
None of the three is undoable (`undoableKinds`), so a move that caught someone cannot be undone as a move: the damage is a pending damage, discarded and undone as one.

## Moving a zone

- Spirit Guardians follows its caster (`followCaster`).
- Cloudkill drifts at the start of its caster's turn (`driftZones`): `planPush` from the caster's square, `driftFt` (10) long, so it stops at a wall; a cloud on the caster's own square stays.
- Moonbeam is moved by `MoveSpellZone` (`zones_move.go`), a keyed `write` of kind `spell_zone_moved`. A player must be the caster's player, on the caster's turn (`mustActNow`), with the action free (it is spent), and within `moveRangeFt` (60) of the zone's origin (`TARGET_OUT_OF_REACH` with `missing_ft`); the square must be on the map and not a wall. The master moves any zone, with none of that. A zone the caller does not read is `not_found`, as one that does not exist. The creatures the new area reaches are caught by `relocate`, the same step that follows and drifts.

## The save question, the end and the other effects

- **The question.** A zone with a save that catches a player's character (`Kind == kindPlayer`) opens an `EFFECT_SAVE` reaction window with `windowTrigger.Zone` (`askZoneSave`, `zones_save.go`); the answer is `RollEffectSave`, which loads the zone in `loadZoneSave` and settles it in `settleZoneSave` (damage halved on a pass, or the web's restraint) through the same `applyZone` an NPC's catch uses. The prompt (`zoneSavePrompt`) carries `zone_id`, `zone_reason`, the damage type and the half, and the DC only for the master and the caster's player. The windows of one creature on one turn share a group (`zoneWindowGroup`), so the existing order rule applies, and `openOne` lists a player the first only. A zone that ends closes its windows (`closeZoneWindows`).
- **No save.** A zone whose row has no save ability inserts a pending damage awaiting its roll with the caster as attacker (`catchWithoutSave`), rolled and applied by the paths of any damage.
- **Ending.** `endZonesOf` writes an `ended` line for each zone (reason `concentration` or `concentration_replaced`), the clock writes `duration`, `EndSpellZone` (master only) writes `ended_by_master`. They are written at once, not held, because a concentration also ends outside the changes that flush the zone lines.
- **Speed and terrain.** A zone with `slows` adds `effect:spirit-guardians-slowed` (speed 50%, until the end of the creature's turn) when it catches a creature. A zone with `difficult` is added to the Difficult layer of `terrainOf` (`withZoneTerrain`, a copy, so the map's own layer is untouched).

## The view and the privacy

`Encounter.zones` is built by `encounterData.zonesView` from the zones read with the combat (`loadEncounter`).
A player gets the zones of casters they `sees` (not hidden, not in the dark of a fog map); the master gets all, with `SPELL_ZONE_VISIBILITY_MASTER_ONLY` on the zones of hidden casters.
`rounds_left` is the round the zone ends at, minus the current round, 0 for a zone with no round count.
The hint of a zone that moved goes to the master alone when the caster is hidden (`publishZoneMoved`).
The event of a catch is `Secret` for a hidden caster or a hidden creature, and its actor is the caster and its target the creature, so the log's own rule hides it from a player who does not see either (RN-10); the DC is the master's alone (`effectLogRoll`).
A player gets the numbers (`save_*`, damage, `notes`, `moves_per_turn_ft`, `rounds_left`) only for a zone of their own character; for another they get `details_hidden` and the name, shape, squares, concentration and the occupants they see. The squares of a visible caster's zone reach every player, also on a fog map where they have not explored them: the shape of a spell is not a secret the fog keeps.

## Tests

`zones_test.go` (Spirit Guardians, Moonbeam, Cloudkill, Web, the push, the forced move, the undo, the player's character, the permissions and the retry) and `zones_hidden_test.go` (the viewer matrix for a hidden caster, a revealed one and a hidden creature, and the stream hint); `internal/leaktest` classifies `MoveSpellZone`.
The grid cube is in `rules/grid/area_test.go`.
