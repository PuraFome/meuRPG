package rules

import (
	"slices"
	"strings"
)

// The aura of a feature (aura effect): a lasting effect the holder carries into every combat it joins, as a table
// spell's aura is, with no spell to cast and no end of its own. The play writes it as a row on the holder when the
// holder joins (see play/lasting_effects_aura_feature.go); the rules only check it and hand it over.

// FeatureAura is an aura effect that holds for a character: the feature that gives it and the effect as the combat
// reads it. Def.Key is the feature's key.
type FeatureAura struct {
	Source string
	Def    *EffectDef
}

// auraShape says what is wrong with an aura effect's own fields (the field and why), "" when right.
func auraShape(e *Effect) (field, msg string) {
	switch a := e.Aura; {
	case a == nil:
		return "aura", "aura is required: the aura the holder carries"
	case a.Aura == nil:
		return "aura.aura", "an aura effect needs the radius and the side (aura.aura)"
	case a.Applies != "" && a.Applies != "all":
		return "aura.applies", "a feature's aura reaches by its side and radius: it has no save or attack"
	case len(a.Conditions) > 0:
		return "aura.conditions", "an aura gives no condition: it is worked out where each creature stands at the roll"
	case len(e.Tags) > 0:
		return "tags", "an aura takes no tags"
	}
	return "", ""
}

// aura checks an aura effect: its fields, and no field but aura, when and text_pt. The effect the combat reads is
// made here, once, from the key of the feature.
func (x effectCompiler) aura() error {
	if field, msg := auraShape(x.e); field != "" {
		return x.fail("%s", msg)
	}
	if !x.takesOnly(func(o *Effect) { o.Type, o.Aura, o.When, o.TextPT = "", nil, "", ""; o.when, o.auraDef = nil, nil }) {
		return x.fail("an aura takes no fields but aura, when and text_pt")
	}
	for _, m := range x.e.Aura.Modifiers {
		if err := checkModifier(m); err != nil {
			return x.fail("%v", err)
		}
	}
	x.e.auraDef = featureAuraDef(x.key, x.e.Aura)
	return nil
}

// featureAuraDef is the effect the combat reads for the aura of the feature key.
func featureAuraDef(key string, a *TableSpellEffect) *EffectDef {
	ef := *a
	ef.Applies = ""
	def := tableEffectDef(&TableSpell{TableEntry: TableEntry{Key: key}, Effect: &ef})
	def.Kind = "feature"
	return def
}

// strayAura refuses the aura field on an effect that is no aura.
func (x effectCompiler) strayAura() error {
	if x.e.Type != "aura" && x.e.Aura != nil {
		return x.fail("aura belongs to the aura effect")
	}
	return nil
}

// checkAura checks an aura effect of the table's content at its own fields: the shape, and the effect through the
// checks of a table spell's (the aura lasts as long as the holder is in the combat).
func (b *overlayBuilder) checkAura(site effectSite, e *Effect) error {
	if field, msg := auraShape(e); field != "" {
		return site.fail("."+field, ReasonValue, "%s", msg)
	}
	ts := &TableSpell{TableEntry: TableEntry{Key: site.owner}, Effect: e.Aura, Duration: TableDuration{Kind: DurationTimed}}
	var c entryErrors
	b.checkLastingEffect(&c, site.owner, site.path+".aura", ts, SpellTarget{Kind: TargetSelf}, true)
	for _, v := range c.list {
		// The spell's own effect is the aura effect's `aura` field: its paths lose the "effect" step.
		v.Field = strings.Replace(v.Field, ".aura.effect", ".aura", 1)
	}
	return c.err()
}

// auraEffects fills Derived.Auras: the aura effects that hold, in the order of the effects. Of the effects of one
// feature the first whose `when` holds is carried: a level that widens the aura is a second effect.
func (x *deriver) auraEffects() {
	for _, a := range x.active {
		e := a.effect
		if e.Type != "aura" || e.auraDef == nil || !x.applies(a) {
			continue
		}
		if slices.ContainsFunc(x.d.Auras, func(o FeatureAura) bool { return o.Source == a.owner }) {
			continue
		}
		x.d.Auras = append(x.d.Auras, FeatureAura{Source: a.owner, Def: e.auraDef})
	}
}
