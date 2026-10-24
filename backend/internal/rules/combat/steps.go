package combat

import "slices"

// The steps that take a damage from what was rolled to what lands (SRD 5.1,
// "Damage Resistance and Vulnerability"): resistance and vulnerability apply
// after every other modifier, resistance first, so a damage halved by
// resistance and then doubled by vulnerability is the rolled number rounded
// down to an even half; multiple instances of resistance or vulnerability to the
// same type count as one; immunity takes the damage to 0.

// The kinds of a Step.
const (
	StepResistance    = "resistance"
	StepVulnerability = "vulnerability"
	StepImmunity      = "immunity"
)

// Modifier is a resistance, vulnerability or immunity to some damage types, from
// one source (a race, a feature, a state, a stat block).
type Modifier struct {
	// Kind is a Step* constant.
	Kind string
	// Source is its key: "race:tiefling", "feature:rage", "monster:skeleton".
	Source string
	// DamageTypes are the damage type keys it covers.
	DamageTypes []string
}

// Step is one change to a damage: what it was before, and after, and where it came from.
type Step struct {
	// Kind is a Step* constant.
	Kind string
	// Sources are the keys of every source counted in this step: several sources of
	// the same kind for one damage type count once, and all are named.
	Sources    []string
	DamageType string
	Before     int
	After      int
}

// Adjust takes a damage of one type through the modifiers that cover it. ignored
// holds the source keys the master told the app to leave out. It returns the damage
// that lands and the steps that changed it; a modifier that changes nothing is no step.
func Adjust(amount int, damageType string, mods []Modifier, ignored []string) (int, []Step) {
	amount = max(amount, 0)
	if damageType == "" || amount == 0 {
		return amount, nil
	}
	covering := func(kind string) (sources []string) {
		for _, m := range mods {
			if m.Kind == kind && slices.Contains(m.DamageTypes, damageType) && !slices.Contains(ignored, m.Source) && !slices.Contains(sources, m.Source) {
				sources = append(sources, m.Source)
			}
		}
		return sources
	}
	var steps []Step
	if src := covering(StepImmunity); len(src) > 0 {
		return 0, []Step{{Kind: StepImmunity, Sources: src, DamageType: damageType, Before: amount, After: 0}}
	}
	if src := covering(StepResistance); len(src) > 0 {
		after := amount / 2 // rounded down (Player's Handbook, "Rounding Down")
		steps = append(steps, Step{Kind: StepResistance, Sources: src, DamageType: damageType, Before: amount, After: after})
		amount = after
	}
	if src := covering(StepVulnerability); len(src) > 0 {
		after := amount * 2
		steps = append(steps, Step{Kind: StepVulnerability, Sources: src, DamageType: damageType, Before: amount, After: after})
		amount = after
	}
	return amount, steps
}
