package rules

import "testing"

func monkBuild(level, str, dex int) Build {
	b := standard("class:monk", level)
	b.BaseScores = map[Ability]int{STR: str, DEX: dex, CON: 13, INT: 12, WIS: 10, CHA: 8}
	b.Weapons = []string{"equipment:dagger", "equipment:quarterstaff"}
	return b
}

// Martial Arts (the monk die, DEX on monk weapons) holds only while the monk
// wears no armor and no shield.
func TestMartialArtsNeedsNoArmorOrShield(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	for _, tc := range []struct {
		name   string
		armor  string
		shield bool
	}{{"leather armor", "equipment:leather-armor", false}, {"shield", "", true}} {
		b := monkBuild(5, 9, 18)
		b.Armor, b.Shield = tc.armor, tc.shield
		d := Derive(b, c)
		dagger, _ := attackOf(d, "equipment:dagger")
		if dagger.Damage != "1d4+4" {
			t.Errorf("%s: dagger damage = %q, want 1d4+4 (finesse with DEX, the weapon's own die)", tc.name, dagger.Damage)
		}
		staff, _ := attackOf(d, "equipment:quarterstaff")
		if staff.Ability != STR {
			t.Errorf("%s: quarterstaff ability = %s, want STR (DEX only while unarmored)", tc.name, staff.Ability)
		}
		unarmed, _ := attackOf(d, unarmedStrikeKey)
		if unarmed.Ability != STR || unarmed.Damage != "1" {
			t.Errorf("%s: unarmed strike = %s %q, want STR and 1 (no monk die with armor)", tc.name, unarmed.Ability, unarmed.Damage)
		}
	}
	d := Derive(monkBuild(5, 9, 18), c)
	if dagger, _ := attackOf(d, "equipment:dagger"); dagger.Damage != "1d6+4" {
		t.Errorf("unarmored dagger damage = %q, want 1d6+4 (monk die)", dagger.Damage)
	}
}

// The monk die replaces the weapon's die on the two-handed line too.
func TestMonkDieAppliesToVersatileDamage(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	for _, tc := range []struct {
		level int
		want  string
	}{{11, "1d8+4"}, {17, "1d10+4"}} {
		a, ok := attackOf(Derive(monkBuild(tc.level, 9, 18), c), "equipment:quarterstaff")
		if !ok {
			t.Fatal("no quarterstaff attack")
		}
		if a.Damage != tc.want || a.VersatileDamage != tc.want {
			t.Errorf("monk %d quarterstaff Damage=%q Versatile=%q, want both %s", tc.level, a.Damage, a.VersatileDamage, tc.want)
		}
	}
}

// Everyone has an unarmed strike: proficient, 1 + STR bludgeoning; a monk
// rolls the Martial Arts die and takes the better of DEX and STR.
func TestUnarmedStrike(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	for _, tc := range []struct {
		name    string
		build   Build
		ability Ability
		bonus   int
		damage  string
	}{
		{"monk 1 DEX 16", Build{
			BaseScores: map[Ability]int{STR: 10, DEX: 16, CON: 10, INT: 10, WIS: 10, CHA: 10},
			Race:       "race:human", Classes: []ClassLevel{{Class: "class:monk", Level: 1}},
		}, DEX, 5, "1d4+3"},
		{"wizard 1 STR 10", Build{
			BaseScores: map[Ability]int{STR: 10, DEX: 10, CON: 10, INT: 10, WIS: 10, CHA: 10},
			Race:       "race:human", Classes: []ClassLevel{{Class: "class:wizard", Level: 1}},
		}, STR, 2, "1"},
		{"wizard 1 STR 16", Build{
			BaseScores: map[Ability]int{STR: 15, DEX: 18, CON: 10, INT: 10, WIS: 10, CHA: 10},
			Race:       "race:human", Classes: []ClassLevel{{Class: "class:wizard", Level: 1}},
		}, STR, 5, "4"},
	} {
		a, ok := attackOf(Derive(tc.build, c), unarmedStrikeKey)
		if !ok {
			t.Errorf("%s: no unarmed strike line", tc.name)
			continue
		}
		if a.Ability != tc.ability || a.AttackBonus != tc.bonus || a.Damage != tc.damage || a.DamageType != "damage-type:bludgeoning" || !a.Melee || !a.Proficient {
			t.Errorf("%s: got %s %+d %q %s melee=%v proficient=%v, want %s %+d %q bludgeoning melee proficient",
				tc.name, a.Ability, a.AttackBonus, a.Damage, a.DamageType, a.Melee, a.Proficient, tc.ability, tc.bonus, tc.damage)
		}
	}
}

// A finesse weapon takes the better of STR and DEX whether it is melee or
// thrown (a dart).
func TestFinesseWeaponTakesBetterAbilityWhenRanged(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	b := standard("class:fighter", 1)
	b.BaseScores = map[Ability]int{STR: 18, DEX: 10, CON: 13, INT: 12, WIS: 10, CHA: 8}
	b.Weapons = []string{"equipment:dart"}
	a, ok := attackOf(Derive(b, c), "equipment:dart")
	if !ok {
		t.Fatal("no dart attack")
	}
	str := abilityOf(Derive(b, c), STR).Modifier
	if a.Ability != STR || a.AttackBonus != str+2 || a.Damage != withModifier("1d4", str) {
		t.Errorf("dart = %s %+d %q, want STR %+d and 1d4%+d", a.Ability, a.AttackBonus, a.Damage, str+2, str)
	}
}

// A Small creature has disadvantage with a heavy weapon, and the sheet says
// so.
func TestHeavyWeaponHintForSmallRace(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	b := standard("class:fighter", 1)
	b.Weapons = []string{"equipment:greatsword"}
	hasHint := func(d Derived) bool {
		for _, h := range d.Hints {
			if h.Source == "equipment:greatsword" && h.Mode == "disadvantage" {
				return true
			}
		}
		return false
	}
	if hasHint(Derive(b, c)) {
		t.Error("a Medium character got the heavy weapon hint")
	}
	b.Race, b.Subrace = "race:halfling", "subrace:lightfoot-halfling"
	if d := Derive(b, c); !hasHint(d) {
		t.Errorf("hints = %+v, want a disadvantage hint for the greatsword", d.Hints)
	}
}

// Eldritch Blast shows its beams: 1, then 2, 3 and 4 at warlock levels 5, 11
// and 17.
func TestEldritchBlastBeamCount(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	for lv, want := range map[int]int{1: 1, 4: 1, 5: 2, 10: 2, 11: 3, 16: 3, 17: 4, 20: 4} {
		b := standard("class:warlock", lv)
		b.Cantrips = []string{"spell:eldritch-blast"}
		a, ok := attackOf(Derive(b, c), "spell:eldritch-blast")
		if !ok {
			t.Fatalf("level %d: no eldritch blast attack", lv)
		}
		if a.Beams != want {
			t.Errorf("level %d: Beams = %d, want %d", lv, a.Beams, want)
		}
	}
}

// Heavy armor whose Strength requirement is not met costs 10 ft of speed;
// dwarves keep theirs.
func TestHeavyArmorStrengthRequirementCutsSpeed(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	for _, tc := range []struct {
		name, race, subrace, armor string
		str, bonus, want           int
	}{
		{"human STR 10 chain mail (needs 13)", "race:human", "", "equipment:chain-mail", 10, 1, 20},
		{"human STR 13 chain mail", "race:human", "", "equipment:chain-mail", 13, 1, 30},
		{"human STR 14 splint (needs 15)", "race:human", "", "equipment:splint-armor", 14, 1, 20},
		{"human STR 15 splint", "race:human", "", "equipment:splint-armor", 15, 1, 30},
		{"human STR 14 plate (needs 15)", "race:human", "", "equipment:plate-armor", 14, 1, 20},
		{"human STR 15 plate", "race:human", "", "equipment:plate-armor", 15, 1, 30},
		{"human STR 8 leather", "race:human", "", "equipment:leather-armor", 8, 1, 30},
		{"hill dwarf STR 8 chain mail", "race:dwarf", "subrace:hill-dwarf", "equipment:chain-mail", 8, 0, 25},
	} {
		b := standard("class:fighter", 1)
		b.Race, b.Subrace = tc.race, tc.subrace
		b.BaseScores[STR] = tc.str - tc.bonus
		b.Armor = tc.armor
		d := Derive(b, c)
		if got := abilityOf(d, STR).Score; got != tc.str {
			t.Fatalf("%s: setup STR = %d, want %d", tc.name, got, tc.str)
		}
		if d.SpeedWalkFt != tc.want {
			t.Errorf("%s: speed = %d, want %d", tc.name, d.SpeedWalkFt, tc.want)
		}
	}
}

// A feature offers a fixed number of options, and an option name counts once
// even when two classes offer it.
func TestFightingStyleCountAndRepeats(t *testing.T) {
	c := loadForTest(t)
	t.Run("the same style from two classes does not stack", func(t *testing.T) {
		b := standard("class:fighter", 2)
		b.Classes = []ClassLevel{{Class: "class:fighter", Level: 2}, {Class: "class:paladin", Level: 2}}
		b.BaseScores[CHA] = 13
		b.Armor = "equipment:chain-mail"
		b.FeatureChoices = []string{"feature:fighter-fighting-style-defense", "feature:fighting-style-defense"}
		d := Derive(b, c)
		if d.ArmorClass != 17 {
			t.Errorf("AC = %d, want 17 (chain mail 16 + Defense once)", d.ArmorClass)
		}
		if !hasIssue(d, IssueChoiceCount) {
			t.Errorf("issues = %v, want a choice_count issue for the repeated style", d.Issues)
		}
	})
	t.Run("two styles for one feature raise an issue", func(t *testing.T) {
		b := standard("class:fighter", 1)
		b.FeatureChoices = []string{"feature:fighter-fighting-style-archery"}
		if hasIssue(Derive(b, c), IssueChoiceCount) {
			t.Fatal("one style raised an issue")
		}
		b.FeatureChoices = append(b.FeatureChoices, "feature:fighter-fighting-style-defense")
		if !hasIssue(Derive(b, c), IssueChoiceCount) {
			t.Error("two styles at Fighter 1: want a choice_count issue")
		}
	})
	t.Run("metamagic and pact boon are counted too", func(t *testing.T) {
		b := standard("class:sorcerer", 3)
		b.FeatureChoices = []string{"feature:metamagic-careful-spell", "feature:metamagic-distant-spell"}
		if hasIssue(Derive(b, c), IssueChoiceCount) {
			t.Fatal("two metamagic options at Sorcerer 3 raised an issue")
		}
		b.FeatureChoices = append(b.FeatureChoices, "feature:metamagic-empowered-spell")
		if !hasIssue(Derive(b, c), IssueChoiceCount) {
			t.Error("three metamagic options at Sorcerer 3: want a choice_count issue")
		}
		w := standard("class:warlock", 3)
		w.FeatureChoices = []string{"feature:pact-of-the-chain", "feature:pact-of-the-blade"}
		if !hasIssue(Derive(w, c), IssueChoiceCount) {
			t.Error("two pact boons: want a choice_count issue")
		}
	})
}
