package rules

import (
	"errors"
	"slices"
)

// The ways of making a new sheet's ability scores (MR-025, RN-24). The standard
// array, the 27-point buy and "4d6, drop the lowest" come from the SRD 5.2.1
// (CC BY 4.0, p. 20); the SRD 5.1 has no way of making scores. They are the
// same numbers as in 2014. The scores checked here are the base scores, before
// the race's bonus. Everything is pure: the dice are rolled elsewhere.

// Limits of the typed method and the point buy.
const (
	// TypedMinScore and TypedMaxScore bound a typed base score (before the
	// race bonus): 3 to 18, the range of three six-sided dice.
	TypedMinScore = 3
	TypedMaxScore = 18
	// PointBuyBudget is what the point buy spends.
	PointBuyBudget = 27
	// PointBuyMinScore and PointBuyMaxScore bound a bought score.
	PointBuyMinScore = 8
	PointBuyMaxScore = 15
	// AbilityRollSets is how many sets of dice "4d6, drop the lowest" makes,
	// one for each ability; AbilityRollDice is the dice in a set.
	AbilityRollSets = 6
	AbilityRollDice = 4
)

// StandardArray is the standard array: each of the six values goes to one ability.
func StandardArray() []int { return []int{15, 14, 13, 12, 10, 8} }

// pointBuyCost is what each bought score costs, from 8 to 15.
var pointBuyCost = [...]int{0, 1, 2, 3, 4, 5, 7, 9}

// PointBuyCost is the cost, in points, of a bought score, and false for a
// score outside 8 to 15.
func PointBuyCost(score int) (int, bool) {
	if score < PointBuyMinScore || score > PointBuyMaxScore {
		return 0, false
	}
	return pointBuyCost[score-PointBuyMinScore], true
}

// Why a set of scores does not follow a method.
var (
	// ErrNotStandardArray: the scores are not 15, 14, 13, 12, 10 and 8, each once.
	ErrNotStandardArray = errors.New("the scores are not the standard array")
	// ErrBadPointBuy: a score outside 8 to 15, or more than 27 points spent.
	ErrBadPointBuy = errors.New("the scores are not a valid 27-point buy")
	// ErrNotTheRolls: the scores are not the six results of the stored rolls, each once.
	ErrNotTheRolls = errors.New("the scores are not the stored rolls")
	// ErrTypedRange: a score outside 3 to 18.
	ErrTypedRange = errors.New("a typed score is outside 3 to 18")
)

// CheckStandardArray says whether scores are a permutation of the standard
// array.
func CheckStandardArray(scores []int) error {
	if !slices.Equal(slices.Sorted(slices.Values(scores)), slices.Sorted(slices.Values(StandardArray()))) {
		return ErrNotStandardArray
	}
	return nil
}

// PointBuySpent is the points the scores cost, and false when one of them is
// outside 8 to 15.
func PointBuySpent(scores []int) (int, bool) {
	spent := 0
	for _, s := range scores {
		c, ok := PointBuyCost(s)
		if !ok {
			return 0, false
		}
		spent += c
	}
	return spent, true
}

// CheckPointBuy says whether scores are a purchase of at most 27 points, each
// score 8 to 15. Spending fewer than 27 is allowed ("Restam 2 pontos").
func CheckPointBuy(scores []int) error {
	if spent, ok := PointBuySpent(scores); !ok || spent > PointBuyBudget {
		return ErrBadPointBuy
	}
	return nil
}

// CheckTyped says whether every score is 3 to 18.
func CheckTyped(scores []int) error {
	for _, s := range scores {
		if s < TypedMinScore || s > TypedMaxScore {
			return ErrTypedRange
		}
	}
	return nil
}

// AbilityRollTotal is the result of one set of "4d6, drop the lowest": the
// dice added without the lowest one. dice has four faces.
func AbilityRollTotal(dice []int) int {
	total := 0
	for _, d := range dice {
		total += d
	}
	return total - slices.Min(dice)
}

// CheckAbilityRollDice says whether sets are six sets of four dice, each 1 to 6.
func CheckAbilityRollDice(sets [][]int) error {
	if len(sets) != AbilityRollSets {
		return errors.New("an ability roll has six sets of dice")
	}
	for _, set := range sets {
		if len(set) != AbilityRollDice {
			return errors.New("each set of an ability roll has four dice")
		}
		for _, d := range set {
			if d < 1 || d > 6 {
				return errors.New("each die of an ability roll is 1 to 6")
			}
		}
	}
	return nil
}

// AbilityRollTotals are the results of the six sets, in order.
func AbilityRollTotals(sets [][]int) []int {
	out := make([]int, len(sets))
	for i, set := range sets {
		out[i] = AbilityRollTotal(set)
	}
	return out
}

// CheckRolled says whether scores are the six results of the sets of dice
// (already checked by CheckAbilityRollDice), each used once, in any order.
func CheckRolled(scores []int, sets [][]int) error {
	if !slices.Equal(slices.Sorted(slices.Values(scores)), slices.Sorted(slices.Values(AbilityRollTotals(sets)))) {
		return ErrNotTheRolls
	}
	return nil
}

// FreeAbilityPoints is how many positive manual ability points (the
// Build.ExtraAbilityBonuses that are above 0) a build may place without the
// table's ways of making scores being worked around (RN-24): 2 for each Ability
// Score Improvement the classes reached, which the guided level-up and the editor
// also write there. The points a race lets the player choose (the half-elf's +1 to
// two abilities) are not manual: they are picks of the choice engine
// (choicegroups.go) and add to the race's bonus. A table race's own "+2 and +1 to
// your choice" (raceChoice) has no choice engine behind it: the player places it
// in the manual bonuses, so its points are free too.
func (c *Content) FreeAbilityPoints(b Build) int {
	n := 0
	for _, v := range c.c.raceChoice[b.Race] {
		n += v
	}
	for _, cl := range b.Classes {
		rows := c.c.classLevels[cl.Class]
		for level := 1; level <= cl.Level && level <= len(rows); level++ {
			if isASILevel(rows[level-1]) {
				n += 2
			}
		}
	}
	return n
}

// PositiveManualBonus is the sum of the positive manual ability bonuses of the
// build. Negative ones are free.
func PositiveManualBonus(b Build) int {
	n := 0
	for _, v := range b.ExtraAbilityBonuses {
		if v > 0 {
			n += v
		}
	}
	return n
}
