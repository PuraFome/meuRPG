# Table rules in combat

Combat applies three of the "Regras da mesa" page's rules (MR-025, RN-24): the critical hit, who sees the death saves and whether a player reads the DC of the saves their creatures roll.
A third, "combat with a map", chooses the mode of a new combat (see [Theatre of the mind](theatre.md)), and "Reações dos inimigos" is in [Reactions](reactions.md).

**The rules are read, never kept in memory.**
Each write reads them in its own transaction (`combatTx.rules`, read in `writeOnce` right after locking the session, through `play.CombatDefaults.StoredTableRules(ctx, tx, ...)`; nothing through the pool inside `InTx`).
Each read outside a transaction reads them when building the `combatViewer` (`viewerWith`, `ListCombatLog`) or the response (`GetTurnOptions`).
Every `combatTx` is born in `writeOnce` or `openTx`, which read the rules: no path forgets.
Changing the rules mid-combat holds from the next roll on; damage already opened stays as opened.

## The critical hit

`RollAttack` (the weapon attack, the spell attack, the creature's and the NPC's, which all go through it, the opportunity attack too) and `CastSpell` of an attacking spell open the damage through the same `openHit`, which asks `combat.CriticalDice` how many dice to roll and what comes without rolling.

| Rule | Pending damage's `dice_count` | `critical_max` | The damage |
| --- | --- | --- | --- |
| "Dados dobrados" (default, SRD) | Twice the attack's dice | 0 | The roll plus the bonus |
| "Máximo mais uma rolagem" | The attack's dice, once | Dice × faces | The roll, the bonus and the maximum |

`pending_damages.critical_max` and `critical_max_rule` store what comes without rolling and the rule the hit was opened with (a fixed-damage critical, with no dice, also says the rule); on a non-critical hit they are 0 and false.
`trap_damages.critical_max` does the same for a trap outside combat.
`RollDamage` rolls `dice_count` dice with the modifier `bonus + critical_max` (so `dice (faces) + modifier = total` still holds in `DiceRoll`); the physical die types only the sum of the dice that are rolled: from `dice_count` to `dice_count × faces`, without the maximum or the bonus.
The screen shows the hint from `PendingDamage.critical_rule` ("role os dados duas vezes" or "o máximo mais uma rolagem"); `GetTurnOptions` and `AttackRoll` say it before the damage; the log (`CombatLogDamage.critical_rule`, `critical_max`) says the same.
The browser does no arithmetic (`core/combat/critical.ts` writes the hint).
The attack's rule goes in the event (`critical_max_rule`), so a repeat's response says the same as the first.

A trap's critical attack (`resolveTrap`, rolled by the server at once) follows the same rule: it comes from the combat's rules or from the trigger's transaction outside it, and the maximum goes in `critical_max`, already added to the `DiceRoll`'s `roll_total` and `modifier`.

## Hidden death saves

With the rule "owner and master only", what holds is what the table had when the roll was written.
The save event and the 0-HP blow's event store `death_hidden`, and the log filters by that mark (as the fog filters by `SeenBy`), so changing the rule never reveals or hides what was already written; the combatant's counts, the current state, follow the current rule.
`combatViewer.hideDeath` and `deathHiddenFrom(combatant)` apply it.

| Path | Whoever is neither the owner nor the master receives |
| --- | --- |
| `Combatant.death_successes` and `death_failures` (`GetEncounter` and every response carrying the combat) | 0 and 0; the state is "Caído" (`DOWN`), never `DYING` (which was always the master's alone). `death_save_due` is the owner's and master's alone |
| The "Estável" and "Morto" state | The same for everyone: the result is the table's, the path to it is not |
| `ListCombatLog`, the `DEATH_SAVE` line | No line for a save that continued (success, failure, natural 20); only the one that made the character stable, with `stable` and nothing else. `DEATH_CONFIRMED` is everyone's |
| `CombatLogDamage.death_failures_added` and `PendingDamage.death_failures_added` | 0 (the `RollDamage` response to the attacking player, and the log line) |
| The stream | Only hints (`encounter_changed`, `combat_log_changed`, `turn_changed`), which never carry a save; a natural 20's `vitals_changed` goes to the owner and the master. The hint raises the combat's revision for everyone, so the signal that "something happened" stays |
| `RollDeathSave` | Only the owner (or the master) calls it and receives the save |
| Highlights and the session summary | Nothing to filter: they carry no death save |

The spell that makes a character stable (Spare the Dying) and the death the master confirms appear to everyone: they are results.
With "visible to all" (the default) nothing is filtered.
Theatre of the mind follows the same rules.

The screen reads the rule once per combat (and again when someone falls) in `GetTableRules` (`deathSaves`), only to say who else sees the marks: "Só você e o mestre" on the owner's card, "Dono e mestre" in the master's order, and a log notice for the other player.
The log line that carries only `stable` reads "estabilizou".

## The DC of a saving throw

`show_save_dc` ("Os jogadores veem a CD dos testes de resistência", RN-24) is off by default: the DC of a save is the master's, and a spell's caster's player has their own spell's.
`combatViewer.showSaveDC` carries the rule beside `hideDeath` (set in `viewerWith`, `combatTx.viewer` and `logViewer`; `activityViewer.showDC` for the trap activity), and `saveDCFor(roller)` says whether the viewer reads the DC of a save that combatant rolls: the master always, a player only when the rule is on and the combatant is theirs.
Nobody else is ever added: a player who does not roll, a bystander, a player who cannot see the caster.

| Where the DC is | Who gets it with the rule on |
| --- | --- |
| `EffectSavePrompt.dc`, a lasting effect's question and a zone's (`effectSavePrompt`, `zoneSavePrompt`) | The question's reactor, besides the master and the caster's player |
| `EffectSaveResult.dc` (the answer to `RollEffectSave` and its retry, `savedResultOf`) | The roller (the caller who answers the window) |
| `EffectSave.dc` on the card of an effect (`cardSaves`) | The player of the combatant the card is on |
| `CombatLogEffect.dc` (`effectLogRoll`) | The player of a target of the line |
| `SaveResult.dc` of a spell cast by an NPC or another character (`saveView`), in the log and the stream | The player of the target |
| `TrapSaveRoll.dc` (`trapView.showDC`), in the log and in `ListTrapActivity` | The player of the target |
| `CombatLogHitRider.dc` (Stunning Strike, Open Hand) | The player of the target (before the rule it was always given with the roll; the DC is now behind the rule) |

Unchanged: `ConcentrationSavePrompt.dc` (10 or half the damage, the owner's own), the Hellish Rebuke and Turn Undead DCs (the master's and the reactor's or cleric's), and every other DC (checks, scenes, contests).
The held roll (`HeldRollOffer`) never carries a DC.

Tests (`play`, with database): `TestRN24_TheCriticalFollowsTheTablesRule`, `TestRN24_ACreaturesCriticalFollowsTheRule`, `TestRN24_TheRulesAreReadInTheCallersTransaction`, `TestRN24_HiddenDeathSavesAreTheOwnersAndTheMasters`, `TestRN24_ARevivalAndADeathStayWithTheOwnerAndTheMaster`, `TestRN24_ADeathTheMasterConfirmsIsTheTables`, `TestRN24_VisibleDeathSavesAreUnchanged`, `TestRN24_TheTheatreFollowsTheSameRules`; the save DC rule: `TestSaveDC*` (`save_dc_rule_test.go`), and the needle through every answer in `internal/leaktest` (`savedc_test.go`); without a database, `TestCriticalDice` (`rules/combat`).
