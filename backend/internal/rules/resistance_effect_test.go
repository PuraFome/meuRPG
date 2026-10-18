package rules

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// The resistance effect: resistance to every damage type but one, while the
// character rages. Every name here is invented.

const (
	pathSubclass = "subclass:trilha-de-teste" + tableSuffix
	pathFeature  = "feature:trilha-resiste" + tableSuffix
	exceptedType = "damage-type:psychic"
)

func resistanceEffect() Effect {
	return Effect{Type: "resistance", Except: exceptedType, While: ResistanceWhileRage}
}

// pathOverlay is a table subclass of the SRD Barbarian whose level 3 feature
// carries the effect e.
func pathOverlay(e Effect) Overlay {
	return Overlay{Revision: 1, Subclasses: []TableSubclass{{
		Key: pathSubclass, NamePT: "Trilha de Teste", Class: "class:barbarian",
		Levels: []TableSubclassLevel{{Level: 3, Features: []TableFeature{tf("trilha-resiste", "Resistência da Trilha", e)}}},
	}}}
}

func pathBuild(level int) Build {
	return Build{
		BaseScores: map[Ability]int{STR: 15, DEX: 14, CON: 13, INT: 12, WIS: 10, CHA: 8},
		Race:       "race:human", Background: "background:acolyte",
		Classes: []ClassLevel{{Class: "class:barbarian", Level: level, Subclass: pathSubclass}},
	}
}

func TestResistanceEffectCoversEveryTypeButTheNamedOneWhileRaging(t *testing.T) {
	c := withOverlay(t, pathOverlay(resistanceEffect()))

	got, ok := resistanceOf(c.Resistances(Derive(pathBuild(3), c)), pathFeature)
	if !ok {
		t.Fatalf("the subclass feature gives no resistance at level 3")
	}
	if got.While != ResistanceWhileRage {
		t.Errorf("While = %q, want %q", got.While, ResistanceWhileRage)
	}
	_, _, types := c.c.namedEntries()
	all := make([]string, 0, len(types))
	for _, ty := range types {
		all = append(all, ty.Key)
	}
	if len(got.DamageTypes) != len(all)-1 || slices.Contains(got.DamageTypes, exceptedType) {
		t.Errorf("DamageTypes = %v, want the %d types but %s", got.DamageTypes, len(all)-1, exceptedType)
	}
	for _, k := range all {
		if k != exceptedType && !slices.Contains(got.DamageTypes, k) {
			t.Errorf("DamageTypes lacks %s", k)
		}
	}

	if _, ok := resistanceOf(c.Resistances(Derive(pathBuild(2), c)), pathFeature); ok {
		t.Error("the resistance holds before the feature's level")
	}
	// The class's own rage resistance stays, as a separate source.
	if _, ok := resistanceOf(c.Resistances(Derive(pathBuild(3), c)), "feature:rage"); !ok {
		t.Error("the barbarian's own resistance is gone")
	}
}

func TestResistanceEffectShowsAsAConditionalHintOnTheSheet(t *testing.T) {
	c := withOverlay(t, pathOverlay(resistanceEffect()))
	d := Derive(pathBuild(3), c)
	i := slices.IndexFunc(d.Hints, func(h Hint) bool { return h.Source == pathFeature })
	if i < 0 {
		t.Fatalf("no hint for the resistance: %+v", d.Hints)
	}
	if h := d.Hints[i]; h.Mode != "note" || !strings.Contains(h.TextPT, "fúria") || !strings.Contains(h.TextPT, c.c.namePT(exceptedType)) {
		t.Errorf("hint = %+v, want a note that says it holds in a rage and names the excepted type", h)
	}
}

func TestResistanceEffectRefusalsNameTheirField(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		edit  func(e *Effect)
		field string
	}{
		{"no excepted type", func(e *Effect) { e.Except = "" }, ".except"},
		{"an unknown damage type", func(e *Effect) { e.Except = "damage-type:fixture-none" }, ".except"},
		{"a key that is no damage type", func(e *Effect) { e.Except = "skill:arcana" }, ".except"},
		{"no condition", func(e *Effect) { e.While = "" }, ".while"},
		{"an unknown condition", func(e *Effect) { e.While = "always" }, ".while"},
		{"a formula condition", func(e *Effect) { e.When = "true" }, ".when"},
		{"tags", func(e *Effect) { e.Tags = []string{"about:fixture"} }, ".tags"},
		{"a field of another type", func(e *Effect) { e.Target = "ac" }, ".target"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := resistanceEffect()
			tc.edit(&e)
			o := pathOverlay(e)
			o.Strict = []string{pathSubclass}
			_, err := loadForTest(t).With(o)
			var oe *OverlayError
			if !errors.As(err, &oe) {
				t.Fatalf("want an OverlayError, got %v", err)
			}
			if !slices.ContainsFunc(oe.Violations(), func(v *OverlayError) bool { return strings.HasSuffix(v.Field, "effects[0]"+tc.field) }) {
				t.Errorf("violations = %+v, want one at effects[0]%s", oe.Violations(), tc.field)
			}
		})
	}
}

func TestResistanceEffectIsOnTheEffectMenu(t *testing.T) {
	menu := loadForTest(t).EffectMenu()
	i := slices.IndexFunc(menu.Types, func(m MenuType) bool { return m.Type == "resistance" })
	if i < 0 {
		t.Fatal("no resistance type on the menu")
	}
	for _, name := range []string{"except", "while"} {
		if !slices.ContainsFunc(menu.Types[i].Fields, func(f MenuField) bool { return f.Name == name && f.Required }) {
			t.Errorf("field %s is missing or not required", name)
		}
	}
	for _, name := range []string{ListDamageTypes, ListResistanceConditions} {
		if !slices.ContainsFunc(menu.Lists, func(l MenuList) bool { return l.Name == name && len(l.Values) > 0 }) {
			t.Errorf("list %s is missing or empty", name)
		}
	}
}
