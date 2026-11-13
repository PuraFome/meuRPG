# Feature state

A feature that a combatant may use only so often (once per turn, once per round, once per combat) leaves a mark on the combatant.
The marks live in ONE column, `combatants.feature_state`, not in a column for each feature.
The code is `backend/internal/play/featurestate.go`; the column is described in [Combat data](../../data/combat.md).
A test (`TestCombatantsGainNoColumnPerFeature`, `backend/internal/archtest`) refuses a new `combatants` column that is about one feature.

## The state

`feature_state` is a JSON object with one entry per feature key:

```json
{"sneak_attack": {"s": "turn", "at": "3:7f2c...", "n": 1}, "feature:giant-slayer": {"s": "turn", "at": "3:7f2c...", "n": 1, "c": "fire"}}
```

| Field | Meaning |
| --- | --- |
| `s` | The scope: how long the mark holds. `turn`, `round` or `combat`. |
| `at` | The mark that says when the entry was written: for `turn`, `turnKey` of the encounter (`"<round>:<combatant on turn>"`); for `round`, the round number as text; for `combat`, `""`. |
| `n` | The uses since the mark, at least 1. |
| `c` | The choice made with the use, left out when there is none. |

The keys in use are `sneak_attack`, `colossus_slayer` and the feature key of each table `extra_damage` that is once per turn, all of scope `turn`.

### Expiry

Nothing resets an entry.
An entry is live only while its mark matches now: a `turn` entry while `at` is the current `turnKey`, a `round` entry while `at` is the current round, a `combat` entry always.
`featureState.live(key, now)` returns the entry only when it is live; a stale entry reads as empty.
A write drops every stale entry (`prune`), so the document does not grow with the turns of a long combat.
`now` is a `stateNow` made from the encounter (`stateNowOf`): the turn key and the round.
Because the combat's own turn and round are the clock, no job runs at the start of a turn to clear marks, and a mark is gone the moment the turn passes.
The state of a combatant goes with its row: it ends with the combat.

### No rest scope

There is no scope for a short or a long rest.
A rest is not a moment of the encounter: it happens outside combat, when a combatant's row may not exist, so an entry that waits for a rest has no clock to expire against in this column.
What a rest gives back stays where it is kept: the uses of a class resource ([Vitals, rests and class resources](rests-and-resources.md)).
The state holds only what ends with the turn, the round or the combat.

### Limits

At most 64 keys, a key of at most 80 characters, a document of at most 8192 bytes.
The database checks the type (an object) and the size (`combatants_feature_state_valid`); `featureState.validate` refuses the same limits, and the keys, with an error before the statement runs.

### Reading and writing inside `combatTx`

A read takes the document of the combatant row already loaded (`readFeatureState(row.FeatureState)`); a document that is empty, not JSON or not an object reads as an empty state.
A write is done inside the combat's transaction (`combatTx`), in one step: read the document, change the entries (`set`, `del`), and store the whole document with `saveFeatureState`, which prunes, validates and runs `SetCombatantFeatureState`.
That query replaces the document by combatant id, so a roll that marks several features changes the document once.

### Undo

An action that marks features keeps, in its event, one before/after pair for each key it touched: `actionEvent.FeatureBefore` (JSON `feature_before`) maps the key to the entry that was live before the action, or to `null` when there was none.
The undo puts each entry back, and takes the key out when it was `null`; the `after` is the document the action wrote, so the undo needs nothing more.
An event of a damage roll written with the older `once_before` (the marks of Sneak Attack, Colossus Slayer and the table) is still undone: its marks are read as entries of scope `turn` and put back by the same path (`legacyFeatureBefore`).

## Using a feature

A feature whose `grant_action` declares a `use` is taken with `FeatureService.UseFeature` (`features.go`), not `TakeAction`: `TakeAction`, the resource flows by key (`startFlow`) and the Hide calls answer `invalid_argument` ("use UseFeature") for its key.
The checks live once (`features_use.go`, `featureScene`) and `GetTurnOptions` runs the same ones (`features_options.go`), so an option listed as enabled is a call the server accepts (for the player; the master's economy override is the one difference, below).

**Order.** The request is read (ids, key, targets, choice), then inside the write transaction (`s.write`, kind `feature_used`, the session row locked, the idempotency key replayed):

1. The combat and the turn. The combatant must be a player's character the caller may act for (own player or master; an NPC is `invalid_argument`; a creature the player does not see is `not_found`). A feature that spends the action, the bonus action or nothing is used on the combatant's turn (`mustActNow`); a feature that spends the reaction is used on any turn (`gateFor` with `offTurn`), by a combatant that is not down, defeated, incapacitated or surprised and whose reaction an effect did not take away (`REACTION_USED`).
2. The feature: not on the sheet is `not_found`; a feature that declares no use is `invalid_argument`. `use_extra_action` is refused (`EXTRA_ACTION_UNAVAILABLE`, as `TakeAction` does for a feature).
3. The economy and the feature's rest resource (the row the rules work out). The master has the last word on the economy only (`masterMayOverride`).
4. The uses of the scope (`rules.ActionUse.Left(spent)`, the spent count read from the state) and of the rest resource: `NO_USES`, for the master too.
5. The targets (`rules/featureuse_targets.go`): how many (`Allows`), who (self, ally, enemy, any; "ally" is another creature of the user's side, the user only when it has no other ally), down (only a heal reaches it), the range (squares of 5 ft between the centres; the master and a combat with no grid are not held to it). A creature the caller does not see, one that is not in the combat and one the feature cannot reach all give `FEATURE_TARGET` with `combatant_ids` as the ids the caller sent (RN-10).
6. The choice: one of the feature's keys, none when it offers none, else `FEATURE_CHOICE`.

**Writes** (all in the one transaction): the combatant's economy and, for a reaction, its record (`recordReaction`); the rest resource; the heal (one server roll, shared by every target, through `healCombatant`) and the temporary hit points (`giveTempHP`); the lasting effect (`addEffects`, `sourceKind` "feature", no concentration, a duration of `effect_rounds` counted from the user's turn; the same feature used again adds its rows, as two uses of a table spell by a table feature would); the `feature_state` entry (scope, mark, count, choice) with the entry it replaced kept in `FeatureBefore`; the `feature_used` event; the revision.

**The event** is an `actionEvent` with `Feature` (`featureUseEvent`: scope, choice, per-target healed and temporary hit points, the dice faces and total, whether an effect was put on): ids, keys and numbers only. `Hits` keep each target's hit points before, `Lasting` the effect rows, `Resource` and `PointsSpent` the pool, `FeatureBefore` the state. Fog is stamped from the actor, the single target and the hits, so a use that touched a hidden creature is secret.

**Options.** `use_total` and `use_left` are the scope's (0 and 0 without a scope), `resource_total` the rest resource's, `feature_targets` the creatures the caller sees with the reason each cannot be picked (`disabled_reason_pt`: "Lado errado: só pega inimigos", "É você", "Caído", "Fora do mapa", "Fora de alcance: até 3 m"), and a row that picks creatures with none to pick is `NO_VALID_TARGET`. A reaction row is enabled off turn: the gate that disables every option for another creature's turn is replaced for it by the reaction's own gate. `feature_state` itself is never sent.

**Undo.** `feature_used` is in `undoableKinds` (it writes hit points, so it keeps holds): the economy, the resource, each target's hit points (`castHitTargetBack`), the effect rows (and the temporary hit points they gave), and the feature state are put back, and the line goes with the undo.

**Log.** `COMBAT_LOG_KIND_FEATURE_USED`, the audience of an action (the table, unless the user or a target is hidden: `hitsVisible`), the numbers (`healed`, `temp_hp`) only for the master and the user's player; the choice's text and the feature's name are read from the sheet.
