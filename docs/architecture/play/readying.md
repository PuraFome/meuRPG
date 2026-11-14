# Readying an action

The standard action Ready (SRD 5.1, "Ready") holds another action until a trigger the player names.
The code is `ready*.go` in `internal/play`; the contract is `ReadyService` in `ready.proto`.
The rules for the table are in [RN-31](../../product/rules.md#rn-31-reactions-and-the-reaction-window).

## Model

A readied action is a `readied_actions` row (migration 00330).
It holds the readier, the trigger text (1 to 200 characters), the kind, the held action and its name, the target named at Ready and the move.
A spell row also holds its slot level (`slot_level`, 0 for a cantrip) and the cast id of its Ready (`spell_cast_id`).
The kinds are `attack`, `move`, `other` and `spell`.
`attack` holds one of the readier's attacks by its key.
`other` holds a standard action other than Attack, Cast a Spell and Ready.
Ready also refuses Hide and Help, which have their own calls a reaction does not run.
It refuses Dash and Disengage too: they last only for the readier's own turn, and no reaction reaches it.
Each of them would be taken for nothing.
`move` holds the speed the readier had at Ready (`move_ft`).
A combat without a map (theatre of the mind) refuses a readied move (`READY_ACTION`): there is no square to walk to.
`spell` holds one of the readier's class spells by its key: see [Readied spells](#readied-spells).
A combatant has at most one waiting row.
`Combatant.readied` (119) is the waiting row, for the master and the readier's player.

## States

| Status | How it gets there |
| --- | --- |
| `waiting` | `ReadyAction` on the readier's turn spends the action and writes the row |
| `fired` | The readier used its READIED window ("Usar") |
| `ignored` | The readier let its READIED window pass ("Deixar passar"), or it passed by itself after 30 s |
| `lost` | The readier's next turn started, or it was defeated or dismissed (`loseReadiedOfDefeated`, in the settle step of every change) |
| `broken` | The concentration that held a readied spell ended (`settleReadiedSpells`) |
| `canceled` | The readier's player gave it up on the readier's turn (`CancelReadied`) |

A row that does not wait is history: no call changes it again, and the undo is the only way back to `waiting`.
The end of the combat drops every row.

## Flow

1. On its turn the readier calls `ReadyAction`: the action is spent and the row waits.
2. When the trigger happens, the master calls `FireReadied` (master only).
3. `FireReadied` opens a reaction window of kind `READIED` for the readier, in its own group, through `openWindows`.
4. The window holds the table like any other: `reactionGate` answers `REACTION_PENDING` until it is answered or closes.
5. The readier's player answers it, or the master for an NPC, with `AnswerReaction` or, for a move, `MoveCombatant`.

`FireReadied` refuses, in this order and writing nothing:

- a row that does not wait, or a readier out of the order (dismissed): `not_found`;
- a defeated readier, or one at 0 hit points: `COMBATANT_DOWN` (`mustReactNowOrOnTurn`);
- a surprised readier before its first turn ends: the surprise refusal of every reaction;
- an incapacitated or unconscious readier, or one an effect keeps from reacting: `CANNOT_ACT`;
- a readier whose reaction is spent: `REACTION_USED`;
- a row whose window is already open: `REACTION_PENDING`.

A refused fire leaves the row waiting until it is lost.
The fire event (`readied_fired` with `opened`) writes no log line.

## The answers

"Usar" runs the held action with the reaction, through the action's own path:

- **attack**: `RollAttack` as a reaction (`as_reaction`), with the cover and the fog as usual.
  `AnswerReaction` builds the request from the row.
  The target is the one named at Ready, or `creature_ids[0]`.
  The d20 is the typed one, or the app's.
  `RollAttack` with `catch_window_id` set to the READIED window does the same.
  The roll waits for the windows any attack roll opens first: Cutting Words, Protection, the master's check of "Sempre" (`holdAttackRoll`).
  Meanwhile the READIED window stays open and the row waits; `AnswerReaction` answers `used` false.
  While the roll is held, the window takes no answer: a second "Usar" and "Deixar passar" are `REACTION_PENDING` (`mustNotBeHeld`).
  The master reads it with `answer_now` false, so the automatic pass leaves it alone.
  A held roll that the attack refuses once replayed is dropped (`settleReplay`), as when the target fell meanwhile.
  The reaction stays free, the row waits and the window takes an answer again.
  The master reads the drop in the log (`held_dropped`); the readier's player reads only the window again.
  A readier that falls meanwhile loses the row, and its window closes.
  The target must be one the attacker can attack now (reach, sight).
  Otherwise the attack's own refusal comes back and the window stays open.
- **move**: `MoveCombatant` off the readier's turn, while its READIED move window is open.
  It walks at most the lesser of `move_ft` and its speed now.
  It pays its way like a move on turn and spends no movement of the turn.
  Move's own refusals apply: a grappled readier has no speed (`TOO_FAR`).
  A refused move spends nothing and the window stays open.
  A move that stops short (a wall, a creature in the way, a trap) is still the use, and the window ends.
  It offers opportunity attacks as any walk does: only a forced move or a teleport would not.
  A Disengage does not cover it: Disengage lasts only for the turn it was taken on (SRD 5.1).
  Its offers hold the end of the turn in progress, the master's too: `EndTurn` is `OPPORTUNITY_PENDING` until each is answered.
  So none of them outlives that turn (`mustNotWaitForReadiedOffers`).
  It is held by a pending hidden reveal (`mustNotHold`) as a move on turn is.
  The master's walk of the readier while the window is open is the use too, as if the player made it.
  `AnswerReaction` "Usar" on a move window is `invalid_argument`: it carries no square.
- **other**: the `TakeAction` path, charged to the reaction instead of the action.
- **spell**: the `CastSpell` path as a reaction: see [Readied spells](#readied-spells).

A use closes the window as answered, marks the row `fired` and writes the `readied_fired` line just before the action's own event.
It records the reaction's kind as `readied` (`recordReaction`).
The attack spends the reaction in its own economy, as an opportunity attack does.
Another action spends it in `TakeAction`'s economy, charged to the reaction.
A move spends it with `spendReactionAs`.
A refused use spends nothing and the window stays open: the readier may still pass.
"Deixar passar" marks the row `ignored`, keeps the reaction and writes the `readied_fired` line (status `IGNORED`).

The window closes by itself, with the reason told to its reactor alone.
That happens when the readier is defeated or incapacitated, spends its reaction elsewhere, or the row stops waiting.

The automatic pass of 30 s counts on the master's screen from when the window opened.
It can pass a window whose player is still choosing a square for a move, or a target: the row is then ignored.

## The reaction (Q1)

The reaction is spent only when the readied action is used.
While the row waits, the readier's other reactions stay offered.
A readier that spent its reaction elsewhere cannot be fired, and the row waits until it is lost.
A lost row gives nothing back: the action spent at Ready stays spent.

## Who reads what (RN-10)

A player who is not the reactor's reads a READIED window as one that does not exist.
`AnswerReaction` checks the caller before it reads what the row holds (`mayAnswerReadied`).
`RollAttack` refuses another's READIED window in `catch_window_id` as a window it does not find.
A `catch_window_id` that is not a UUID is `invalid_argument`.

The trigger and the held action are the master's and the readier's player's: on `Combatant.readied`, the window and the log lines.
The window carries no prompt; the master's `ReactionTrigger` names the readier, the target and the held action.
Another player reads only `reaction_wait`: "Esperando a reação de <name>" for a player character they see, else "Esperando o mestre".
Another player reads the lines (`ACTION_READIED`, `READIED_FIRED`, `READIED_LOST`) with the actor alone.
A hidden readier's window and lines are the master's alone.
`internal/leaktest` (`ready_test.go`, `ready_fire_test.go`, `ready_spell_test.go`) checks each of these with canaries and positive controls.

## Readied spells

A readied spell is cast at Ready and released by the trigger (SRD 5.1, "Ready").
The code is `ready_spell.go` and `ready_spell_lost.go`.

### At Ready

`ReadyAction` with `kind` `SPELL` names the spell (`spell_key`) and the slot level (`slot_level`, 0 for a cantrip).
It checks the spell as `CastSpell` does (`checkSpell`): a class spell the caster knows or prepared, a slot of that level, silence.
Metamagic and spells granted by a feature are not offered.
The casting time must be exactly 1 action, else `READY_SPELL_TIME` (a reaction or a bonus action spell, a ritual is never cast in combat).
A spell attack cantrip (Fire Bolt) is refused as a spell (`invalid_argument`): it is readied as an `attack`.
A pact slot is spent only when no plain slot of that level is free.
Targets and range are checked at the release, not at Ready.
Ready spends the slot and the action, and notes the spell cast this turn.
It sets the readier's concentration to the sentinel `readied` (`concentrationReadied`), even for a spell without concentration.
A previous concentration ends there, with its line and its dismissed creatures, as a new concentration spell ends it.
No effect of the spell happens: no damage, no lasting effect, no zone, no `spell_casts` record.

### The release

The release is the spell's cast as a reaction, through `castSpell` with `readied` set (`findReadied`).
`AnswerReaction` "Usar" builds the `CastSpellRequest` from the row: the targets are `creature_ids`, or the target named at Ready.
A spell placed on the map (an area, a wall) is released with `CastSpell` by the readier, with its point, while the READIED window is open.
The release spends no slot, the reaction instead of the action, and takes the slot level of Ready.
Metamagic at the release is `invalid_argument`.
A concentration spell keeps the concentration, now with its own key; another spell ends it.
The cast waits for the windows any cast opens (Counterspell), with the READIED window open and `REACTION_PENDING` (`readiedHeld`).
A countered release is the use: the row is `fired`, the reaction spent, the concentration ended and the slot lost.
A refused release spends nothing and the window stays open.
"Deixar passar" loses the spell: the row is `ignored`, a `READIED_LOST` line names the spell, and the concentration ends.

### When it is lost

A held spell and its concentration go together, checked in the settle step of every change (`settleReadiedSpells`).
A waiting spell whose readier no longer holds the sentinel is `broken`, with a `READIED_LOST` line (status `BROKEN`).
That covers a failed concentration save, 0 hit points (`endConcentrationOfDownCharacter` calls it too), an incapacitating condition, a new concentration spell, and the player or the master ending the concentration.
A readier that holds the sentinel with no waiting spell stops concentrating, with no line of its own.
That covers the next turn, a defeat, a dismissal, `CancelReadied` and the pass, whose own lines name the loss.
The slot stays spent in every case.

### Who reads it

The held spell's key and name, its level and that it is a spell are the master's and the readier's player's (RN-10).
`Combatant.concentration_spell` carries the held key for them and nothing for the others (`heldSpellView`).
The `ACTION_READIED` line carries the slot (`spell.slot`) for them alone.
The end of the previous concentration (`concentration_ended_key`) is public on that line, as on a cast line.
The concentration save lines of a held spell are the master's and the owner's; another player reads none.
Their concentration save line names it "a magia preparada".
`internal/leaktest` (`ready_spell_test.go`) checks these with canaries and positive controls.

## Undo

The undo of a Ready gives the action back and deletes the row and its line.
The Ready of a row already lost cannot be undone (`chainReadied`): the lost row and its line stay.
The undo of a use takes back the action, the line and the reaction, makes the row wait and opens its window again.
The undo of a pass makes the row wait and opens its window again.
The undo of the Ready of a spell also gives back the slot, the spell cast this turn and the concentration it ended, with its creatures.
The undo of a spell's release is the cast's undo, the slot excepted, and holds the spell again with the sentinel.
The undo of a spell's pass holds the spell again.
The undo of a change that broke a held spell makes the row wait again when it gives the concentration back (`brokenReadiedBack`), and takes the line with it.
Otherwise the row stays broken: `chainReadiedDefeat` goes past its line.
The undo of the fire deletes its window while it was not answered (open, or closed by itself); the row keeps waiting.
A fire whose window was answered comes back after the answer's undo.
The lost line of a defeat or a dismissal is written before the change's own event; the undo of that change goes past it and the row stays lost.

## Events and log

| Event | Line | Undo |
| --- | --- | --- |
| `action_readied` | `ACTION_READIED` (104) | Yes, unless the row was lost since |
| `readied_fired` with `opened` (the fire) | None | Yes, while its window was not answered |
| `readied_fired` with `ended: ignored` (the pass) | `READIED_FIRED` (105), status `IGNORED` | Yes |
| The used action's event with `readied.ended: fired` | `READIED_FIRED` (105), status `FIRED`, before the action's own line | Yes, with the action |
| `readied_lost` | `READIED_LOST` (106) | Passed over by the undo of the change that defeated or dismissed the readier |
| `readied_lost` with `ended: broken` | `READIED_LOST` (106), status `BROKEN` | With the change that ended the concentration, when it gives it back |
| `readied_lost` with `ended: ignored` (a spell's pass) | `READIED_LOST` (106), status `IGNORED` | With the pass |
