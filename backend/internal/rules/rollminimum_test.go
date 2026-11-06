package rules

import (
	"errors"
	"slices"
	"testing"
)

// The roll_minimum effect and the scope of the critical range, on the table's content. It reuses the
// overlay of rolleffects_test.go; the names are invented.

func TestDeriveExposesTheRollMinimumsInTheOrderOfTheEffects(t *testing.T) {
	t.Parallel()
	c := withOverlay(t, rollOverlay(
		[]Effect{
			{Type: "roll_minimum", Rolls: []string{RollAppliesAttack, RollAppliesSave}, Value: "10"},
			{Type: "roll_minimum", Rolls: []string{RollDamage}, Value: "2", DamageType: "damage-type:fire", When: "level() >= 3"},
		},
		[]Effect{
			{Type: "roll_minimum", Rolls: []string{RollDamage}, Value: "3", When: "level() >= 5"},
			{Type: "roll_minimum", Rolls: []string{RollAppliesCheck}, Value: "9"},
		}))
	want := []RollMinimum{
		{Applies: []string{RollAppliesAttack, RollAppliesSave}, Value: 10, Source: rollFirst},
		{Applies: []string{RollDamage}, Value: 2, DamageType: "damage-type:fire", Source: rollFirst},
		{Applies: []string{RollAppliesCheck}, Value: 9, Source: rollSecond},
	}
	for range 3 { // the order is stable across derivations
		got := Derive(rollBuild(3), c).RollMinimums
		if !slices.EqualFunc(got, want, func(a, b RollMinimum) bool {
			return slices.Equal(a.Applies, b.Applies) && a.Value == b.Value && a.DamageType == b.DamageType && a.Source == b.Source
		}) {
			t.Fatalf("roll minimums = %+v, want %+v (the level 5 one not yet)", got, want)
		}
	}
	if got := Derive(rollBuild(5), c).RollMinimums; len(got) != 4 {
		t.Errorf("level 5 roll minimums = %+v, want the three of level 3 and the one of level 5", got)
	}
	if got := Derive(rollBuild(2), c).RollMinimums; len(got) != 0 {
		t.Errorf("before the features: %+v, want none", got)
	}
}

func TestARollMinimumNeedsNoResource(t *testing.T) {
	t.Parallel()
	// The entry defines no resource at all: the minimum is always there.
	c := withOverlay(t, rollOverlay([]Effect{{Type: "roll_minimum", Rolls: []string{RollDamage}, Value: "2"}}, nil))
	if got := Derive(rollBuild(3), c).RollMinimums; len(got) != 1 || got[0].Value != 2 {
		t.Errorf("roll minimums = %+v, want the one minimum", got)
	}
}

func TestRollMinimumRefusalsNameTheirField(t *testing.T) {
	t.Parallel()
	const at = "subclasses[0].levels[0].features[0].effects["
	rm := func(mut func(e *Effect)) Effect {
		e := Effect{Type: "roll_minimum", Rolls: []string{RollAppliesAttack}, Value: "10"}
		mut(&e)
		return e
	}
	crit := func(applies string) Effect {
		return Effect{Type: "modifier", Target: CriticalRangeTarget, Mode: "set", Value: "19", Applies: applies}
	}
	cases := []struct {
		name          string
		effect        Effect
		field, reason string
	}{
		{"value 1", rm(func(e *Effect) { e.Value = "1" }), ".value", ReasonValue},
		{"value 21", rm(func(e *Effect) { e.Value = "21" }), ".value", ReasonValue},
		{"value empty", rm(func(e *Effect) { e.Value = "" }), ".value", ReasonValue},
		{"value is a formula", rm(func(e *Effect) { e.Value = "prof()" }), ".value", ReasonValue},
		{"empty rolls", rm(func(e *Effect) { e.Rolls = nil }), ".rolls", ReasonValue},
		{"unknown roll", rm(func(e *Effect) { e.Rolls = []string{"initiative"} }), ".rolls", ReasonValue},
		{"repeated roll", rm(func(e *Effect) { e.Rolls = []string{RollDamage, RollDamage} }), ".rolls", ReasonValue},
		{"tags", rm(func(e *Effect) { e.Tags = []string{"against:magic"} }), ".tags", ReasonValue},
		{"at_most", rm(func(e *Effect) { e.AtMost = 2 }), ".at_most", ReasonValue},
		{"resource", rm(func(e *Effect) { e.Resource = rollUses }), ".resource", ReasonValue},
		{"cost", rm(func(e *Effect) { e.Cost = "1" }), ".cost", ReasonValue},
		{"damage_type on a d20 roll", rm(func(e *Effect) { e.DamageType = "damage-type:fire" }), ".damage_type", ReasonValue},
		{"damage_type with more than damage", rm(func(e *Effect) {
			e.Rolls, e.DamageType = []string{RollAppliesAttack, RollDamage}, "damage-type:fire"
		}), ".damage_type", ReasonValue},
		{"unknown damage_type", rm(func(e *Effect) { e.Rolls, e.DamageType = []string{RollDamage}, "damage-type:luck" }), ".damage_type", ReasonReference},
		{"stray applies", rm(func(e *Effect) { e.Applies = "attack" }), ".applies", ReasonValue},
		{"critical_range with an unknown scope", crit("melee"), ".applies", ReasonValue},
		{"scope on another target", Effect{Type: "modifier", Target: "ac", Mode: "add", Value: "1", Applies: CriticalScopeSpell}, ".applies", ReasonValue},
		{"rolls on a modifier", Effect{Type: "modifier", Target: "ac", Mode: "add", Value: "1", Rolls: []string{RollDamage}}, ".rolls", ReasonValue},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			o := rollOverlay(luck(tc.effect), nil)
			o.Strict = []string{rollSubclass}
			_, err := loadForTest(t).With(o)
			var oe *OverlayError
			if !errors.As(err, &oe) || oe.Field != at+"1]"+tc.field || oe.Reason != tc.reason {
				t.Errorf("error = %v, want field %s1]%s reason %s", err, at, tc.field, tc.reason)
			}
		})
	}
}

func TestRollMinimumAndScopeAreAcceptedWhenRight(t *testing.T) {
	t.Parallel()
	for _, e := range []Effect{
		{Type: "roll_minimum", Rolls: []string{RollAppliesAttack, RollAppliesCheck, RollAppliesSave, RollDamage}, Value: "20"},
		{Type: "roll_minimum", Rolls: []string{RollDamage}, Value: "2"},
		{Type: "roll_minimum", Rolls: []string{RollDamage}, Value: "2", DamageType: "damage-type:fire", TextPT: "Mínimo.", When: "level() >= 3"},
		{Type: "modifier", Target: CriticalRangeTarget, Mode: "set", Value: "19", Applies: CriticalScopeWeapon},
		{Type: "modifier", Target: CriticalRangeTarget, Mode: "set", Value: "19", Applies: CriticalScopeSpell},
		{Type: "modifier", Target: CriticalRangeTarget, Value: "19", Applies: CriticalScopeBoth},
	} {
		o := rollOverlay(luck(e), nil)
		o.Strict = []string{rollSubclass}
		if _, err := loadForTest(t).With(o); err != nil {
			t.Errorf("%+v refused: %v", e, err)
		}
	}
}

func TestRollMinimumThroughTheEngineLoad(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)
	for name, e := range map[string]*Effect{
		"value 1":            {Type: "roll_minimum", Rolls: []string{RollDamage}, Value: "1"},
		"value 21":           {Type: "roll_minimum", Rolls: []string{RollDamage}, Value: "21"},
		"no rolls":           {Type: "roll_minimum", Value: "5"},
		"a tag":              {Type: "roll_minimum", Rolls: []string{RollDamage}, Value: "5", Tags: []string{"against:magic"}},
		"damage_type on d20": {Type: "roll_minimum", Rolls: []string{RollAppliesSave}, Value: "5", DamageType: "damage-type:fire"},
		"unknown damage":     {Type: "roll_minimum", Rolls: []string{RollDamage}, Value: "5", DamageType: "damage-type:luck"},
		"rolls on ambush":    {Type: "ambush", Applies: AmbushUnactedAdvantage, Rolls: []string{RollDamage}},
		"unknown scope":      {Type: "modifier", Target: CriticalRangeTarget, Mode: "set", Value: "19", Applies: "melee"},
	} {
		if err := srd.c.compileEffect("feature:x", e); err == nil {
			t.Errorf("%s: the loader accepted it", name)
		}
	}
	for name, e := range map[string]*Effect{
		"a minimum":       {Type: "roll_minimum", Rolls: []string{RollAppliesAttack, RollDamage}, Value: "10"},
		"a typed minimum": {Type: "roll_minimum", Rolls: []string{RollDamage}, Value: "2", DamageType: "damage-type:fire"},
		"a spell scope":   {Type: "modifier", Target: CriticalRangeTarget, Mode: "set", Value: "19", Applies: CriticalScopeSpell},
		"a weapon scope":  {Type: "modifier", Target: CriticalRangeTarget, Mode: "set", Value: "19", Applies: CriticalScopeWeapon},
	} {
		if err := srd.c.compileEffect("feature:x", e); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func critical(scope, value string, extra ...func(*Effect)) Effect {
	e := Effect{Type: "modifier", Target: CriticalRangeTarget, Mode: "set", Value: value, Applies: scope}
	for _, f := range extra {
		f(&e)
	}
	return e
}

func TestCriticalRangeScope(t *testing.T) {
	t.Parallel()
	tagged := func(e *Effect) { e.Tags = []string{"against:undead"} }
	for _, tc := range []struct {
		name          string
		effects       []Effect
		weapon, spell int
	}{
		{"no modifier", nil, 20, 20},
		{"the default scope is the weapon's", []Effect{critical("", "18")}, 18, 20},
		{"weapon scope", []Effect{critical(CriticalScopeWeapon, "18")}, 18, 20},
		{"spell scope", []Effect{critical(CriticalScopeSpell, "18")}, 20, 18},
		{"both", []Effect{critical(CriticalScopeBoth, "17")}, 17, 17},
		{"the lowest of each range", []Effect{critical(CriticalScopeWeapon, "19"), critical(CriticalScopeSpell, "18"), critical(CriticalScopeBoth, "19")}, 19, 18},
		{"both and a lower weapon", []Effect{critical(CriticalScopeBoth, "19"), critical("", "17")}, 17, 19},
		{"a spell formula is clamped to 15", []Effect{critical(CriticalScopeSpell, "level() - 10")}, 20, 15},
		{"a tagged spell scope is a hint only", []Effect{critical(CriticalScopeSpell, "18", tagged)}, 20, 20},
		{"a false when keeps the spell range", []Effect{critical(CriticalScopeSpell, "18", func(e *Effect) { e.When = "level() >= 10" })}, 20, 20},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			d := Derive(rollBuild(3), withOverlay(t, rollOverlay(tc.effects, nil)))
			if d.CriticalRange != tc.weapon || d.SpellCriticalRange != tc.spell {
				t.Errorf("weapon %d, spell %d; want %d and %d", d.CriticalRange, d.SpellCriticalRange, tc.weapon, tc.spell)
			}
		})
	}
	// Improved Critical (the SRD) lowers the weapon range only.
	b := postureBuild(3)
	b.Classes[0].Subclass = "subclass:champion"
	if d := Derive(b, loadForTest(t)); d.CriticalRange != 19 || d.SpellCriticalRange != 20 {
		t.Errorf("champion 3: weapon %d, spell %d; want 19 and 20", d.CriticalRange, d.SpellCriticalRange)
	}
}
