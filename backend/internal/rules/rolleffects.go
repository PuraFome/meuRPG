package rules

import (
	"slices"
	"strconv"

	"github.com/PuraFome/meuRPG/backend/internal/rules/formula"
)

// The rolls a reroll or a roll_minimum effect covers (its `rolls`): the d20 of an attack roll, of an
// ability check and of a saving throw, and a die of weapon or spell damage.
const (
	RollAttack = "attack"
	RollCheck  = "check"
	RollSave   = "save"
	RollDamage = "damage"
)

var rollApplies = []string{RollAttack, RollCheck, RollSave, RollDamage}

// The bounds of the numbers of the roll effects.
const (
	// MinCriticalRange and MaxCriticalRange bound the lowest natural d20 that is a critical hit.
	MinCriticalRange = 15
	MaxCriticalRange = 20
	// MinRerollAtMost and MaxRerollAtMost bound a reroll's at_most: a die showing it or lower may be rolled again.
	MinRerollAtMost = 1
	MaxRerollAtMost = 20
	// MinRollMinimum and MaxRollMinimum bound a roll_minimum's value: a die that shows less counts as it.
	MinRollMinimum = 2
	MaxRollMinimum = 20
)

// CriticalRangeTarget is the modifier target that widens the critical range: the lowest natural d20 that
// is a critical hit. Only the mode set is meaningful, and only the deriver reads it (Derived.CriticalRange).
const CriticalRangeTarget = "critical_range"

// RerollRule is a reroll effect the character has: a die of the covered rolls that shows AtMost or lower may
// be rolled again, and the new roll must be used (SRD 5.1, Lucky). Each reroll spends Cost uses of Resource
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

// RollMinimum is a roll_minimum effect the character has: a die of the covered rolls that shows less than
// Value counts as Value (the SRD's Reliable Talent and Elemental Adept shape).
type RollMinimum struct {
	// Applies are the rolls it covers (Roll*).
	Applies []string
	// Value is the least a die counts as.
	Value int
	// Source is the feature (or option) key that gives it.
	Source string
}

// rollsShape says what is wrong with the list of rolls of a roll effect: the field and why, both "" when
// right. It is the shared check of the engine's load and of the table's menu.
func rollsShape(e *Effect) (field, msg string) {
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

// rollMinimumShape says what is wrong with a roll_minimum effect's own fields.
func rollMinimumShape(e *Effect) (field, msg string) {
	if f, m := rollsShape(e); f != "" {
		return f, m
	}
	n, err := strconv.Atoi(e.Value)
	switch {
	case err != nil || n < MinRollMinimum || n > MaxRollMinimum:
		return "value", "value is a whole number from 2 to 20"
	case e.Resource != "":
		return "resource", "a roll minimum spends no resource"
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

// rollMinimum checks a roll_minimum effect.
func (x effectCompiler) rollMinimum() error {
	if field, msg := rollMinimumShape(x.e); field != "" {
		return x.fail("%s", msg)
	}
	return nil
}

// checkRoll checks a reroll or roll_minimum of the table's content at its own fields.
func (b *overlayBuilder) checkRoll(site effectSite, e *Effect) error {
	shape := rollMinimumShape
	if e.Type == "reroll" {
		shape = rerollShape
		if e.Resource != "" && !b.resourceDefined(site.owner, e.Resource) {
			return site.fail(".resource", ReasonReference, "the reroll spends %q, which no resource effect of this entry or its class defines", e.Resource)
		}
	}
	if field, msg := shape(e); field != "" {
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

// rollEffects fills Derived.Rerolls and Derived.RollMinimums from the effects the character has, in the
// order of the effects.
func (x *deriver) rollEffects() {
	for _, a := range x.active {
		e := a.effect
		switch {
		case !x.applies(a):
		case e.Type == "roll_minimum":
			n, _ := strconv.Atoi(e.Value)
			x.d.RollMinimums = append(x.d.RollMinimums, RollMinimum{Applies: slices.Clone(e.Rolls), Value: n, Source: a.owner})
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
// it spends does not exist at this level, or the cost formula failed or gave less than 1.
func (x *deriver) rerollCost(a activeEffect) (int, bool) {
	e := a.effect
	if e.Resource == "" {
		return 1, true
	}
	if !slices.ContainsFunc(x.d.Resources, func(r Resource) bool { return r.Key == e.Resource }) {
		return 0, false
	}
	if e.cost == nil {
		return 1, true
	}
	n, ok := x.evalInt(e.cost, a)
	return n, ok && n >= 1
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
	RollAttack: "Jogada de ataque (o d20)", RollCheck: "Teste de habilidade (o d20)",
	RollSave: "Teste de resistência (o d20)", RollDamage: "Dado de dano (de arma ou magia)",
}
