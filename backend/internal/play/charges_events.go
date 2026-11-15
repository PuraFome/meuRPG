package play

// The session events of the charges of magic items and of the ammunition (session_event_kinds). Each payload holds ids,
// numbers and the catalog's names, never free text.
const (
	// eventItemChargeSpent: charges were spent, or the d20 of the last charge ended the wait with nothing else.
	eventItemChargeSpent = "item_charge_spent"
	// eventItemChargesRestored: the master corrected an item's charges, or a 20 on the last charge gave charges back.
	eventItemChargesRestored = "item_charges_restored"
	// eventItemDestroyed: an item was lost, with its last charge, on a 1 or by the master's correction.
	eventItemDestroyed = "item_destroyed"
	// eventAmmunitionSpent: a ranged attack fired a piece of ammunition (written in the attack's transaction).
	eventAmmunitionSpent = "ammunition_spent"
	// eventAmmunitionRecovered: the pieces of a combat were searched for and half taken back.
	eventAmmunitionRecovered = "ammunition_recovered"
)
