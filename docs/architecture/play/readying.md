# Readying an action

The standard action Ready (SRD 5.1, "Ready") holds another action until a trigger the player names.
The code is `ready*.go` in `internal/play`; the contract is `ReadyService` in `ready.proto`.
The rules for the table are in [RN-31](../../product/rules.md#rn-31-reactions-and-the-reaction-window).

## Model

A readied action is a `readied_actions` row (migration 00330).
It holds the readier, the trigger text (1 to 200 characters), the kind, the held action and its name, the target named at Ready and the move.
The kinds are `attack`, `move`, `other` and `spell`.
`attack` holds one of the readier's attacks by its key.
`other` holds a standard action other than Attack, Cast a Spell and Ready.
Ready also refuses Hide and Help, which have their own calls a reaction does not run.
It refuses Dash and Disengage too: they last only for the readier's own turn, and no reaction reaches it.
Each of them would be taken for nothing.
`move` holds the speed the readier had at Ready (`move_ft`).
A combat without a map (theatre of the mind) refuses a readied move (`READY_ACTION`): there is no square to walk to.
`spell` is refused at Ready (`READY_ACTION`) for now.
A combatant has at most one waiting row.
`Combatant.readied` (119) is the waiting row, for the master and the readier's player.

## States

| Status | How it gets there |
| --- | --- |
| `waiting` | `ReadyAction` on the readier's turn spends the action and writes the row |
| `fired` | The readier used its READIED window ("Usar") |
| `ignored` | The readier let its READIED window pass ("Deixar passar"), or it passed by itself after 30 s |
| `lost` | The readier's next turn started, or it was defeated or dismissed (`loseReadiedOfDefeated`, in the settle step of every change) |
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
  The target must be one the attacker can attack now (reach, sight).
  Otherwise the attack's own refusal comes back and the window stays open.
- **move**: `MoveCombatant` off the readier's turn, while its READIED move window is open.
  It walks at most the lesser of `move_ft` and its speed now.
  It pays its way like a move on turn and spends no movement of the turn.
  Move's own refusals apply: a grappled readier has no speed (`TOO_FAR`).
  A refused move spends nothing and the window stays open.
  A move that stops short (a wall, a creature in the way, a trap) is still the use, and the window ends.
  It offers opportunity attacks as any walk does: only a forced move, a teleport or Disengage would not.
  It is held by a pending hidden reveal (`mustNotHold`) as a move on turn is.
  The master's walk of the readier while the window is open is the use too, as if the player made it.
  `AnswerReaction` "Usar" on a move window is `invalid_argument`: it carries no square.
- **other**: the `TakeAction` path, charged to the reaction instead of the action.

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


The trigger and the held action are the master's and the readier's player's: on `Combatant.readied`, the window and the log lines.
The window carries no prompt; the master's `ReactionTrigger` names the readier, the target and the held action.
Another player reads only `reaction_wait`: "Esperando a reação de <name>" for a player character they see, else "Esperando o mestre".
Another player reads the lines (`ACTION_READIED`, `READIED_FIRED`, `READIED_LOST`) with the actor alone.
A hidden readier's window and lines are the master's alone.
`internal/leaktest` (`ready_test.go`, `ready_fire_test.go`) checks each of these with canaries and positive controls.

## Undo

The undo of a Ready gives the action back and deletes the row and its line.
The Ready of a row already lost cannot be undone (`chainReadied`): the lost row and its line stay.
The undo of a use takes back the action, the line and the reaction, makes the row wait and opens its window again.
The undo of a pass makes the row wait and opens its window again.
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
