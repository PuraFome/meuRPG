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

(filled by the UseFeature task)
