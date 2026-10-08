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
	// Flame Strike's higher slots parse: each type's table is its dice when it is the one that grows.
	if fs := get("spell:flame-strike").DamageAt(6, 11); len(fs) != 2 || !fs[0].Parsed || fs[0].Raw != "5d6" || !fs[1].Parsed || fs[1].Raw != "5d6" {
		t.Errorf("flame strike at 6th = %+v", fs)
	}
	if mt := get("spell:acid-arrow"); mt.Components.MaterialText == "" || !slices.Contains(mt.Spell.Classes, "class:wizard") || len(mt.HigherLevel) != 1 {
		t.Errorf("acid arrow = %+v", mt)
	}
}

// damageOfType is the roll of a damage type the spell makes at a slot level.
func damageOfType(t *testing.T, c *Content, key, damageType string, slot int) (DamageRoll, bool) {
	t.Helper()
	d, ok := c.SpellDetails(key)
	if !ok {
		t.Fatalf("%s is not in the content", key)
	}
	for _, r := range d.DamageAt(slot, 20) {
		if r.Type == damageType {
			return r, true
		}
	}
	return DamageRoll{}, false
}

// TestSpellDamageGrowsWithTheSlot: the damage of a spell that adds dice for
// each slot level above its own follows the SRD's "At Higher Levels" text.
func TestSpellDamageGrowsWithTheSlot(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	for _, tc := range []struct {
		key, damageType string
		slot            int
		want            string
	}{
		{"spell:disintegrate", "damage-type:force", 6, "10d6 + 40"},
		{"spell:disintegrate", "damage-type:force", 7, "13d6 + 40"},
		{"spell:disintegrate", "damage-type:force", 8, "16d6 + 40"},
		{"spell:disintegrate", "damage-type:force", 9, "19d6 + 40"},
		{"spell:freezing-sphere", "damage-type:cold", 6, "10d6"},
		{"spell:freezing-sphere", "damage-type:cold", 7, "11d6"},
		{"spell:freezing-sphere", "damage-type:cold", 9, "13d6"},
		{"spell:phantasmal-killer", "damage-type:psychic", 4, "4d10"},
		{"spell:phantasmal-killer", "damage-type:psychic", 5, "5d10"},
		{"spell:phantasmal-killer", "damage-type:psychic", 9, "9d10"},
		{"spell:wall-of-fire", "damage-type:fire", 4, "5d8"},
		{"spell:wall-of-fire", "damage-type:fire", 5, "6d8"},
		{"spell:wall-of-fire", "damage-type:fire", 9, "10d8"},
		{"spell:spirit-guardians", "damage-type:radiant", 3, "3d8"},
		{"spell:spirit-guardians", "damage-type:radiant", 5, "5d8"},
		{"spell:spirit-guardians", "damage-type:necrotic", 9, "9d8"},
		{"spell:arcane-hand", "damage-type:force", 5, "4d8"},
		{"spell:arcane-hand", "damage-type:force", 7, "8d8"},
		{"spell:glyph-of-warding", "damage-type:thunder", 3, "5d8"},
		{"spell:glyph-of-warding", "damage-type:fire", 5, "7d8"},
	} {
		got, ok := damageOfType(t, c, tc.key, tc.damageType, tc.slot)
		if !ok || got.Raw != tc.want || !got.Parsed {
			t.Errorf("%s at slot %d, %s = %q (parsed %v, listed %v), want %s", tc.key, tc.slot, tc.damageType, got.Raw, got.Parsed, ok, tc.want)
		}
	}
}

// TestSpellsWithoutTheirDamageInTheSnapshot: spells whose damage the snapshot
// lacks have it from the SRD text, and a spell whose effect comes on later turns
// records no saving throw that a cast would open at once.
func TestSpellsWithoutTheirDamageInTheSnapshot(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	for _, tc := range []struct {
		key, damageType, want string
		slot                  int
	}{
		{"spell:spike-growth", "damage-type:piercing", "2d4", 2},
		{"spell:web", "damage-type:fire", "2d4", 2},
		{"spell:earthquake", "damage-type:bludgeoning", "5d6", 8},
		{"spell:teleport", "damage-type:force", "3d10", 7},
	} {
		got, ok := damageOfType(t, c, tc.key, tc.damageType, tc.slot)
		if !ok || got.Raw != tc.want {
			t.Errorf("%s damage = %q (listed %v), want %s %s", tc.key, got.Raw, ok, tc.want, tc.damageType)
		}
	}
	for _, key := range []string{"spell:spike-growth", "spell:web", "spell:earthquake", "spell:teleport", "spell:glyph-of-warding"} {
		if d, _ := c.SpellDetails(key); d.Save != nil || d.AttackType != "" {
			t.Errorf("%s records a saving throw or attack %+v / %q, which a cast would roll at once", key, d.Save, d.AttackType)
		}
	}
	sg, _ := c.SpellDetails("spell:spirit-guardians")
	if sg.Save == nil || sg.Save.Ability != WIS || sg.Save.OnSuccess != "half" {
		t.Errorf("Spirit Guardians save = %+v, want Wisdom, half on a success", sg.Save)
	}
}

// TestSpellAttacksAndSaves: spells the SRD makes attack rolls or saving throws
// carry them with their damage.
func TestSpellAttacksAndSaves(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	for key, want := range map[string]string{"spell:scorching-ray": "ranged", "spell:flame-blade": "melee", "spell:arcane-hand": "melee"} {
		d, ok := c.SpellDetails(key)
		if !ok || d.AttackType != want {
			t.Errorf("%s attack type = %q, want %q", key, d.AttackType, want)
		}
	}
	sr, _ := c.SpellDetails("spell:scorching-ray")
	if rolls := sr.DamageAt(2, 5); len(rolls) != 1 || rolls[0].Raw != "2d6" || rolls[0].Type != "damage-type:fire" {
		t.Errorf("Scorching Ray damage = %+v, want 2d6 fire for each ray", rolls)
	}
	cl, _ := c.SpellDetails("spell:call-lightning")
	if cl.Save == nil || cl.Save.Ability != DEX || cl.Save.OnSuccess != "half" {
		t.Fatalf("Call Lightning save = %+v, want Dexterity, half on a success", cl.Save)
	}
	if rolls := cl.DamageAt(3, 5); len(rolls) != 1 || rolls[0].Raw != "3d10" {
		t.Errorf("Call Lightning damage = %+v, want 3d10", rolls)
	}
}

// TestFlameStrikeScalesTheChosenType: Flame Strike deals 4d6 fire and 4d6 radiant,
// and each slot level above the 5th adds 1d6 to the fire damage or to the radiant
// damage, the caster's choice (SRD 5.1); the other type stays at 4d6.
func TestFlameStrikeScalesTheChosenType(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	d, ok := c.SpellDetails("spell:flame-strike")
	if !ok {
		t.Fatal("no flame strike")
	}
	if got := d.DamageTypeChoices(); !slices.Equal(got, []string{"damage-type:fire", "damage-type:radiant"}) {
		t.Fatalf("DamageTypeChoices = %v, want fire and radiant", got)
	}
	for _, tc := range []struct {
		slot            int
		pick, fire, rad string
	}{
		{5, "", "4d6", "4d6"},
		{6, "", "5d6", "4d6"}, // no pick takes the first type
		{6, "damage-type:radiant", "4d6", "5d6"},
		{9, "damage-type:fire", "8d6", "4d6"},
		{9, "damage-type:radiant", "4d6", "8d6"},
	} {
		got := map[string]string{}
		for _, r := range d.DamageAtChoosing(tc.slot, 20, tc.pick) {
			if !r.Parsed {
				t.Errorf("slot %d pick %q: %q does not parse", tc.slot, tc.pick, r.Raw)
			}
			got[r.Type] = r.Raw
		}
		if got["damage-type:fire"] != tc.fire || got["damage-type:radiant"] != tc.rad || len(got) != 2 {
			t.Errorf("Flame Strike at slot %d picking %q = %v, want fire %s and radiant %s", tc.slot, tc.pick, got, tc.fire, tc.rad)
		}
	}
}

// TestSpiritGuardiansDealsOneTypeOfTheTwo: radiant or necrotic, the caster's
// alignment decides; a cast deals the chosen one only.
func TestSpiritGuardiansDealsOneTypeOfTheTwo(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	d, _ := c.SpellDetails("spell:spirit-guardians")
	for pick, want := range map[string]string{"": "damage-type:radiant", "damage-type:necrotic": "damage-type:necrotic"} {
		rolls := d.DamageAtChoosing(4, 9, pick)
		if len(rolls) != 1 || rolls[0].Type != want || rolls[0].Raw != "4d8" {
			t.Errorf("Spirit Guardians at slot 4 picking %q = %+v, want 4d8 %s alone", pick, rolls, want)
		}
	}
	if fb, _ := c.SpellDetails("spell:fireball"); fb.DamageTypeChoices() != nil || len(fb.DamageAtChoosing(3, 5, "")) != 1 {
		t.Errorf("Fireball has no damage type to pick")
	}
}
