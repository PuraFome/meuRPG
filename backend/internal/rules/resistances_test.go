package rules

import (
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

// Damage resistances of features and traits (SRD 5.1, "Damage Resistance and
// Vulnerability"): effects/damage_resistances.json and what Content.Resistances
// reads from a derived sheet. These tests need no database.

func resistanceOf(rs []Resistance, source string) (Resistance, bool) {
	i := slices.IndexFunc(rs, func(r Resistance) bool { return r.Source == source })
	if i < 0 {
		return Resistance{}, false
	}
	return rs[i], true
}

func TestRacialAndRageResistancesComeFromTheSheet(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	scores := map[Ability]int{STR: 12, DEX: 12, CON: 12, INT: 12, WIS: 12, CHA: 12}
	tests := []struct {
		name  string
		build Build
		key   string
		types []string
		while string
	}{
		{"tiefling: fire", Build{BaseScores: scores, Race: "race:tiefling", Background: "background:acolyte", Classes: []ClassLevel{{Class: "class:fighter", Level: 1}}},
			"trait:hellish-resistance", []string{"damage-type:fire"}, ResistanceAlways},
		{"hill dwarf: poison", Build{BaseScores: scores, Race: "race:dwarf", Subrace: "subrace:hill-dwarf", Background: "background:acolyte", Classes: []ClassLevel{{Class: "class:fighter", Level: 1}}},
			"trait:dwarven-resilience", []string{"damage-type:poison"}, ResistanceAlways},
		{"red dragonborn: fire", Build{BaseScores: scores, Race: "race:dragonborn", Background: "background:acolyte", FeatureChoices: []string{"trait:draconic-ancestry-red"}, Classes: []ClassLevel{{Class: "class:fighter", Level: 1}}},
			"trait:draconic-ancestry-red", []string{"damage-type:fire"}, ResistanceAlways},
		{"silver dragonborn: cold", Build{BaseScores: scores, Race: "race:dragonborn", Background: "background:acolyte", FeatureChoices: []string{"trait:draconic-ancestry-silver"}, Classes: []ClassLevel{{Class: "class:fighter", Level: 1}}},
			"trait:draconic-ancestry-silver", []string{"damage-type:cold"}, ResistanceAlways},
		{"barbarian: rage", Build{BaseScores: scores, Race: "race:human", Background: "background:acolyte", Classes: []ClassLevel{{Class: "class:barbarian", Level: 1}}},
			"feature:rage", []string{"damage-type:bludgeoning", "damage-type:piercing", "damage-type:slashing"}, ResistanceWhileRage},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rs := c.Resistances(Derive(tt.build, c))
			r, ok := resistanceOf(rs, tt.key)
			if !ok {
				t.Fatalf("no resistance from %s in %+v", tt.key, rs)
			}
			if !slices.Equal(r.DamageTypes, tt.types) || r.While != tt.while || r.NamePT == "" {
				t.Fatalf("resistance = %+v", r)
			}
		})
	}
	// A human fighter has none, and a dragonborn only the lineage chosen.
	human := Build{BaseScores: scores, Race: "race:human", Background: "background:acolyte", Classes: []ClassLevel{{Class: "class:fighter", Level: 1}}}
	if rs := c.Resistances(Derive(human, c)); len(rs) != 0 {
		t.Fatalf("a human fighter resists nothing: %+v", rs)
	}
	dragon := Build{BaseScores: scores, Race: "race:dragonborn", Background: "background:acolyte", FeatureChoices: []string{"trait:draconic-ancestry-green"}, Classes: []ClassLevel{{Class: "class:fighter", Level: 1}}}
	rs := c.Resistances(Derive(dragon, c))
	if len(rs) != 1 || rs[0].Source != "trait:draconic-ancestry-green" || rs[0].DamageTypes[0] != "damage-type:poison" {
		t.Fatalf("a green dragonborn resists poison alone: %+v", rs)
	}
}

func TestEveryDraconicLineageHasItsDamageType(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	// SRD 5.1, Dragonborn: the Draconic Ancestry table.
	want := map[string]string{
		"black": "acid", "blue": "lightning", "brass": "fire", "bronze": "lightning", "copper": "acid",
		"gold": "fire", "green": "poison", "red": "fire", "silver": "cold", "white": "cold",
	}
	for colour, typ := range want {
		r, ok := c.c.resistances["trait:draconic-ancestry-"+colour]
		if !ok || !slices.Equal(r.DamageTypes, []string{"damage-type:" + typ}) {
			t.Errorf("%s dragon: %+v", colour, r)
		}
	}
}

func TestDamageResistancesAreChecked(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	load := func(body string) error {
		return c.c.loadDamageResistances(fstest.MapFS{"effects/damage_resistances.json": {Data: []byte(body)}})
	}
	good := `{"source":"SRD 5.1","resistances":{"trait:hellish-resistance":{"damage_types":["damage-type:fire"]}}}`
	if err := load(good); err != nil {
		t.Fatalf("a good file: %v", err)
	}
	for name, body := range map[string]string{
		"no source":              `{"resistances":{"trait:hellish-resistance":{"damage_types":["damage-type:fire"]}}}`,
		"a key that is no trait": `{"source":"x","resistances":{"race:tiefling":{"damage_types":["damage-type:fire"]}}}`,
		"an unknown trait":       `{"source":"x","resistances":{"trait:nope":{"damage_types":["damage-type:fire"]}}}`,
		"an unknown damage type": `{"source":"x","resistances":{"trait:hellish-resistance":{"damage_types":["damage-type:love"]}}}`,
		"no damage type":         `{"source":"x","resistances":{"trait:hellish-resistance":{"damage_types":[]}}}`,
		"an unknown while":       `{"source":"x","resistances":{"trait:hellish-resistance":{"damage_types":["damage-type:fire"],"while":"moon"}}}`,
		"an unknown field":       `{"source":"x","resistances":{"trait:hellish-resistance":{"damage_types":["damage-type:fire"],"half":true}}}`,
	} {
		err := load(body)
		if err == nil {
			t.Errorf("%s: accepted", name)
			continue
		}
		if !strings.Contains(err.Error(), "damage_resistances.json") {
			t.Errorf("%s: the error does not name the file: %v", name, err)
		}
	}
}

func TestWeaponAndArmorFactsForTheCombat(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	scores := map[Ability]int{STR: 16, DEX: 14, CON: 12, INT: 10, WIS: 10, CHA: 10}
	d := Derive(Build{
		BaseScores: scores, Race: "race:human", Background: "background:acolyte", Armor: "equipment:plate-armor",
		Classes: []ClassLevel{{Class: "class:fighter", Level: 1}},
		Weapons: []string{"equipment:greataxe", "equipment:shortsword", "equipment:longbow", "equipment:longsword"},
	}, c)
	if d.ArmorCategory != "heavy" {
		t.Fatalf("armor category = %q, want heavy", d.ArmorCategory)
	}
	flags := map[string][2]bool{}
	for _, a := range d.Attacks {
		flags[a.Key] = [2]bool{a.Finesse, a.TwoHanded}
	}
	want := map[string][2]bool{
		"equipment:greataxe": {false, true}, "equipment:shortsword": {true, false},
		"equipment:longbow": {false, true}, "equipment:longsword": {false, false},
	}
	for key, w := range want {
		if flags[key] != w {
			t.Errorf("%s finesse/two-handed = %v, want %v", key, flags[key], w)
		}
	}
	if none := Derive(Build{BaseScores: scores, Race: "race:human", Background: "background:acolyte", Classes: []ClassLevel{{Class: "class:fighter", Level: 1}}}, c); none.ArmorCategory != "none" {
		t.Fatalf("no armor is %q", none.ArmorCategory)
	}
}

func TestLevelInAndHasFeature(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	d := Derive(Build{
		BaseScores: map[Ability]int{STR: 16, DEX: 14, CON: 12, INT: 10, WIS: 10, CHA: 10}, Race: "race:human", Background: "background:acolyte",
		Classes: []ClassLevel{{Class: "class:rogue", Level: 5}},
	}, c)
	if LevelIn(d, "class:rogue") != 5 || LevelIn(d, "class:barbarian") != 0 {
		t.Fatalf("levels: %+v", d.Classes)
	}
	if !HasFeature(d, "feature:sneak-attack") || HasFeature(d, "feature:rage") {
		t.Fatal("a level 5 rogue has Sneak Attack and no Rage")
	}
}
