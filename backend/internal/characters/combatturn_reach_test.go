package characters

import (
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// The melee reach a weapon rule gives an attack line reaches the combat (link.Attack.ReachFt), so a
// thrown weapon is a melee attack up to that reach. No database is needed.
func TestCombatAttackCarriesTheMeleeReachOfTheLine(t *testing.T) {
	t.Parallel()
	spear := rules.Attack{Key: "weapon:spear", Kind: "weapon", Melee: true, RangeFt: 20, LongRangeFt: 60, ReachFt: 10}
	if got := combatAttack(spear).ReachFt; got != 10 {
		t.Errorf("combatAttack ReachFt = %d, want 10", got)
	}
	spear.ReachFt = 0
	if got := combatAttack(spear).ReachFt; got != 0 {
		t.Errorf("without a rule, combatAttack ReachFt = %d, want 0", got)
	}
}

// The attack rolls the dice of its grip: both hands roll the versatile dice, and count as two-handed for Great Weapon
// Fighting; one hand rolls the one-handed dice.
func TestCombatAttackRollsTheDiceOfTheGrip(t *testing.T) {
	t.Parallel()
	axe := rules.Attack{
		Key: "equipment:woodcutter-axe", Kind: "weapon", Melee: true,
		DamageDice: rules.DiceFormula{Count: 1, Sides: 6, Bonus: 3}, VersatileDice: rules.DiceFormula{Count: 1, Sides: 8, Bonus: 3},
	}
	for _, tc := range []struct {
		grip      rules.Grip
		sides     int
		twoHanded bool
	}{
		{rules.GripNone, 6, false},
		{rules.GripOneHandShield, 6, false},
		{rules.GripOneHandWeapon, 6, false},
		{rules.GripTwoHands, 8, true},
	} {
		axe.Grip = tc.grip
		got := combatAttack(axe)
		if got.DiceSides != tc.sides || got.DiceCount != 1 || got.DiceBonus != 3 || got.TwoHanded != tc.twoHanded {
			t.Errorf("grip %q: dice %dd%d%+d two-handed=%v, want 1d%d+3 two-handed=%v", tc.grip, got.DiceCount, got.DiceSides, got.DiceBonus, got.TwoHanded, tc.sides, tc.twoHanded)
		}
	}
}
