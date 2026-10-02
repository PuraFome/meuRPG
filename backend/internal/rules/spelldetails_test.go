package rules

import (
	"slices"
	"testing"
)

func TestParseDice(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   string
		want DiceFormula
		ok   bool
	}{
		{"1d10", DiceFormula{Count: 1, Sides: 10}, true},
		{"1d8+3", DiceFormula{Count: 1, Sides: 8, Bonus: 3}, true},
		{"1d6-1", DiceFormula{Count: 1, Sides: 6, Bonus: -1}, true},
		{"7d8 + 30", DiceFormula{Count: 7, Sides: 8, Bonus: 30}, true},
		{"1d8 + MOD", DiceFormula{Count: 1, Sides: 8, AddsModifier: true}, true},
		{"20", DiceFormula{Bonus: 20}, true},
		{"4d6 OR 5d6", DiceFormula{}, false},
		{"", DiceFormula{}, false},
		{"d6", DiceFormula{}, false},
		{"0d6", DiceFormula{}, false},
		{"20 + MOD", DiceFormula{}, false},
	}
	for _, tt := range tests {
		got, ok := ParseDice(tt.in)
		if ok != tt.ok || got != tt.want {
			t.Errorf("ParseDice(%q) = %+v, %v; want %+v, %v", tt.in, got, ok, tt.want, tt.ok)
		}
	}
}

func TestParseSpellStrings(t *testing.T) {
	t.Parallel()
	ct := []struct {
		in   string
		want CastingTime
	}{
		{"1 action", CastingTime{Amount: 1, Unit: CastAction, Raw: "1 action"}},
		{"1 bonus action", CastingTime{Amount: 1, Unit: CastBonusAction, Raw: "1 bonus action"}},
		{"1 reaction", CastingTime{Amount: 1, Unit: CastReaction, Raw: "1 reaction"}},
		{"10 minutes", CastingTime{Amount: 10, Unit: CastMinute, Raw: "10 minutes"}},
		{"1 hour", CastingTime{Amount: 1, Unit: CastHour, Raw: "1 hour"}},
		{"1 reaction, which you take when you are hit", CastingTime{Amount: 1, Unit: CastReaction, Trigger: "which you take when you are hit", Raw: "1 reaction, which you take when you are hit"}},
	}
	for _, tt := range ct {
		if got := parseCastingTime(tt.in); got != tt.want {
			t.Errorf("parseCastingTime(%q) = %+v, want %+v", tt.in, got, tt.want)
		}
	}
	rg := []struct {
		in   string
		kind string
		ft   int
	}{
		{"Self", RangeSelf, 0},
		{"Touch", RangeTouch, 0},
		{"60 feet", RangeRanged, 60},
		{"1 mile", RangeRanged, 5280},
		{"500 miles", RangeRanged, 2640000},
		{"Sight", RangeSight, 0},
		{"Unlimited", RangeUnlimited, 0},
		{"Special", RangeSpecial, 0},
	}
	for _, tt := range rg {
		if got := parseRange(tt.in); got.Kind != tt.kind || got.DistanceFt != tt.ft || got.Raw != tt.in {
			t.Errorf("parseRange(%q) = %+v, want %s %d ft", tt.in, got, tt.kind, tt.ft)
		}
	}
	du := []struct {
		in   string
		conc bool
		want SpellDuration
	}{
		{"Instantaneous", false, SpellDuration{Kind: DurationInstantaneous, Raw: "Instantaneous"}},
		{"Up to 1 minute", true, SpellDuration{Kind: DurationTimed, Amount: 1, Unit: DurationMinute, UpTo: true, Concentration: true, Raw: "Up to 1 minute"}},
		{"8 hours", false, SpellDuration{Kind: DurationTimed, Amount: 8, Unit: DurationHour, Raw: "8 hours"}},
		{"1 round", false, SpellDuration{Kind: DurationTimed, Amount: 1, Unit: DurationRound, Raw: "1 round"}},
		{"10 days", false, SpellDuration{Kind: DurationTimed, Amount: 10, Unit: DurationDay, Raw: "10 days"}},
		{"Until dispelled", false, SpellDuration{Kind: DurationUntilDispelled, Raw: "Until dispelled"}},
		{"Special", false, SpellDuration{Kind: DurationSpecial, Raw: "Special"}},
	}
	for _, tt := range du {
		if got := parseDuration(tt.in, tt.conc); got != tt.want {
			t.Errorf("parseDuration(%q) = %+v, want %+v", tt.in, got, tt.want)
		}
	}
}

// TestSpellDetailsCoverTheCatalog: every SRD spell has structured details,
// and the SRD strings that do not parse are only the ones the SRD itself
// calls "Special".
func TestSpellDetailsCoverTheCatalog(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	for _, e := range c.Catalog().Spells {
		d, ok := c.SpellDetails(e.Key)
		if !ok {
			t.Errorf("%s has no details", e.Key)
			continue
		}
		if d.CastingTime.Unit == "" || d.CastingTime.Amount < 1 {
			t.Errorf("%s: casting time %q did not parse", e.Key, d.CastingTime.Raw)
		}
		if d.Range.Kind == "" {
			t.Errorf("%s: range %q did not parse", e.Key, d.Range.Raw)
		}
		if d.Duration.Kind == DurationSpecial && d.Duration.Raw != "Special" {
			t.Errorf("%s: duration %q did not parse", e.Key, d.Duration.Raw)
		}
		if d.Range.Kind == RangeSpecial && d.Range.Raw != "Special" {
			t.Errorf("%s: range %q did not parse", e.Key, d.Range.Raw)
		}
		if d.Components.Material != (d.Components.MaterialText != "") {
			t.Errorf("%s: material flag %v with text %q", e.Key, d.Components.Material, d.Components.MaterialText)
		}
		if len(d.Description) == 0 {
			t.Errorf("%s has no description", e.Key)
		}
		if d.Spell.CastingTime != d.CastingTime {
			t.Errorf("%s: the catalog entry and the details disagree on the casting time", e.Key)
		}
	}
	if _, ok := c.SpellDetails("spell:nope"); ok {
		t.Error("SpellDetails(unknown) = ok")
	}
}

func TestSpellDetailsExamples(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	get := func(key string) *SpellDetails {
		t.Helper()
		d, ok := c.SpellDetails(key)
		if !ok {
			t.Fatalf("no details for %s", key)
		}
		return d
	}

	fb := get("spell:fire-bolt")
	if fb.AttackType != "ranged" || fb.Range.DistanceFt != 120 || fb.Save != nil || !fb.Components.Verbal || !fb.Components.Somatic || fb.Components.Material {
		t.Errorf("fire bolt = %+v", fb)
	}
	// A cantrip grows with the character's level.
	for level, want := range map[int]string{1: "1d10", 4: "1d10", 5: "2d10", 11: "3d10", 20: "4d10"} {
		if r := fb.DamageAt(0, level); len(r) != 1 || r[0].Raw != want || r[0].Type != "damage-type:fire" {
			t.Errorf("fire bolt at level %d = %+v, want %s fire", level, r, want)
		}
	}

	sf := get("spell:sacred-flame")
	if sf.Save == nil || sf.Save.Ability != DEX || sf.Save.OnSuccess != "none" {
		t.Errorf("sacred flame save = %+v, want DEX, none", sf.Save)
	}
	if fbl := get("spell:fireball"); fbl.Save == nil || fbl.Save.OnSuccess != "half" || fbl.Duration.Kind != DurationInstantaneous || fbl.Range.DistanceFt != 150 {
		t.Errorf("fireball = %+v", fbl)
	}

	mm := get("spell:magic-missile")
	r := mm.DamageAt(1, 3)
	if len(r) != 1 || !r[0].Parsed || r[0].Dice != (DiceFormula{Count: 3, Sides: 4, Bonus: 3}) {
		t.Errorf("magic missile at 1st = %+v, want 3d4+3", r)
	}
	if r := mm.DamageAt(2, 3); r[0].Dice.Count != 4 {
		t.Errorf("magic missile at 2nd = %+v, want 4d4+4", r)
	}

	// Healing by slot level, with the spellcasting modifier.
	cw := get("spell:cure-wounds")
	if h, ok := cw.HealAt(2); !ok || h.Dice != (DiceFormula{Count: 2, Sides: 8, AddsModifier: true}) || len(cw.DamageAt(2, 3)) != 0 {
		t.Errorf("cure wounds at 2nd = %+v, %v", h, ok)
	}
	if _, ok := fb.HealAt(1); ok {
		t.Error("fire bolt heals")
	}

	sh := get("spell:shield")
	if sh.CastingTime.Unit != CastReaction || sh.Duration.Raw != "1 round" || sh.Spell.Level != 1 {
		t.Errorf("shield = %+v", sh)
	}
	if dm := get("spell:detect-magic"); !dm.Spell.Ritual || !dm.Duration.Concentration || !dm.Duration.UpTo || dm.Duration.Amount != 10 {
		t.Errorf("detect magic = %+v", dm.Duration)
	}
	// The level 6 Flame Strike entry is "4d6 OR 5d6": kept as text.
	if fs := get("spell:flame-strike").DamageAt(6, 11); len(fs) != 2 || fs[0].Parsed || fs[0].Raw != "4d6 OR 5d6" {
		t.Errorf("flame strike at 6th = %+v", fs)
	}
	if mt := get("spell:acid-arrow"); mt.Components.MaterialText == "" || !slices.Contains(mt.Spell.Classes, "class:wizard") || len(mt.HigherLevel) != 1 {
		t.Errorf("acid arrow = %+v", mt)
	}
}
