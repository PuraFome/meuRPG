package combat

import "slices"

// The steps that take a damage from what was rolled to what lands (SRD 5.1,
// "Damage Resistance and Vulnerability"): resistance and vulnerability apply
// after every other modifier, resistance first, so a damage halved by
// resistance and then doubled by vulnerability is the rolled number rounded
// down to an even half; multiple instances of resistance or vulnerability to the
// same type count as one; immunity takes the damage to 0. A resistance or immunity
// can be limited to attacks that are not magical (the stat blocks' "from nonmagical
// attacks"): it covers a damage only when the damage comes from an attack and the
// attack is not magical.

// The kinds of a Step.
const (
	StepResistance    = "resistance"
	StepVulnerability = "vulnerability"
	StepImmunity      = "immunity"
)

// QualifierNonmagical limits a Modifier to the damage of attacks that are not magical.
const QualifierNonmagical = "nonmagical"

// Modifier is a resistance, vulnerability or immunity to some damage types, from
// one source (a race, a feature, a state, a stat block).
type Modifier struct {
	// Kind is a Step* constant.
	Kind string
	// Source is its key: "race:tiefling", "feature:rage", "monster:skeleton".
	Source string
	// DamageTypes are the damage type keys it covers.
	DamageTypes []string
	// Qualifier limits it: "" covers every damage of its types, QualifierNonmagical only
	// the damage of a nonmagical attack.
	Qualifier string `json:",omitempty"`
}

// Origin says where a damage comes from, for the modifiers that depend on it. The zero
// Origin is a damage that is no attack: a trap, a hazard, a fall, the damage of an
// ongoing effect.
type Origin struct {
	// Attack says the damage is that of an attack (a weapon, a spell attack, the damage
	// of a spell).
	Attack bool `json:"attack,omitempty"`
	// Magical says the attack is magical: a spell, a magic weapon, a creature's attack
	// with the trait "Magic Weapons", an attack the sheet marks.
	Magical bool `json:"magical,omitempty"`
}

// Covers says the modifier applies to a damage of this type from this origin.
func (m Modifier) Covers(damageType string, from Origin) bool {
	if !slices.Contains(m.DamageTypes, damageType) {
		return false
	}
	switch m.Qualifier {
	case "":
		return true
	case QualifierNonmagical:
		return from.Attack && !from.Magical
	}
	return false
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
	// Qualifier is the Modifier's when every source of the step has it: the step holds
	// only for that kind of damage. It is "" when any source covers every damage.
	Qualifier string
}

// Adjust takes a damage of one type through the modifiers that cover it. ignored
// holds the source keys the master told the app to leave out; from says where the
// damage comes from. It returns the damage that lands and the steps that changed it; a
// modifier that changes nothing is no step.
func Adjust(amount int, damageType string, mods []Modifier, ignored []string, from Origin) (int, []Step) {
	amount = max(amount, 0)
	if damageType == "" || amount == 0 {
		return amount, nil
	}
	covering := func(kind string) (sources []string, qualifier string) {
		plain := false
		for _, m := range mods {
			if m.Kind != kind || !m.Covers(damageType, from) || slices.Contains(ignored, m.Source) || slices.Contains(sources, m.Source) {
				continue
			}
			sources = append(sources, m.Source)
			plain = plain || m.Qualifier == ""
			qualifier = m.Qualifier
		}
		if plain {
			qualifier = ""
		}
		return sources, qualifier
	}
	var steps []Step
	if src, q := covering(StepImmunity); len(src) > 0 {
		return 0, []Step{{Kind: StepImmunity, Sources: src, DamageType: damageType, Before: amount, After: 0, Qualifier: q}}
	}
	if src, q := covering(StepResistance); len(src) > 0 {
		after := amount / 2 // rounded down (Player's Handbook, "Rounding Down")
		steps = append(steps, Step{Kind: StepResistance, Sources: src, DamageType: damageType, Before: amount, After: after, Qualifier: q})
		amount = after
	}
	if src, q := covering(StepVulnerability); len(src) > 0 {
		after := amount * 2
		steps = append(steps, Step{Kind: StepVulnerability, Sources: src, DamageType: damageType, Before: amount, After: after, Qualifier: q})
		amount = after
	}
	return amount, steps
}
