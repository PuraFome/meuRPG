package rules

import "errors"

// ErrSpend is the base of the errors of a spend outside what an action allows.
var ErrSpend = errors.New("spend")

// ActionResult is what one use of a grant_action gives, at its base cost. Spending
// more points (Action.CostMax) adds the per-point parts. Heal and TempHP follow the
// rules of the SRD spells (combat.ApplyHeal, combat.ApplyTempHP): the heal never
// passes the maximum, a dead creature is not healed and temporary hit points do not
// stack.
type ActionResult struct {
	// Heal is the dice healed, the ability modifier already in Bonus; nil when the
	// action heals nothing.
	Heal *DiceFormula
	// TempHP is the temporary hit points given; 0 when there are none.
	TempHP int
	// HealPerPoint is what each point spent above the base cost adds to Heal (the
	// same sides), TempHPPerPoint what it adds to TempHP.
	HealPerPoint   DiceFormula
	TempHPPerPoint int
}

// Outcome is the result of one use that spent some points.
type Outcome struct {
	// Heal is the dice to roll, nil when there is no heal.
	Heal *DiceFormula
	// TempHP is the temporary hit points to give, 0 for none.
	TempHP int
}

// UnitCost is how many uses one use of the action spends at least: 0 for an
// action with no resource, otherwise the cost (1 when it has none).
func (a Action) UnitCost() int { return -1 }

// ChoosesSpend says the player picks how many points to spend (CostMax above the
// cost).
func (a Action) ChoosesSpend() bool { return false }

// SpendRange is the least and the most the player may spend with left uses of the
// resource left; ok is false when fewer than the cost are left (the action is off).
func (a Action) SpendRange(left int) (lo, hi int, ok bool) { return -1, -1, false }

// Spend validates the number the player picked (0 takes the least) against the
// range and returns what to spend; ErrSpend otherwise.
func (a Action) Spend(chosen, left int) (int, error) { return -1, nil }

// Outcome is what spending spent points gives.
func (a Action) Outcome(spent int) Outcome { return Outcome{TempHP: -1} }
