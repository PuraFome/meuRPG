# Reactions

One mechanism covers every reaction the SRD gives a creature: the reaction window (PM-04).
A window opens when something a reaction can change happens, and the action that caused it waits: nothing of it is written until every window of its group is answered or closed.
The code is `combat_reactions.go` and `combat_reaction_*.go`; the answer is `AnswerReaction`.

## Windows

`Encounter.reaction_windows[]` is backed by two tables: `reaction_windows` (one row per reactor and trigger, with a `group_id` for the windows of the same trigger and a `step` for two-step ones) and `reaction_holds` (the request of an action that waits).

| Reaction | Trigger (SRD 5.1) | Held until answered |
| --- | --- | --- |
| Shield | A hit on the reactor, or Magic Missile | The hit's damage roll (`AWAITING_REACTION`) |
| Uncanny Dodge | A hit the rogue sees, after the damage is rolled | The damage landing: halved, rounded down |
| Deflect Missiles | A ranged weapon hit on the monk | The damage landing, less 1d10 + Dexterity + monk level; at 0 it is caught and, for 1 ki, thrown back with `RollAttack(catch_window_id)`. The server builds the attack from the missile (its dice plus Dexterity, proficiency plus Dexterity to hit, 20/60 ft) and ignores `attack_key`. The second-step prompt (`DeflectMissilesThrowPrompt`) carries the missile's name and bonus; closing that sheet without a roll passes the window (the missile is kept) |
| Hellish Rebuke | The reactor was damaged by a creature it sees within 60 ft, and survived | Nothing: it happens after. Offers the Infernal Legacy (a 2nd-level cast, once per long rest) and the slots; the NPC aggressor's save is the master's (second step) |
| Counterspell | A cast by a creature the reactor sees within 60 ft | The whole cast (held with its request and replayed after); the countered cast still spends its slot |
| Cutting Words | An attack roll (a weapon's, a cantrip's or a spell cast from a slot with one target), an ability check in combat or a damage roll of a creature of the other side that the bard sees within 60 ft | The roll (the bard answers before the die is thrown). A creature that cannot hear or is immune to charm opens no window (`cuttingWindows` skips it), and neither does one the bard's player cannot see (`reactorSees`: a hidden NPC, the fog). The sheet setting asks for checks only in "todos". The checks held are the initiator's roll of a contest (`StartContest`), the answer to a contest (`RespondContest`) and a `Hide`; the spell attack joins the Counterspell windows of its cast. See [Cutting Words on a check](#cutting-words-on-a-check) |
| Feather Fall | A fall of a pit trap (`fall_ft` of the preset) within 60 ft | The fall damage (`AWAITING_REACTION` trap damage); up to 5 creatures saved |
| Opportunity attack | A creature leaves a reach | The mover's turn (its own offers, shown as a window; see [Movement](movement.md#opportunity-attacks)) |
| Contest | A grapple, shove or escape that waits for the defender's roll or the shover's choice | Nothing: the contest row itself. Answered with `RespondContest` or `ResolveShove`, never `AnswerReaction`. Opened by `syncContests`, closed by itself when the contest no longer waits (see [Contests](contests.md)) |
| Hidden reveal | A player's area spell hit hidden creatures and the table asks ("Perguntar a cada vez") | The turn: the master reads it as a window (`HIDDEN_REVEAL`) and answers with `ResolveHiddenReveal` |
| Concentration save | Damage that lands on a concentrating creature (DC 10 or half the damage); 0 HP ends it with no save | The attack that dealt the damage (`CONCENTRATION_SAVE_PENDING`), answered with `ResolveConcentrationSave` (the app's die, a physical die, "Deixar o mestre rolar por mim" or the master's "keep") |
| Effect save | An end-of-turn or start-of-turn save of a lasting effect (`EFFECT_SAVE`) | The turn (see [Lasting effects](lasting-effects.md)) |
| Master check | The table rule "Reações dos inimigos" set to "Sempre" (`MASTER_CHECK`): a one-tap check on every attack, hit, cast and damage of a player against an enemy, so the pause says nothing | The action |

`AnswerReaction` answers a window: "Usar" (with the slot, `use_racial`, the die, the creatures of a Feather Fall) or "Deixar passar".
It is the reactor's player, or the master for an NPC's and, to answer for the player, any.
`UseReaction` and `DeclineReaction` are the Shield's two answers kept for clients that still send them; they act as `AnswerReaction` on the hit's Shield window.

## Shield

When a non-critical blow hits a player character who can cast Shield now (prepared, with a free slot, reaction free, standing), the pending damage is born `AWAITING_REACTION`, not `AWAITING_ROLL`.
The attacker sees "esperando a reação"; the hit player and the master get the notice in `GetEncounter` (`reaction_prompts`, by who sees: the master all, the player their own character's, without the attacker and without the blow's total).
The player decides without knowing the total, as at a table.
The attacker's sheet follows the answer: on each `encounter_changed` it reads `GetTurnOptions` again, a pending damage that is gone (Shield stopped the hit) turns the result into a miss, and one back in `AWAITING_ROLL` goes on to the damage.
The player learns only that, never the armor class.

```mermaid
sequenceDiagram
    actor M as Master (attacks)
    participant S as play (server)
    actor J as Hit player
    M->>S: RollAttack(Captain, target: Pensantus)
    S->>S: the d20 beats the AC, not a critical, and the target can cast Shield
    S-->>M: Hit, pending damage AWAITING_REACTION
    S-->>J: encounter_changed, reaction_prompts in GetEncounter
    alt The player uses Shield
        J->>S: AnswerReaction(window, Usar, slot level)
        S->>S: spends the slot and the reaction, ac_bonus = 5, compares the stored total with AC + 5
        alt The total fell short
            S-->>J: STOPPED, the pending damage is discarded and the log says Shield held it
        else Still hits
            S-->>J: STILL_HIT, the pending damage goes to AWAITING_ROLL
        end
    else They let it through, or the master answers for them
        J->>S: AnswerReaction(window, Deixar passar)
        S-->>M: the pending damage goes to AWAITING_ROLL
    end
    M->>S: RollDamage(pending_id)
```

`RollDamage` and `ApplyPendingDamage` on a blow waiting for the reaction give `REACTION_PENDING`; the attacker's `EndTurn` gives `PENDING_DAMAGE` (only the master passes, discarding).
The +5 stays in `combatants.ac_bonus` until the start of the character's next turn (`ResetCombatantTurn`): every attack in the meantime uses the sheet's AC plus the bonus, and the bonus never alters the sheet (RN-04).
The opportunity attack is `RollAttack` with `as_reaction`: a melee attack off turn spending the reaction in place of the action.

## Cutting Words on a check

A check is held as an attack roll is (`combat_cutting_checks.go`): `holdCheck` opens the Cutting Words windows with `Roll: "test"`, keeps the request in a hold of kind `contest`, `contest_answer` or `hide`, and the handler returns before anything is spent or rolled.
When the windows are answered the replay runs `startContest`, `respondContest` or `hide` again as the original caller, and `rollFor` takes the die off the total (`cutRoll`) and adds a note "Palavras de Interrupção: −N" to the roll, so the contest compares the cut number.
A spell attack from a slot is held in `holdCast` beside the Counterspell windows (a single target only, because the die cuts one roll) and the replay lowers the spell's attack bonus, as `RollAttack` does for a weapon.
The window opens before the roll, so it names the roller and never a total; a roller the bard's player does not see opens none, and in "Sempre" a player's check against an enemy waits for the master's check when no bard could cut it.
The rolls with no window: a check outside a combat (a group check, a scene check, a trap search, a puzzle hint) has no encounter to hold a window in, and a roll the player leaves to the master (`defer_to_master`) is held when the master rolls it.
The Help action rolls nothing.

## The settle step and the gate

After every write of a combat, `settleReactions` runs:

1. it closes the windows that stopped being valid (the reactor spent the reaction, fell or is incapacitated, the trigger is gone) with a reason (`reaction.Closure`);
2. it lets a hit whose windows are all closed go on to its damage roll;
3. it replays the held actions in the combat's transaction (the same handler as the RPC, with what the answers decided in `replayState`: the countered cast, the Cutting Words die, the halved or deflected damage).
   Replaying a held request with its idempotency key returns the same answer.

`reactionGate` refuses `EndTurn`, a second action, `RollDamage` and `ApplyPendingDamage` with `REACTION_PENDING` (or `CONCENTRATION_SAVE_PENDING`) while a window waits, for the master too (who ends the combat or answers).
Windows are answered in the initiative order inside a group (`NOT_YOUR_TURN_TO_ANSWER` otherwise); two answers at once settle to one.
A window the reactor can no longer use closes by itself and says so, in words, only to its own reactor.
Ending the combat discards every window and held action.

## The automatic pass (client side)

An optional reaction a player does not answer passes after 30 seconds.
The server keeps no timer: the master's screen (`core/combat/reaction-autopass.ts`, driven by `combat-view`) remembers when it first saw each open window of kind Shield, Uncanny Dodge, Hellish Rebuke (the cast), Counterspell, Cutting Words, Deflect Missiles or Feather Fall waiting for its player (`reactor_is_player`, `answer_now`), and after `AUTO_PASS_SECONDS` (30) sends the same `AnswerReaction` "Deixar passar" the master can tap, with the idempotency key `auto-pass:<window id>`, so two tabs or a retry never answer twice; a refusal because it was answered meanwhile is ignored.
Saving throws (`CONCENTRATION_SAVE`, `EFFECT_SAVE`, the Hellish Rebuke second step), contests and the master's check never auto-pass.
A reload starts the 30 s again (in memory only).
The player's prompt only shows the countdown; their client never answers by itself.

**The player is rolling.** A physical die takes longer than 30 s to find, roll and type, so when a player taps "Usar" on an optional reaction and moves to the die step, their screen calls `MarkReactionRolling` (the reactor's player alone; others get `permission_denied`).
It is idempotent, writes no event or log line, and stores `rolling` in the window's `trigger` JSON.
The window then carries `rolling` for the master and the reactor, and the master's clock (`AutoPassClock`) restarts the wait from when it first saw the mark with `ROLLING_WAIT_SECONDS` (120), still bounded: after it the window passes like any other, and the master can pass it by hand at any time.
The player reads the same countdown on the die step; the master's card reads "Rolando… passa sozinho em N s".
If the call fails the roll goes on with the 30 s clock.

## Who reads what (RN-10, RN-20)

A player gets only their own windows (`for_you`) and `reaction_wait`, the line the server writes for the whole table ("Esperando o mestre", "Esperando a reação de Pensantus": the name only when the reactor is a player character the reader sees).
Never which NPC, which reaction or why.
The Counterspell prompt carries no spell name or level; a Cutting Words prompt to a bard has no total; the numbers of the trigger (`ReactionTrigger`) are the master's.
`reaction_window_opened` and `reaction_window_closed` go only to the reactor and the master.
The table rule "Reações dos inimigos" (`enemy_reactions`) defaults to opening a window only for an enemy that can react; "Sempre" opens the master's check on every one.
Hellish Rebuke's second-step prompt carries the aggressor's modifier only for the master.

## The log and undo

Each reaction is one line (`COMBAT_LOG_KIND_REACTION_WINDOW`) with the master's and the players' text; `reaction_answered` and `concentration_save_rolled` are the session event kinds.
A used Shield keeps `reaction_used` (undoable).
An undo reopens its window, closes the windows of the damage it takes back and gives an NPC's slot back; the undo of the master's "Não aplicar" on a hit that waited reopens the windows the discard closed (`closed_windows`; see [Undo](undo.md)).
A held action's event closes the undo chain.

## What the SRD leaves out

The app decides, and says so:

- the triggers' "see" and 60 ft come from the Player's Handbook reaction texts;
- a reaction spell cannot be countered by the app;
- an NPC bard's ability-check rolls are not asked (no flow rolls an enemy's checks);
- Hellish Rebuke's fire is always rolled in the app;
- Deflect Missiles' free-hand condition is not checked;
- Feather Fall's trigger is the pit trap (a creature thrown into a fall by another effect has no window).
