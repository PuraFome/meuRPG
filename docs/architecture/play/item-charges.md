# Item charges and ammunition

The charges of a magic item and the ammunition of a character are spent, rolled and recovered through `ChargesService` (`proto/meurpg/play/v1/charges.proto`), which `play.Service` implements.
What the sheet holds (`SheetMagicItem.charges_left`, `lost`, `last_roll_pending`, and `FullSheet.ammunition`) belongs to the `characters` module.
`play` reaches it through `play.ItemKeeper`, which `characters.Service` implements and `VitalsKeeper` embeds.
The code is `charges.go` and `charges_events.go`.
The rule is in [the product rules](../../product/rules.md).

## Who may call

- `SpendCharges`, `RollLastCharge` and `RecoverAmmunition` are for the character's player and the master.
- A dead character is refused with `CHARACTER_DEAD`.
- The calls work on a locked sheet and need an open game session, in a combat and out of one.
- `SetCharges` and `Dawn` are the master's and have no player.
- `SetAmmunition` needs no session and writes no event: the owner may while the sheet is open to editing, the master always.
- Spending charges is never an action of the turn.

## Spending

`SpendCharges` takes the charges in one transaction with the idempotency key.
It refuses with `CharacterBlocked` `ITEM_DESTROYED`, `ITEM_NO_CHARGES` (none left, or a rolled start the master has not set) and `CHARGES_NOT_ENOUGH`.
The last charge of an item that is lost at once destroys it and writes `item_destroyed`.
The last charge of an item that rolls a d20 sets `last_roll_pending` and waits for `RollLastCharge`.

## The last charge

`RollLastCharge` works only while `last_roll_pending` is true.
The d20 follows the table's dice mode ([Table rules](table-rules.md)): the app rolls it, or the player types the face of a real die.
A 1 loses the item as the catalog says, and a 20 on an item that regains charges gives back a die of them, rolled the same way.
A typed 20 without the regain die's face is refused with `ChargesBlocked` `REGAIN_FACE_NEEDED`.
Any other face only ends the wait.
Two calls with the same idempotency key give one roll: the second reads the first one's answer.
Two calls with different keys give one success: the one that gets the lock second finds no wait and is refused with `invalid_argument` "the item has no last charge to roll", with no blocked reason, also when the first roll destroyed the item.

## Undo

A charge cannot be given back by the master's undo.
`item_charge_spent`, `item_charges_restored` and `item_destroyed` are not undoable kinds, so each of them closes the undo chain, and the attack before it can no longer be undone ([Undo](undo.md)).
The master corrects the charges with `SetCharges`.

## The master's correction

`SetCharges` sets the charges left (0 to the maximum, which is also how a rolled start gets its count) and whether the item was lost.
It ends a wait for the d20, writes `item_charges_restored` (or `item_destroyed` when the item is lost) and no line of the combat log.

## The dawn

`Dawn` is the master's "Amanhecer": it gives the items of every character of the campaign their dawn.
A long rest with `TakeRestRequest.restore_dawn` does the same in the rest's own transaction, after the characters' rest and before the rest's event.
A short rest ignores the flag.
The numbers are the keeper's `RestoreDawn` (`characters/dawn.go`), which takes a `link.DawnRoll` and returns the `link.DawnItem`s that regained charges.
`dawn.go` in play rolls the dice with `Service.roller` and writes the events.
The dice are always the app's, also at a table of real dice (RN-18): the master rolls a dawn for the table, as the app rolls the NPCs' dice, and no field of the request types a face.
For each stored row of an entry on a sheet:

- A lost item, an item that waits for its last d20 and an item that never recharges are left as they are.
  An item waiting for its d20 keeps `last_roll_pending` and regains nothing until the roll is made.
- An item that regains charges at dawn gets all of them back or the catalog's dice, cut to the maximum, and an item at its maximum changes nothing and writes nothing.
- An item that waits for dawns after its last charge counts one when it has no charge left, is full on the last dawn of its wait (the figurines wait 7) and starts counting again from 0.
  An item with a charge left counts none, and a count left over is cleared.
- An item that rolls its start and has no row has no count yet, so it is skipped.

The keeper reads the campaign's rows (`ListCampaignItemCharges`), locks each character that has one in the order of their ids, like a spend does, and reads the rows again after the lock, so a spend and a dawn on one item take turns.
Each item that regained charges writes one `item_charges_restored` session event (`regained` outcome, no key, a plain session event and never a line of a combat's log).
The idempotency of a rest is the rest's key: a retry replays the stored `rest_taken` event and never restores again.
`Dawn` has its own key, stored with the `dawn_taken` event, which also lists the items that regained charges, so the retry answers with them as they stand and a dawn that only counted a day for the figurines is not counted twice.
A call that changes nothing still writes `dawn_taken`.
The events are the master's and the owner's, as the charges are (RN-10).
The call tells the streams that the sheets of the characters changed (`vitals_changed`, to the master and the owner), and nothing else.

## Ammunition

`SetAmmunition` sets the pieces of one kind, 0 to 9999.
A kind is counted once the character has a row for it, also at 0, and S2 has no way to remove a kind.
A kind with no row is not counted.
`RecoverAmmunition` takes back half, rounded down, of what each kind fired in the character's latest combat, once for each combat.
It refuses with `ChargesBlocked` `COMBAT_RUNNING` while that combat is not ended, and `ALREADY_RECOVERED` after it ran.
A combat that fired nothing is skipped, and the recovery goes to the newest earlier ended combat that fired some, among the latest ten.
A character that fired nothing in any of them is `invalid_argument`.
It writes the session event `ammunition_recovered`.

## Ammunition in combat

A ranged attack with a weapon that fires ammunition takes one piece in the attack's transaction ([Attacks and damage](attacks-and-damage.md#the-two-step-attack)).
A kind with no row spends nothing: the keeper's `SpendAmmunition` answers `counted` false, and the shot writes no `ammunition_spent` event, sets no `ammo` on the attack's event and so refunds nothing on undo.
The keeper's `SpendAmmunition` starts the count of a combat over (nothing fired, not recovered) when the combat is not the one it last counted.
With none left the attack is refused with `EncounterBlocked` `AMMUNITION_EMPTY`.
Each shot writes the session event `ammunition_spent` with the combat's round, which reads as the line `AMMUNITION_SPENT`.
The undo of the attack gives the piece back: the keeper's `RefundAmmunition` adds it to the stock and lowers the count of what the combat fired, in the undo's transaction, and the line leaves the log with the attack.
It locks the stock row like the shot does, so a refund and a shot at the same time take turns.
When the stock row counts another combat only the piece comes back, and a row left with nothing fired belongs to no combat again.

## Events

The events are `item_charge_spent`, `item_charges_restored`, `item_destroyed`, `dawn_taken`, `ammunition_spent` and `ammunition_recovered`.
In a combat that has the character, `SpendCharges` writes `COMBAT_LOG_KIND_ITEM_CHARGE_SPENT` and `RollLastCharge` writes `COMBAT_LOG_KIND_ITEM_LAST_CHARGE_ROLLED`.
An event of `ammunition_spent` reads as `COMBAT_LOG_KIND_AMMUNITION_SPENT`.
The web words the three lines in `core/combat/combat-log-item-use.ts` ("gastou a última carga da Varinha de teia", "rolou 1 no d20 da última carga: ...", "atirou com o Arco longo: 1 flecha, restam 3").
`SetCharges`, `RecoverAmmunition` and any call outside a combat write the session event only, and no line.

## Consumables

`UseConsumable` uses up one unit of a consumable of the sheet: a potion, a scroll or another item of `effects/consumables.json`.
One entry of `FullSheet.magic_items` is one unit, and the keeper's `RemoveConsumable` takes it off the sheet in the call's transaction.
The code is `consumables.go`, and the dice, the healing, the resistance and the payload are in `play/consumable`.
The character's player and the master may call it, a dead user or a dead receiver is refused with `CHARACTER_DEAD`, and it needs an open game session.
The receiver is the user, or `target_character_id` for a gift: the user's entry is the one spent, and the effect is the receiver's.
Out of a combat there is no turn and no adjacency.
A user or a receiver that fights in the running combat is refused with `ChargesBlocked` `COMBAT_RUNNING` (`canUseConsumable` is the seam for the combat's own rules, and the combat's line is `COMBAT_LOG_KIND_CONSUMABLE_USED`).
`rules.Content.PotionEffect` says what a potion does (`effects/potions.json`).
A healing potion rolls its dice and heals the receiver up to the maximum, and the response says the hit points really regained.
The dice follow the table's dice mode like the d20 of the last charge: the app rolls them, or `typed_faces` holds one face for each die, each a face of the die.
A resistance potion puts a record of `character_effects` on the receiver with the damage type and 3600 seconds of game time, the master's and the owner's to read.
The record counts down with `AdvanceGameTime` like any effect outside a combat, and a second potion of the same kind starts the hour again.
Any other consumable only spends the unit, and the response returns the item's text.
`consumable_used` is written with the key of the call, so a retry returns the first answer and spends nothing again.
Two calls with different keys on one entry give one use: the second finds no entry and is refused with `not_found`.
`consumable_used` is not an undoable kind, so it closes the undo chain ([Undo](undo.md)).
The sheet's `effect_pt` says what a potion does ("Cura 2d4 + 2"), and `spell_key` and `spell_name_pt` are for the spell of a scroll, empty until scrolls are cast.

## Privacy

Charges, ammunition and these lines are the master's and the owning player's alone (RN-10).
No other player reads the line, the event (secret) or the hint that the stream carries.
The use of a consumable and the resistance it leaves are the same: the master and the owner read them.
In a combat the line is also the receiver's owner's, and a hidden receiver is not named to a player who is not its owner.
The leak tests are `internal/leaktest/itemuse_test.go` and `internal/leaktest/consumables_test.go`.

## Refusals

`ChargesBlocked` (`charges.proto`) carries `REGAIN_FACE_NEEDED` (with the die's size in `needed`), `COMBAT_RUNNING` and `ALREADY_RECOVERED`.
The web reads it in `core/play/charges-errors.ts`.

## The character sheet

`ChargesClient` (`core/play/charges-client.ts`) wraps the six calls and reads the table's dice (`GetCampaign`).
`charges-errors.ts` words each refusal in Portuguese, by the typed reason: the item's own refusals name the item, and a call that did not get through says nothing changed.
`core/characters/charges.ts` holds the pure parts: an item's state, its words, the four SRD kinds of ammunition, the recovery and the free-text hint.
`SheetCharges` (`pages/character-sheet/charges/sheet-charges.ts`) is built by `createSheetMagicItems` and reads the sheet again after each write.
`EquipmentPanel` draws the item rows with charges, the "Munição" section and the live regions of their results.
A row with `charges_max` 0 is drawn as before.
"Usar" and "Rolar" work on a locked sheet and for the master; "Adicionar munição" follows the right to edit.
"Usar" opens `UseChargesSheet`, which sends the amount under one idempotency key and keeps it across retries.
The last charge opens `LastChargeSheet`, an alert dialog that follows the table's dice (RN-18): the app rolls, or the player types "Face do d20", and a typed 20 may ask for the regain die's face.
Its body says what a 1 and a 20 do for that item, from `SheetMagicItem.recharge` (`lastChargeBody` in `recharge-text.ts`).
The close button and Esc close it and the backdrop does not; closing leaves the row asking "Rolar".
The use sheet and the d20 sheet do not restore the focus themselves: `SheetCharges.focusRow` puts it, with the focus ring, on the item's "Usar" or "Rolar" (or the row itself when the item is lost) once the sheet has been drawn again.
`AddAmmunitionSheet` sends the pieces to add, and `SheetCharges` reads the sheet again and sends the total, since `SetAmmunition` has no delta.
"Recuperar a metade" is off with nothing fired and after the recovery, and goes off during a combat when the server refuses with `COMBAT_RUNNING`.
The free-text hint shows only while the ammunition list is empty and the items text names an arrow, bolt, sling bullet or needle; the text is never converted.
An item row with `SheetMagicItem.recharge` says when its charges return ("Volta 1d6 + 1 ao amanhecer", "Não volta: ..."), the dice of a rolled start, and what a lost item became ("Virou um bordão comum.", "Ficou só o +2 no ataque e no dano.").
The use sheet repeats the recharge line with the most the item holds, and "Sem cargas" names when the item comes back.
A sheet with charges words its lock line as "Travada: usar cargas continua valendo, o resto, só o mestre muda."
The ammunition kinds (`AMMUNITION_KINDS`) also name the catalog weapons that fire them, so the attack row names its kind by the weapon's key.

## The master's panel

`PartyChargesPanel` (`pages/live-session/party-charges`) is "Cargas e munição" in the master's Grupo tab, above the rest card.
`PartyCharges` (`core/play/party-charges-state.ts`) reads the sheet of each character of the party through `ChargesClient.sheetOf` and derives the rows and the list of what a dawn touches.
"Corrigir" is `SetCharges` (with the "Destruída" switch) or `SetAmmunition`, and the sheets are read again after it.
"Amanhecer" in the panel head is `ChargesService.Dawn`, and its answer lists what each item regained.
The rest card asks "Amanhecer" in a long rest, on by default, and always sends `TakeRestRequest.restore_dawn` explicitly: the row's answer, true when no item waits, false for a short rest.
`recharge-text.ts` words the recharge line ("volta 1d6 + 1") and spaces the dice with the minus sign.
The panel reads the sheets when the party changes and after its own writes, so a charge a player spends shows after the next read.
"Corrigir" and the long rest's question read the sheets again before they show a number, so "eram N" and the dawn's list are fresh.
When that read fails the correction does not open, and the "Amanhecer" row stays on and says the items could not be read.
