package rules

import (
	"slices"

	"github.com/PuraFome/meuRPG/backend/internal/rules/formula"
)

// What an extra_damage effect asks of the target (its `applies`).
const (
	// ExtraDamageAnyTarget adds the damage to a weapon hit on any creature.
	ExtraDamageAnyTarget = "any_target"
	// ExtraDamageLargerTarget adds it only when the target is of a larger size than the attacker.
	ExtraDamageLargerTarget = "larger_target"
)

var extraDamageApplies = []string{ExtraDamageAnyTarget, ExtraDamageLargerTarget}

// Bounds of the damage an extra_damage adds: a formula that gives anything else is ignored.
const (
	minExtraDamage = 1
	maxExtraDamage = 999
)

// ExtraDamage is an extra_damage effect of a feature the character has: damage added to a
// weapon hit once per turn, at the player's choice, when the target is what Applies says.
// The engine is generic; the table names it.
type ExtraDamage struct {
	// Key is the feature that gives it, NamePT its name.
	Key, NamePT string
	// Amount is the damage, worked out for the character; it is a fixed number, never doubled
	// by a critical hit.
	Amount int
	// Applies is one of the ExtraDamage* constants.
	Applies string
	// Resource is the resource each use spends, one use each; empty when it only has the
	// once-per-turn limit.
	Resource string
	// TextPT is the line the master reads.
	TextPT string
}

// extraDamage checks the fields of an extra_damage effect.
func (x effectCompiler) extraDamage() error {
	e := x.e
	if !slices.Contains(extraDamageApplies, e.Applies) {
		return x.fail("unknown applies %q", e.Applies)
	}
	if e.Value == "" {
		return x.fail("value is required: the damage added")
	}
	var err error
	if e.value, err = x.compile(e.Value, formula.Int); err != nil {
		return err
	}
	return nil
}

// extraDamages fills Derived.ExtraDamages from the extra_damage effects the character has.
// A resource the character does not have at this level leaves the effect out, as for a maneuver.
func (x *deriver) extraDamages() {
	for _, a := range x.active {
		e := a.effect
		if e.Type != "extra_damage" || !x.applies(a) {
			continue
		}
		if e.Resource != "" && !slices.ContainsFunc(x.d.Resources, func(r Resource) bool { return r.Key == e.Resource }) {
			continue
		}
		amount, err := e.value.Int(x.env)
		if err != nil || amount < minExtraDamage || amount > maxExtraDamage {
			x.issue(IssueFormula, "", "Um efeito de %s foi ignorado: o dano extra não é válido.", x.c.namePT(a.owner))
			continue
		}
		x.d.ExtraDamages = append(x.d.ExtraDamages, ExtraDamage{
			Key: a.owner, NamePT: x.c.namePT(a.owner), Amount: amount, Applies: e.Applies, Resource: e.Resource, TextPT: e.TextPT,
		})
	}
}

// checkExtraDamage checks an extra_damage of the table's content: what it asks of the target and
// the resource it spends, which an entry or its class defines.
func (b *overlayBuilder) checkExtraDamage(site effectSite, e *Effect) error {
	if !slices.Contains(extraDamageApplies, e.Applies) {
		return site.fail(".applies", ReasonValue, "applies %q is not on the table's menu", e.Applies)
	}
	if e.Value == "" {
		return site.fail(".value", ReasonValue, "the damage added needs a value")
	}
	if e.Resource != "" && !b.resourceDefined(site.owner, e.Resource) {
		return site.fail(".resource", ReasonReference, "the extra damage spends %q, which no resource effect of this entry or its class defines", e.Resource)
	}
	return nil
}

func extraDamageValues() []MenuValue {
	out := make([]MenuValue, 0, len(extraDamageApplies))
	for _, k := range extraDamageApplies {
		out = append(out, MenuValue{Key: k, NamePT: extraDamageNms[k][0], HintPT: extraDamageNms[k][1]})
	}
	return out
}
