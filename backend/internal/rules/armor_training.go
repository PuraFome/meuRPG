package rules

// ArmorUntrained says which of the worn pieces the character is not proficient
// with. The sheet's warning and the play rules read this one answer: wearing a
// piece without the training gives disadvantage on every ability check, saving
// throw and attack roll that uses Strength or Dexterity, and the character
// cannot cast spells (SRD 5.1, "Armor").
type ArmorUntrained struct {
	// Armor is set when the worn body armor is one the character is not trained
	// in (its category or the armor itself); false without armor.
	Armor bool
	// Shield is set when a shield in use (no two-handed weapon beside it) is carried without the training.
	Shield bool
}

// Any says whether some worn piece breaks the proficiency.
func (u ArmorUntrained) Any() bool { return u.Armor || u.Shield }

// armorUntrained works out which of the worn armor and shield the character has
// no proficiency for.
func (x *deriver) armorUntrained() ArmorUntrained {
	return ArmorUntrained{
		Armor:  x.armor != nil && !x.armorProficient(x.armorCategory, x.b.Armor),
		Shield: x.shieldWielded() && !x.armorProficient("shield", "equipment:shield"),
	}
}
