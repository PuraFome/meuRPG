package play

import "github.com/PuraFome/meuRPG/backend/internal/play/playdb"

// The arithmetic of Aid's bonus to the hit point maximum, shared by the combat and by a
// cast outside one, so one place owns "raise the maximum" and "take it back".

// aidRaise is what a new Aid does to a target that already has bonusBefore from an
// earlier one: casts of the same spell do not combine, the stronger bonus applies, and
// the maximum (and the current hit points) rise by the difference.
func aidRaise(bonusBefore, amount int32) (stronger, rose int32) {
	return 0, 0
}

// aidEnd is the hit points and the maximum of a target whose bonus ends: the maximum
// falls back (never below 1) and the current hit points lose only what is above it,
// never the last one of a target that still has some.
func aidEnd(hp, maxBefore, bonus int32) (hpAfter, maxAfter int32) {
	return 0, 0
}

// aidLeft is the bonus a target keeps when one Aid ends: the strongest bonus of the
// other Aids still on it (0 for none), never more than the bonus it has.
func aidLeft(bonus int32, others []int32) int32 {
	return bonus
}

// aidTargetsOf lists the targets a cast raised the maximum of.
func aidTargetsOf(r playdb.SpellCast) []castTarget {
	return nil
}
