package encounter

import (
	"errors"
	"fmt"
	"slices"
)

// Band is how hard an encounter is for a party, in the words of the 2024 guide.
type Band int

const (
	// BandLow is an encounter that fits the party's low budget, and also one that
	// costs less than that: under "Baixa" it still reads "Baixa".
	BandLow Band = iota + 1
	// BandModerate is the smallest band that holds the total, when it is above the
	// low budget and within the moderate one.
	BandModerate
	// BandHigh is above the moderate budget and within the high one.
	BandHigh
	// BandAbove is above the high budget. The 2024 guide has no band for it (it
	// only has three), so the encounter is allowed and reported as above high,
	// with how far above.
	BandAbove
)

// Budget is the XP a party can spend in each band. The three numbers grow:
// Low < Moderate < High.
type Budget struct {
	Low, Moderate, High int
}

// Add returns the sum of two budgets.
func (b Budget) Add(o Budget) Budget {
	return Budget{Low: b.Low + o.Low, Moderate: b.Moderate + o.Moderate, High: b.High + o.High}
}

// For returns the XP of a band: 0 for BandAbove, which has no budget of its own.
func (b Budget) For(band Band) int {
	switch band {
	case BandLow:
		return b.Low
	case BandModerate:
		return b.Moderate
	case BandHigh:
		return b.High
	}
	return 0
}

// ErrLevel is the error of a level outside the table (1 to its length).
var ErrLevel = errors.New("encounter: level outside the budget table")

// PartyBudget is the sum of each member's budget, band by band. levels are the
// members' levels, 1 to len(table); a party of nobody has a zero budget.
func PartyBudget(table []Budget, levels []int) (Budget, error) {
	var sum Budget
	for _, l := range levels {
		if l < 1 || l > len(table) {
			return Budget{}, fmt.Errorf("%w: %d", ErrLevel, l)
		}
		sum = sum.Add(table[l-1])
	}
	return sum, nil
}

// Classify says which band the total XP is: the smallest band whose budget holds
// it. Above the high budget it is BandAbove, and over is how many XP it passes the
// high budget by (0 in every other band).
func Classify(b Budget, total int) (band Band, over int) {
	switch {
	case total <= b.Low:
		return BandLow, 0
	case total <= b.Moderate:
		return BandModerate, 0
	case total <= b.High:
		return BandHigh, 0
	}
	return BandAbove, total - b.High
}

// MaxCR is the highest challenge rating a creature may have in an encounter for
// the party: its lowest level plus 3, in eighths of a rating (so 1/8 is 1 and 2 is
// 16), never above maxEighths (the SRD's rating 30 is 240). Every member counts, a
// NPC the master added with the level he gave too. A party of nobody gets 0.
func MaxCR(levels []int, maxEighths int) int {
	if len(levels) == 0 {
		return 0
	}
	return min((slices.Min(levels)+3)*8, maxEighths)
}

// LowestLevel is the smallest of the levels, 0 for none.
func LowestLevel(levels []int) int {
	if len(levels) == 0 {
		return 0
	}
	return slices.Min(levels)
}

// Creature is what the arithmetic needs to know about an SRD creature.
type Creature struct {
	// Key is the creature's key ("monster:ogre").
	Key string
	// CR is the challenge rating in eighths of a rating.
	CR int
	// XP is what the creature gives when defeated, by its rating. The 2024 guide
	// uses no multipliers: an encounter costs the sum of these.
	XP int
	// Type is the SRD's type ("humanoid").
	Type string
}

// Entry is a creature and how many of it fight.
type Entry struct {
	Key   string
	Count int
}

// Total is the XP of an encounter: each creature's XP times its count. The
// creatures are looked up by key; ok is false when an entry names one that is not in
// the pool. (It is a sum, with no multiplier for the number of creatures: that
// belonged to the 2014 guide.)
func Total(pool map[string]Creature, entries []Entry) (xp int, ok bool) {
	for _, e := range entries {
		c, found := pool[e.Key]
		if !found || e.Count < 0 {
			return 0, false
		}
		xp += c.XP * e.Count
	}
	return xp, true
}
