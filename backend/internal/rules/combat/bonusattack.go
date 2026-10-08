package combat

// Bonus action attacks (SRD 5.1): the off-hand attack of Two-Weapon Fighting,
// the Martial Arts unarmed strike, and the unarmed strikes of Flurry of Blows.
// All of them follow the Attack action.

// BonusKind is which rule lets an attack be made with the bonus action.
type BonusKind int

const (
	// BonusNone says the attack cannot be made as a bonus action now.
	BonusNone BonusKind = iota
	// BonusTwoWeapon is a light melee weapon after the Attack action with a
	// light melee weapon. The damage leaves out the ability modifier, unless
	// it is negative or the character has the Two-Weapon Fighting style.
	BonusTwoWeapon
	// BonusMartialArts is one unarmed strike after the Attack action with an
	// unarmed strike or a monk weapon.
	BonusMartialArts
	// BonusFlurry is one of the two unarmed strikes of Flurry of Blows, whose
	// bonus action is already spent.
	BonusFlurry
)

// AttackTraits is what the bonus action rules read of an attack.
type AttackTraits struct {
	// Spell says it is a cantrip: no bonus action attack goes with it.
	Spell bool
	// Melee and Light say it is a light melee weapon.
	Melee, Light bool
	// Unarmed says it is the unarmed strike.
	Unarmed bool
	// MartialArts says the Martial Arts strike goes with it (the unarmed strike
	// or a monk weapon, while Martial Arts holds).
	MartialArts bool
}

// BonusAttackTurn is the state of the turn a bonus action attack reads.
type BonusAttackTurn struct {
	// AttackAction says the Attack action was taken with a weapon or an
	// unarmed strike (the action is spent and made at least one attack).
	AttackAction bool
	// BonusUsed says the bonus action is spent.
	BonusUsed bool
	// FlurryLeft is how many unarmed strikes of Flurry of Blows are left.
	FlurryLeft int
	// Last is the last attack the Attack action made.
	Last AttackTraits
}

// BonusAttack says whether next may be made now as a bonus action attack, and
// by which rule. Flurry of Blows comes first (its bonus action is already
// spent), then the Martial Arts strike, then Two-Weapon Fighting.
func BonusAttack(turn BonusAttackTurn, next AttackTraits) BonusKind {
	if !turn.AttackAction || turn.Last.Spell || next.Spell {
		return BonusNone
	}
	if turn.FlurryLeft > 0 && next.Unarmed {
		return BonusFlurry
	}
	if turn.BonusUsed {
		return BonusNone
	}
	switch {
	case turn.Last.MartialArts && next.Unarmed:
		return BonusMartialArts
	case turn.Last.Melee && turn.Last.Light && next.Melee && next.Light:
		return BonusTwoWeapon
	}
	return BonusNone
}

// OffHandBonus is the damage bonus of a Two-Weapon Fighting bonus action
// attack: the attack's bonus without its ability modifier, unless the modifier
// is negative or the character has the fighting style.
func OffHandBonus(damageBonus, abilityMod int, style bool) int {
	if style || abilityMod <= 0 {
		return damageBonus
	}
	return damageBonus - abilityMod
}
