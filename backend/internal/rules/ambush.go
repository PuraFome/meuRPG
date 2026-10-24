package rules

import "slices"

// What an ambush effect gives its owner against a target (its `applies`).
const (
	// AmbushUnactedAdvantage gives advantage on attack rolls against a creature that has not taken
	// a turn in the combat yet.
	AmbushUnactedAdvantage = "unacted_advantage"
	// AmbushSurprisedCritical makes a hit against a surprised creature a critical hit.
	AmbushSurprisedCritical = "surprised_critical"
)

var ambushApplies = []string{AmbushUnactedAdvantage, AmbushSurprisedCritical}

// ambush checks the fields of an ambush effect.
func (x effectCompiler) ambush() error {
	if !slices.Contains(ambushApplies, x.e.Applies) {
		return x.fail("unknown applies %q", x.e.Applies)
	}
	return nil
}

// checkOnTarget checks the effects that ask something of the target: extra_damage and ambush.
func (b *overlayBuilder) checkOnTarget(site effectSite, e *Effect) error {
	if e.Type == "ambush" {
		return b.checkAmbush(site, e)
	}
	return b.checkExtraDamage(site, e)
}

// checkAmbush checks an ambush of the table's content: what it gives, from the closed list.
func (b *overlayBuilder) checkAmbush(site effectSite, e *Effect) error {
	if !slices.Contains(ambushApplies, e.Applies) {
		return site.fail(".applies", ReasonValue, "applies %q is not on the table's menu", e.Applies)
	}
	return nil
}

// ambushes fills Derived.UnactedAdvantage and Derived.SurprisedCritical with the key of the feature
// that gives each, from the ambush effects the character has.
func (x *deriver) ambushes() {
	for _, a := range x.active {
		if a.effect.Type != "ambush" || !x.applies(a) {
			continue
		}
		switch a.effect.Applies {
		case AmbushUnactedAdvantage:
			x.d.UnactedAdvantage = a.owner
		case AmbushSurprisedCritical:
			x.d.SurprisedCritical = a.owner
		}
	}
}

func ambushValues() []MenuValue {
	out := make([]MenuValue, 0, len(ambushApplies))
	for _, k := range ambushApplies {
		out = append(out, MenuValue{Key: k, NamePT: ambushNms[k][0], HintPT: ambushNms[k][1]})
	}
	return out
}
