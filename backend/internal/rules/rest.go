package rules

import (
	"errors"
	"fmt"
	"slices"
)

// Resting (SRD 5.1, "Resting"). The app records the rests the master calls; it
// does not count hours. A short rest is at least 1 hour and a long rest at least
// 8 hours, and the SRD allows one long rest in 24 hours: the master keeps that
// clock at the table, and the app only warns when the session already had one.

// The rest kinds are RestShort and RestLong (casting.go).

// RechargesOnRest says whether a resource that recharges as recharge comes back
// when the character finishes a rest of the kind. A resource that comes back
// "after a short or long rest" (Ki, Channel Divinity, Second Wind, Action Surge,
// Wild Shape, Bardic Inspiration from bard level 5) is RechargeShortRest here: a
// long rest restores it too. A resource that comes back "after a long rest"
// (Rage, Sorcery Points, Lay on Hands) waits for the long one. A dawn or "none"
// recharge belongs to a magic item or to the master's own ruling, and no rest
// restores it by itself.
func RechargesOnRest(recharge, kind string) bool {
	switch recharge {
	case RechargeShortRest:
		return kind == RestShort || kind == RestLong
	case RechargeLongRest:
		return kind == RestLong
	}
	return false
}

// PactSlotsReturnOn says whether a warlock's pact magic slots come back with the
// rest (SRD 5.1, Warlock, Pact Magic: "You regain all expended spell slots when
// you finish a short or long rest"), and SpellSlotsReturnOn whether the other
// spell slots do ("Spell Slots": a long rest restores them).
func PactSlotsReturnOn(kind string) bool { return kind == RestShort || kind == RestLong }

// SpellSlotsReturnOn is PactSlotsReturnOn for the slots every other caster has.
func SpellSlotsReturnOn(kind string) bool { return kind == RestLong }

// TemporaryHitPointsEndOn says whether the rest takes the temporary hit points
// away: they last "until they're depleted or you finish a long rest" (SRD 5.1,
// "Temporary Hit Points"). Aid, which the app gives a character as temporary hit
// points, ends the same way.
func TemporaryHitPointsEndOn(kind string) bool { return kind == RestLong }

// LongRestNeedsHitPoints is the SRD's condition for a long rest to do anything: "a
// character must have at least 1 hit point at the start of the rest to gain its
// benefits".
func LongRestNeedsHitPoints(hitPoints int) bool { return hitPoints >= 1 }

// ErrHitDice is the base of the errors of the hit dice functions: a die size the
// character does not have, or more dice than it has left.
var ErrHitDice = errors.New("hit dice")

// HitDiceTotal is how many hit dice the character has in all: its level.
func HitDiceTotal(have []HitDice) int {
	total := 0
	for _, hd := range have {
		total += hd.Count
	}
	return total
}

// HitDiceUsed is how many hit dice are spent, by die size (d6 -> 2). Only the
// sizes a character has appear, and none with a zero count.
type HitDiceUsed map[int]int

// Total is the number of dice spent.
func (u HitDiceUsed) Total() int {
	total := 0
	for _, n := range u {
		total += n
	}
	return total
}

// Clean returns the spent dice cut to what the character has: a size it does not
// have is dropped (a class it left) and a count above the dice of the size is cut
// (the sheet lost a level). The result is a new map without zero counts.
func (u HitDiceUsed) Clean(have []HitDice) HitDiceUsed {
	out := HitDiceUsed{}
	for _, hd := range have {
		if n := min(max(u[hd.Die], 0), hd.Count); n > 0 {
			out[hd.Die] = n
		}
	}
	return out
}

// SplitHitDiceUsed turns a count of spent dice that was kept without the sizes
// (the way a character's hit dice were stored before they were kept by size) into
// spent dice by size: the largest dice are the ones spent first, so a character is
// never left with more healing than the old number allowed. A count above the dice
// the character has is cut. For a character with one die size it is exact.
func SplitHitDiceUsed(have []HitDice, total int) HitDiceUsed {
	out := HitDiceUsed{}
	sorted := slices.Clone(have)
	slices.SortFunc(sorted, func(a, b HitDice) int { return b.Die - a.Die })
	for _, hd := range sorted {
		take := min(total, hd.Count)
		if take > 0 {
			out[hd.Die] += take
			total -= take
		}
	}
	return out
}

// HitDiceLeft is the dice the character can still spend, by size.
func HitDiceLeft(have []HitDice, used HitDiceUsed) HitDiceUsed {
	out := HitDiceUsed{}
	for _, hd := range have {
		if n := hd.Count - used[hd.Die]; n > 0 {
			out[hd.Die] = n
		}
	}
	return out
}

// SpendHitDice marks the dice as spent: spend is how many of each size. It
// refuses a size the character does not have, a count that is not positive and
// more dice than are left of a size. The result is a new map.
func SpendHitDice(have []HitDice, used, spend HitDiceUsed) (HitDiceUsed, error) {
	left := HitDiceLeft(have, used)
	out := copyUsed(used)
	for die, n := range spend {
		switch {
		case !slices.ContainsFunc(have, func(hd HitDice) bool { return hd.Die == die }):
			return nil, fmt.Errorf("%w: the character has no d%d", ErrHitDice, die)
		case n < 1:
			return nil, fmt.Errorf("%w: the d%d count must be at least 1", ErrHitDice, die)
		case n > left[die]:
			return nil, fmt.Errorf("%w: only %d d%d left", ErrHitDice, left[die], die)
		}
		out[die] += n
	}
	return out, nil
}

// HitDieHeal is what one spent hit die heals: the die rolled plus the
// Constitution modifier (SRD 5.1, "Short Rest"). The SRD does not say what a
// negative total does; the app never takes hit points away, so it heals 0.
func HitDieHeal(face, constitutionModifier int) int {
	return max(face+constitutionModifier, 0)
}

// LongRestHitDiceLimit is how many spent hit dice a long rest gives back: half of
// the character's total number of them, and at least one (SRD 5.1, "Long Rest":
// "up to a number of dice equal to half of the character's total number of them
// (minimum of one die)").
func LongRestHitDiceLimit(have []HitDice) int {
	return max(HitDiceTotal(have)/2, 1)
}

// HitDiceReturned works out the dice a long rest gives back and what is spent
// after it. choice is how many of each size the player wants back; nil takes the
// largest dice first, the most a die can heal. A choice may give back fewer than
// the limit, never more, and never more of a size than is spent.
func HitDiceReturned(have []HitDice, used, choice HitDiceUsed) (back, after HitDiceUsed, err error) {
	limit := LongRestHitDiceLimit(have)
	back = HitDiceUsed{}
	if choice == nil {
		sorted := slices.Clone(have)
		slices.SortFunc(sorted, func(a, b HitDice) int { return b.Die - a.Die })
		for _, hd := range sorted {
			if take := min(used[hd.Die], limit); take > 0 {
				back[hd.Die] = take
				limit -= take
			}
		}
	} else {
		if choice.Total() > limit {
			return nil, nil, fmt.Errorf("%w: a long rest gives back %d at most", ErrHitDice, limit)
		}
		for die, n := range choice {
			switch {
			case !slices.ContainsFunc(have, func(hd HitDice) bool { return hd.Die == die }):
				return nil, nil, fmt.Errorf("%w: the character has no d%d", ErrHitDice, die)
			case n < 1:
				return nil, nil, fmt.Errorf("%w: the d%d count must be at least 1", ErrHitDice, die)
			case n > used[die]:
				return nil, nil, fmt.Errorf("%w: only %d d%d spent", ErrHitDice, used[die], die)
			}
			back[die] = n
		}
	}
	after = copyUsed(used)
	for die, n := range back {
		after[die] -= n
		if after[die] <= 0 {
			delete(after, die)
		}
	}
	return back, after, nil
}

// copyUsed copies a spent-dice map without its zero counts.
func copyUsed(u HitDiceUsed) HitDiceUsed {
	out := HitDiceUsed{}
	for die, n := range u {
		if n > 0 {
			out[die] = n
		}
	}
	return out
}
