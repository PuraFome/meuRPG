package combat

import (
	"errors"
	"fmt"

	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// ErrBadRoll is returned for a die face outside the die, or for the wrong
// number of dice.
var ErrBadRoll = errors.New("combat: the dice do not match the formula")

// AttackResult is the outcome of an attack roll.
type AttackResult struct {
	// Total is the d20 plus the attack bonus.
	Total int
	Hit   bool
	// Critical is a natural 20 (or lower, with Improved Critical). A natural 1 never hits (Fumble).
	Critical bool
	Fumble   bool
}

// naturalTwenty is the d20 face that is always a critical hit.
const naturalTwenty = 20

// ResolveAttack compares an attack roll with the target's armor class. A
// natural 20 always hits and is a critical hit; a natural 1 always misses;
// anything else hits when the total reaches the armor class. d20Face is the
// die itself (1 to 20), without the bonus; the caller checks the range.
func ResolveAttack(attackBonus, targetAC, d20Face int) AttackResult {
	return ResolveAttackFrom(attackBonus, targetAC, d20Face, naturalTwenty)
}

// ResolveAttackFrom is ResolveAttack for an attacker whose critical hits start
// below 20 (Improved Critical: 19, Superior Critical: 18): a natural roll of
// criticalFrom or more always hits and is a critical hit. A value above 20 or
// below 2 counts as 20.
func ResolveAttackFrom(attackBonus, targetAC, d20Face, criticalFrom int) AttackResult {
	if criticalFrom < 2 || criticalFrom > naturalTwenty {
		criticalFrom = naturalTwenty
	}
	r := AttackResult{Total: d20Face + attackBonus}
	switch {
	case d20Face >= criticalFrom:
		r.Hit, r.Critical = true, true
	case d20Face == 1:
		r.Fumble = true
	default:
		r.Hit = r.Total >= targetAC
	}
	return r
}

// DiceToRoll is how many dice of the formula are rolled: a critical hit
// rolls the damage dice twice (the bonus is never doubled).
func DiceToRoll(f rules.DiceFormula, critical bool) int {
	if critical {
		return f.Count * 2
	}
	return f.Count
}

// CriticalRule is what a critical hit does to the damage dice (RN-24, MR-025):
// the table's choice. The zero value is the SRD's.
type CriticalRule int

const (
	// CriticalDoubledDice rolls every damage die twice (the SRD's).
	CriticalDoubledDice CriticalRule = iota
	// CriticalMaxPlusRoll counts the dice as their maximum and adds one normal
	// roll of them.
	CriticalMaxPlusRoll
)

// CriticalDice is how a damage is made: count is how many dice are rolled, and
// fixed what is added to them without rolling (the maximum of the dice, under
// CriticalMaxPlusRoll). The bonus is never doubled and never part of fixed. A
// hit that is not a critical rolls the formula's dice, whatever the rule.
func CriticalDice(f rules.DiceFormula, critical bool, rule CriticalRule) (count, fixed int) {
	switch {
	case !critical:
		return f.Count, 0
	case rule == CriticalMaxPlusRoll:
		return f.Count, f.Count * f.Sides
	}
	return f.Count * 2, 0
}

// DiceRange is the smallest and largest total of the dice alone (without
// the bonus), for checking a total typed from physical dice.
func DiceRange(f rules.DiceFormula, critical bool) (lowest, highest int) {
	n := DiceToRoll(f, critical)
	return n, n * f.Sides
}

// DamageTotal adds up the rolled faces and the formula's bonus. A critical
// hit needs twice the dice and does not double the bonus. Damage never goes
// below 0. ErrBadRoll says the faces do not fit the formula.
func DamageTotal(f rules.DiceFormula, faces []int, critical bool) (int, error) {
	if len(faces) != DiceToRoll(f, critical) {
		return 0, fmt.Errorf("%w: want %d dice, got %d", ErrBadRoll, DiceToRoll(f, critical), len(faces))
	}
	total := f.Bonus
	for _, face := range faces {
		if face < 1 || face > f.Sides {
			return 0, fmt.Errorf("%w: %d on a d%d", ErrBadRoll, face, f.Sides)
		}
		total += face
	}
	return max(total, 0), nil
}

// ConcentrationDC is the Constitution save DC to keep concentrating after
// taking damage: 10 or half the damage, whichever is higher (rounded down).
// RN-22 only reminds the table; this is the number in the reminder.
func ConcentrationDC(damage int) int {
	return max(10, damage/2)
}

// ShieldACBonus is what the Shield spell adds to the armor class until the start
// of the caster's next turn.
const ShieldACBonus = 5

// MissileDarts is how many darts Magic Missile makes when cast with a slot of
// the level: three, and one more for each level above the 1st.
func MissileDarts(slotLevel int) int {
	return 3 + max(slotLevel-1, 0)
}

// SaveSucceeded says whether a saving throw's total reaches the spell's DC.
func SaveSucceeded(total, dc int) bool {
	return total >= dc
}

// HalfDamage is half of a damage, rounded down: what a creature that saved
// takes from a spell that says "half as much damage on a successful save".
func HalfDamage(damage int) int {
	return max(damage, 0) / 2
}
