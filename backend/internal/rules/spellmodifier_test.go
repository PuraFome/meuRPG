package rules

import (
	"errors"
	"slices"
	"testing"
)

// The spell_modifier effect: a cast option a feature offers for the spells it fits. The names are invented;
// the engine is generic.

const (
	optSubclass = "subclass:tecelao-de-brasas" + tableSuffix
	optFeature  = "feature:brasa-contida" + tableSuffix
	optUses     = "brasas"
)

func spellModifierOverlay(effects ...Effect) Overlay {
	if effects == nil {
		effects = []Effect{
			{Type: "resource", Resource: optUses, Max: "3", Recharge: "long_rest"},
			{Type: "spell_modifier", AppliesTo: "school:evocation", SpellChange: SpellChangeBonusDamage, Value: "prof()", DamageType: "damage-type:fire", Resource: optUses, TextPT: "Brasas."},
		}
	}
	return Overlay{Revision: 1, Subclasses: []TableSubclass{{
		Key: optSubclass, NamePT: "Tecelão de Brasas", Class: "class:fighter",
		Levels: []TableSubclassLevel{{Level: 3, Features: []TableFeature{{
			Key: optFeature, NamePT: "Brasa Contida", DescPT: []string{"Uma opção de magia."}, Effects: effects,
		}}}},
	}}}
}

func optBuild(level int) Build {
	b := postureBuild(level)
	b.Classes[0].Subclass = optSubclass
	return b
}

func TestDeriveExposesTheSpellModifierWorkedOut(t *testing.T) {
	c := withOverlay(t, spellModifierOverlay())
	b := optBuild(5)
	d := Derive(b, c)
	if len(d.SpellModifiers) != 1 {
		t.Fatalf("spell modifiers = %+v, want one", d.SpellModifiers)
	}
	m := d.SpellModifiers[0]
	if m.Key != optFeature || m.AppliesTo != "school:evocation" || m.Change != SpellChangeBonusDamage || m.BonusType != "damage-type:fire" || m.Resource != optUses || m.TextPT != "Brasas." || m.Bonus < 1 {
		t.Errorf("spell modifier = %+v", m)
	}
}

func TestSpellModifierKindsKeepTheirFields(t *testing.T) {
	c := withOverlay(t, spellModifierOverlay(
		Effect{Type: "spell_modifier", AppliesTo: "damage_spell", SpellChange: SpellChangeDamageTypeSwap, From: []string{"damage-type:fire", "damage-type:cold"}},
	))
	m := Derive(optBuild(3), c).SpellModifiers
	if len(m) != 1 || !slices.Equal(m[0].From, []string{"damage-type:fire", "damage-type:cold"}) || m[0].Resource != "" {
		t.Fatalf("spell modifiers = %+v, want a free swap of fire and cold", m)
	}
	c = withOverlay(t, spellModifierOverlay(Effect{Type: "spell_modifier", AppliesTo: "cantrip", SpellChange: SpellChangeExtraDice, Count: 2}))
	if m := Derive(optBuild(3), c).SpellModifiers; len(m) != 1 || m[0].ExtraDice != 2 {
		t.Errorf("spell modifiers = %+v, want 2 extra dice", m)
	}
}

func TestSpellModifierIsOnlyTheFeaturesOwner(t *testing.T) {
	c := withOverlay(t, spellModifierOverlay())
	if got := Derive(optBuild(3), c); len(got.SpellModifiers) != 1 {
		t.Fatalf("at level 3 = %+v", got.SpellModifiers)
	}
	if got := Derive(optBuild(2), c); len(got.SpellModifiers) != 0 {
		t.Fatalf("at level 2 = %+v, want none before the feature", got.SpellModifiers)
	}
	b := optBuild(3)
	b.Classes[0].Subclass = ""
	if got := Derive(b, c).SpellModifiers; len(got) != 0 {
		t.Errorf("without the subclass = %+v, want none", got)
	}
}

func TestSpellModifierRefusalsNameTheirField(t *testing.T) {
	t.Parallel()
	const at = "subclasses[0].levels[0].features[0].effects[0]"
	fire, cold := "damage-type:fire", "damage-type:cold"
	cases := []struct {
		name          string
		effect        Effect
		field, reason string
	}{
		{"no applies_to", Effect{Type: "spell_modifier", SpellChange: SpellChangeBonusDamage, Value: "1"}, at + ".applies_to", ReasonValue},
		{"an applies_to that is nothing", Effect{Type: "spell_modifier", AppliesTo: "school:sonho", SpellChange: SpellChangeBonusDamage, Value: "1"}, at + ".applies_to", ReasonReference},
		{"a damage type of a school", Effect{Type: "spell_modifier", AppliesTo: "skill:stealth", SpellChange: SpellChangeBonusDamage, Value: "1"}, at + ".applies_to", ReasonReference},
		{"no spell_change", Effect{Type: "spell_modifier", AppliesTo: "cantrip", Value: "1"}, at + ".spell_change", ReasonValue},
		{"an unknown spell_change", Effect{Type: "spell_modifier", AppliesTo: "cantrip", SpellChange: "range", Value: "1"}, at + ".spell_change", ReasonValue},
		{"a swap of one type", Effect{Type: "spell_modifier", AppliesTo: "cantrip", SpellChange: SpellChangeDamageTypeSwap, From: []string{fire}}, at + ".from", ReasonValue},
		{"a swap of the same type twice", Effect{Type: "spell_modifier", AppliesTo: "cantrip", SpellChange: SpellChangeDamageTypeSwap, From: []string{fire, fire}}, at + ".from", ReasonValue},
		{"a swap of a thing that is no damage type", Effect{Type: "spell_modifier", AppliesTo: "cantrip", SpellChange: SpellChangeDamageTypeSwap, From: []string{fire, "skill:stealth"}}, at + ".from", ReasonReference},
		{"a swap with a value", Effect{Type: "spell_modifier", AppliesTo: "cantrip", SpellChange: SpellChangeDamageTypeSwap, From: []string{fire, cold}, Value: "1"}, at + ".value", ReasonValue},
		{"a bonus without a value", Effect{Type: "spell_modifier", AppliesTo: "cantrip", SpellChange: SpellChangeBonusDamage}, at + ".value", ReasonValue},
		{"a bonus with a formula that does not compile", Effect{Type: "spell_modifier", AppliesTo: "cantrip", SpellChange: SpellChangeBonusDamage, Value: "mod("}, at + ".value", ReasonFormula},
		{"a bonus of a type that does not exist", Effect{Type: "spell_modifier", AppliesTo: "cantrip", SpellChange: SpellChangeBonusDamage, Value: "1", DamageType: "damage-type:sonho"}, at + ".damage_type", ReasonReference},
		{"a bonus with dice count", Effect{Type: "spell_modifier", AppliesTo: "cantrip", SpellChange: SpellChangeBonusDamage, Value: "1", Count: 1}, at + ".from", ReasonValue},
		{"extra dice without count", Effect{Type: "spell_modifier", AppliesTo: "cantrip", SpellChange: SpellChangeExtraDice}, at + ".count", ReasonValue},
		{"extra dice too many", Effect{Type: "spell_modifier", AppliesTo: "cantrip", SpellChange: SpellChangeExtraDice, Count: 7}, at + ".count", ReasonValue},
		{"a resource nobody defines", Effect{Type: "spell_modifier", AppliesTo: "cantrip", SpellChange: SpellChangeExtraDice, Count: 1, Resource: "fantasma"}, at + ".resource", ReasonReference},
		{"an economy", Effect{Type: "spell_modifier", AppliesTo: "cantrip", SpellChange: SpellChangeExtraDice, Count: 1, Economy: "action"}, at + ".economy", ReasonValue},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			o := spellModifierOverlay(tc.effect)
			o.Strict = []string{optSubclass}
			_, err := loadForTest(t).With(o)
			if err == nil {
				t.Fatal("the effect is accepted")
			}
			var oe *OverlayError
			if !errors.As(err, &oe) || oe.Field != tc.field || oe.Reason != tc.reason {
				t.Errorf("error = %v, want field %s reason %s", err, tc.field, tc.reason)
			}
		})
	}
}

func TestAFeatureHasOneSpellModifier(t *testing.T) {
	t.Parallel()
	two := spellModifierOverlay(
		Effect{Type: "spell_modifier", AppliesTo: "cantrip", SpellChange: SpellChangeExtraDice, Count: 1},
		Effect{Type: "spell_modifier", AppliesTo: "cantrip", SpellChange: SpellChangeExtraDice, Count: 2},
	)
	_, err := loadForTest(t).With(two)
	var oe *OverlayError
	if !errors.As(err, &oe) || oe.Field != "subclasses[0].levels[0].features[0].effects[1].type" || oe.Reason != ReasonEffect {
		t.Errorf("error = %v, want the second spell_modifier refused at its type", err)
	}
}

func TestSpellModifierFieldsOnAnotherEffectTypeAreRefused(t *testing.T) {
	t.Parallel()
	const at = "subclasses[0].levels[0].features[0].effects[0]"
	for field, e := range map[string]Effect{
		"applies_to":   {Type: "note", AppliesTo: "cantrip"},
		"spell_change": {Type: "note", SpellChange: SpellChangeBonusDamage},
	} {
		o := spellModifierOverlay(e)
		o.Strict = []string{optSubclass}
		_, err := loadForTest(t).With(o)
		var oe *OverlayError
		if !errors.As(err, &oe) || oe.Field != at+"."+field || oe.Reason != ReasonValue {
			t.Errorf("%s: error = %v, want it refused at the field", field, err)
		}
	}
}

func TestSpellModifierIsOnTheMenu(t *testing.T) {
	menu := withOverlay(t, spellModifierOverlay()).EffectMenu()
	i := slices.IndexFunc(menu.Types, func(ty MenuType) bool { return ty.Type == "spell_modifier" })
	if i < 0 {
		t.Fatal("no spell_modifier on the menu")
	}
	for _, name := range []string{"applies_to", "spell_change", "from", "value", "damage_type", "count", "resource", "when", "text_pt"} {
		if !slices.ContainsFunc(menu.Types[i].Fields, func(f MenuField) bool { return f.Name == name }) {
			t.Errorf("the menu type has no %s field", name)
		}
	}
	list := func(name string) []MenuValue {
		j := slices.IndexFunc(menu.Lists, func(l MenuList) bool { return l.Name == name })
		if j < 0 {
			t.Fatalf("no %s list", name)
		}
		return menu.Lists[j].Values
	}
	has := func(vs []MenuValue, key string) bool {
		return slices.ContainsFunc(vs, func(v MenuValue) bool { return v.Key == key })
	}
	targets := list(ListSpellModifierTargets)
	for _, k := range []string{SpellAppliesDamageSpell, SpellAppliesCantrip, "school:evocation", "damage-type:fire"} {
		if !has(targets, k) {
			t.Errorf("the targets list has no %s", k)
		}
	}
	if got := list(ListSpellChanges); len(got) != 3 {
		t.Errorf("the changes list = %+v, want three", got)
	}
}
