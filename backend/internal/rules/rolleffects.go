package rules

import (
	"slices"
	"strconv"

	"github.com/PuraFome/meuRPG/backend/internal/rules/formula"
)

// RollDamage is, besides the d20 rolls of combateffects.go, a roll a reroll covers (its
// `rolls`): a die of weapon or spell damage.
const RollDamage = "damage"

// rollApplies are the rolls a reroll covers: RollAppliesAttack, RollAppliesCheck and
// RollAppliesSave (the d20 rolls) and RollDamage (a die of weapon or spell damage).
var rollApplies = []string{RollAppliesAttack, RollAppliesCheck, RollAppliesSave, RollDamage}

// The bounds of the numbers of the roll effects.
const (
	// MinCriticalRange and MaxCriticalRange bound the lowest natural d20 that is a critical hit.
	MinCriticalRange = 15
	MaxCriticalRange = 20
	// MinRerollAtMost and MaxRerollAtMost bound a reroll's at_most: a die showing it or lower may be rolled again.
	MinRerollAtMost = 1
	MaxRerollAtMost = 20
)

// CriticalRangeTarget is the modifier target that widens the critical range: the lowest natural d20 that
// is a critical hit. Only the mode set is meaningful, and only the deriver reads it (Derived.CriticalRange).
const CriticalRangeTarget = "critical_range"

// RerollRule is a reroll effect the character has: a die of the covered rolls that shows AtMost or lower may
// be rolled again, and the new roll must be used (the halfling's Lucky trait). Each reroll spends Cost uses of Resource
// when it names one.
type RerollRule struct {
	// Applies are the rolls it covers (Roll*).
	Applies []string
	// AtMost is the highest face that may be rolled again.
	AtMost int
	// Resource is the resource each reroll spends, "" for a free one, and Cost how many uses.
	Resource string
	Cost     int
	// Source is the feature (or option) key that gives it.
	Source string
}

// rollsShape says what is wrong with the list of rolls of a roll effect: the field and why, both "" when
// right. It is the shared check of the engine's load and of the table's menu.
func rollsShape(e *Effect) (field, msg string) {
	if len(e.Tags) > 0 {
		return "tags", "a " + e.Type + " has no tags: the engine never applies a tagged effect, and these have no hint to show"
	}
	if len(e.Rolls) == 0 {
		return "rolls", "rolls are required: attack, check, save or damage"
	}
	for i, r := range e.Rolls {
		if !slices.Contains(rollApplies, r) {
			return "rolls", "rolls " + strconv.Quote(r) + " is not on the menu: attack, check, save or damage"
		}
		if slices.Contains(e.Rolls[:i], r) {
			return "rolls", "rolls lists " + strconv.Quote(r) + " twice"
		}
	}
	return "", ""
}

// rerollShape says what is wrong with a reroll effect's own fields (the field and why), "" when right. The
// resource's existence and the cost formula are checked by the callers.
func rerollShape(e *Effect) (field, msg string) {
	if f, m := rollsShape(e); f != "" {
		return f, m
	}
	switch {
	case e.AtMost < MinRerollAtMost || e.AtMost > MaxRerollAtMost:
		return "at_most", "at_most is a face from 1 to 20"
	case e.Value != "":
		return "value", "a reroll has no value"
	case e.Cost != "" && e.Resource == "":
		return "cost", "cost needs the resource it spends"
	}
	if n, err := strconv.Atoi(e.Cost); err == nil && n < 1 {
		return "cost", "cost must be at least 1"
	}
	return "", ""
}

// criticalRangeShape says what is wrong with a modifier of the critical range: only the mode set, and a
// literal value from 15 to 20 (a formula is clamped when it is worked out).
func criticalRangeShape(e *Effect) (field, msg string) {
	if e.Mode != "" && e.Mode != "set" {
		return "mode", "unknown mode " + strconv.Quote(e.Mode) + " for critical_range: only set"
	}
	if n, err := strconv.Atoi(e.Value); err == nil && (n < MinCriticalRange || n > MaxCriticalRange) {
		return "value", "critical_range is a natural d20 from 15 to 20"
	}
	return "", ""
}

// reroll checks a reroll effect and compiles its cost.
func (x effectCompiler) reroll() error {
	if field, msg := rerollShape(x.e); field != "" {
		return x.fail("%s", msg)
	}
	var err error
	x.e.cost, err = x.compile(x.e.Cost, formula.Int)
	return err
}

// checkReroll checks a reroll of the table's content at its own fields.
func (b *overlayBuilder) checkReroll(site effectSite, e *Effect) error {
	if e.Resource != "" && !b.resourceDefined(site.owner, e.Resource) {
		return site.fail(".resource", ReasonReference, "the reroll spends %q, which no resource effect of this entry or its class defines", e.Resource)
	}
	if field, msg := rerollShape(e); field != "" {
		return site.fail("."+field, ReasonValue, "%s", msg)
	}
	return nil
}

// checkModifier checks the part of a modifier the engine's load does not tell apart: the critical range.
func (b *overlayBuilder) checkModifier(site effectSite, e *Effect) error {
	if e.Target != CriticalRangeTarget {
		return nil
	}
	if field, msg := criticalRangeShape(e); field != "" {
		return site.fail("."+field, ReasonValue, "%s", msg)
	}
	return nil
}

// rollEffects fills Derived.Rerolls from the effects the character has, in the order of the effects.
func (x *deriver) rollEffects() {
	for _, a := range x.active {
		e := a.effect
		switch {
		case len(e.Tags) > 0 || !x.applies(a):
		case e.Type == "reroll":
			if cost, ok := x.rerollCost(a); ok {
				x.d.Rerolls = append(x.d.Rerolls, RerollRule{
					Applies: slices.Clone(e.Rolls), AtMost: e.AtMost, Resource: e.Resource, Cost: cost, Source: a.owner,
				})
			}
		}
	}
}

// rerollCost is the uses a reroll spends (1 by default), and false when the reroll is dropped: the resource
// it spends does not exist at this level, the cost formula failed or gave less than 1, or the cost is more than the resource has.
func (x *deriver) rerollCost(a activeEffect) (int, bool) {
	e := a.effect
	if e.Resource == "" {
		return 1, true
	}
	i := slicesIndexResource(x.d.Resources, e.Resource)
	if i < 0 {
		return 0, false
	}
	n := 1
	if e.cost != nil {
		var ok bool
		if n, ok = x.evalInt(e.cost, a); !ok || n < 1 {
			return 0, false
		}
	}
	return n, n <= x.d.Resources[i].Max // a cost above the uses the resource has could never be paid
}

// criticalRangeFrom lowers the critical range by a modifier of the critical_range target: the lowest of the
// range so far and the effect's value, clamped to what the menu allows. A tagged modifier is a hint only.
func (x *deriver) criticalRangeFrom(a activeEffect) {
	e := a.effect
	if e.Type != "modifier" || e.Target != CriticalRangeTarget || len(e.Tags) > 0 {
		return
	}
	if v, ok := x.value(a); ok {
		x.d.CriticalRange = min(x.d.CriticalRange, min(max(v, MinCriticalRange), MaxCriticalRange))
	}
}

// rollAppliesValues is the menu's list of rolls.
func rollAppliesValues() []MenuValue {
	return namedMenu(rollApplies, func(k string) string { return rollAppliesNms[k] })
}

var rollAppliesNms = map[string]string{
	RollAppliesAttack: "Jogada de ataque (o d20)", RollAppliesCheck: "Teste de habilidade (o d20)",
	RollAppliesSave: "Teste de resistência (o d20)", RollDamage: "Dado de dano (de arma ou magia)",
}
