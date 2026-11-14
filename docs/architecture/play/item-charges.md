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
- `SetCharges` is the master's correction and has no player.
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
Two calls for the last charge give one success: the second reads the first one's answer or `ITEM_DESTROYED`.

## The master's correction

`SetCharges` sets the charges left (0 to the maximum, which is also how a rolled start gets its count) and whether the item was lost.
It ends a wait for the d20, writes `item_charges_restored` (or `item_destroyed` when the item is lost) and no line of the combat log.

## Ammunition

`SetAmmunition` sets the pieces of one kind, 0 to 9999.
`RecoverAmmunition` takes back half, rounded down, of what each kind fired in the character's latest combat, once for each combat.
It refuses with `ChargesBlocked` `COMBAT_RUNNING` while that combat is not ended, and `ALREADY_RECOVERED` after it ran.
A combat that fired nothing is skipped, and the recovery goes to the newest earlier ended combat that fired some, among the latest ten.
A character that fired nothing in any of them is `invalid_argument`.
It writes the session event `ammunition_recovered`.

## Ammunition in combat

A ranged attack with a weapon that fires ammunition takes one piece in the attack's transaction ([Attacks and damage](attacks-and-damage.md#the-two-step-attack)).
The keeper's `SpendAmmunition` starts the count of a combat over (nothing fired, not recovered) when the combat is not the one it last counted.
With none left the attack is refused with `EncounterBlocked` `AMMUNITION_EMPTY`.
Each shot writes the session event `ammunition_spent` with the combat's round, which reads as the line `AMMUNITION_SPENT`.

## Events

The events are `item_charge_spent`, `item_charges_restored`, `item_destroyed`, `ammunition_spent` and `ammunition_recovered`.
In a combat that has the character, `SpendCharges` writes `COMBAT_LOG_KIND_ITEM_CHARGE_SPENT` and `RollLastCharge` writes `COMBAT_LOG_KIND_ITEM_LAST_CHARGE_ROLLED`.
An event of `ammunition_spent` reads as `COMBAT_LOG_KIND_AMMUNITION_SPENT`.
`SetCharges`, `RecoverAmmunition` and any call outside a combat write the session event only, and no line.

## Privacy

Charges, ammunition and these lines are the master's and the owning player's alone (RN-10).
No other player reads the line, the event (secret) or the hint that the stream carries.
The leak test is `internal/leaktest/itemuse_test.go`.

## Refusals

`ChargesBlocked` (`charges.proto`) carries `REGAIN_FACE_NEEDED` (with the die's size in `needed`), `COMBAT_RUNNING` and `ALREADY_RECOVERED`.
The web reads it in `core/play/charges-errors.ts`.
