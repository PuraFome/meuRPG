package rules

import (
	"slices"
	"testing"
)

// salvia is the Etapa 9 designs' druid: Half-elf, Druid 5 (Circle of the Land),
// darkvision 60, Wisdom 16.
func salvia(level int) Build {
	return Build{
		BaseScores: map[Ability]int{STR: 10, DEX: 14, CON: 14, INT: 10, WIS: 16, CHA: 8},
		Race:       "race:half-elf", Background: "background:acolyte",
		Classes:             []ClassLevel{{Class: "class:druid", Level: level}},
		SkillProficiencies:  []string{"skill:perception", "skill:nature"},
		ExtraAbilityBonuses: map[Ability]int{CON: 1, DEX: 1},
	}
}

// TestWildShapeForms: the beasts by druid level, with the real filters.
func TestWildShapeForms(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	for level, want := range map[int]int{2: 31, 3: 31, 4: 48, 7: 48, 8: 70, 20: 70} {
		if got := len(c.WildShapeForms(salvia(level))); got != want {
			t.Errorf("druid %d: %d beasts, want %d", level, got, want)
		}
	}
	if got := c.WildShapeForms(salvia(1)); got != nil {
		t.Errorf("a level 1 druid has no Wild Shape: %v", got)
	}
	if got := c.WildShapeForms(pensantus()); got != nil {
		t.Errorf("a wizard has no Wild Shape: %v", got)
	}
	has := func(level int, key string) bool {
		return slices.ContainsFunc(c.WildShapeForms(salvia(level)), func(e CreatureEntry) bool { return e.Key == key })
	}
	for _, tt := range []struct {
		level int
		key   string
		want  bool
	}{
		{2, "monster:wolf", true},
		{2, "monster:dire-wolf", false},
		{2, "monster:raven", false},
		{2, "monster:crab", false},
		{2, "monster:cat", true},
		{2, "monster:goblin", false},
		{4, "monster:crocodile", true},
		{4, "monster:giant-eagle", false},
		{4, "monster:dire-wolf", false},
		{4, "monster:crab", true},
		{8, "monster:dire-wolf", true},
		{8, "monster:giant-eagle", true},
		{8, "monster:raven", true},
		{8, "monster:giant-crocodile", false},
	} {
		if has(tt.level, tt.key) != tt.want {
			t.Errorf("druid %d may take %s: %v, want %v", tt.level, tt.key, !tt.want, tt.want)
		}
	}
	for _, e := range c.WildShapeForms(salvia(8)) {
		if e.Type != "beast" {
			t.Errorf("%s is not a beast", e.Key)
		}
	}
	if !c.WildShapeAllows(salvia(5), "monster:wolf") || c.WildShapeAllows(salvia(5), "monster:dire-wolf") {
		t.Error("WildShapeAllows disagrees with the list")
	}
	if lim, ok := c.WildShapeLimitFor(salvia(5)); !ok || lim.MaxCR != "1/2" || !lim.NoFly || lim.NoSwim {
		t.Errorf("limit at level 5 = %+v, want CR 1/2, no flying", lim)
	}
}

// TestWildShapeDerived: Sálvia as a wolf.
func TestWildShapeDerived(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	sal := Derive(salvia(5), c)
	if got := len(sal.Senses); got != 1 || sal.Senses[0].Key != "darkvision" || sal.Senses[0].RangeFt != 60 {
		t.Fatalf("Sálvia sees in the dark to 60 ft: %+v", sal.Senses)
	}
	wolf, err := c.WildShapeDerived(sal, "monster:wolf")
	if err != nil {
		t.Fatal(err)
	}
	if wolf.ArmorClass != 13 || wolf.HitPointsMax != 11 || wolf.SpeedWalkFt != 40 || wolf.SpeedFlyFt != 0 || wolf.SpeedSwimFt != 0 {
		t.Errorf("wolf AC %d, HP %d, speeds %d/%d/%d; want 13, 11, 40", wolf.ArmorClass, wolf.HitPointsMax, wolf.SpeedWalkFt, wolf.SpeedFlyFt, wolf.SpeedSwimFt)
	}
	for ab, want := range map[Ability]int{STR: 12, DEX: 15, CON: 12} {
		if got := abilityOf(wolf, ab); got.Score != want || got.Modifier != modifier(want) {
			t.Errorf("%s as a wolf = %+v, want the wolf's %d", ab, got, want)
		}
	}
	for _, ab := range []Ability{INT, WIS, CHA} {
		if abilityOf(wolf, ab) != abilityOf(sal, ab) {
			t.Errorf("%s changed in Wild Shape: %+v, was %+v", ab, abilityOf(wolf, ab), abilityOf(sal, ab))
		}
	}
	if len(wolf.HitDice) != 1 || wolf.HitDice[0] != (HitDice{Die: 8, Count: 2}) {
		t.Errorf("hit dice as a wolf = %+v, want the wolf's 2d8", wolf.HitDice)
	}
	if len(wolf.Senses) != 0 {
		t.Errorf("a wolf has no darkvision, so neither does Sálvia in its shape: %+v", wolf.Senses)
	}
	if len(wolf.Attacks) != 1 || wolf.Attacks[0].Name != "Bite" || wolf.Attacks[0].AttackBonus != 4 || wolf.Attacks[0].Damage != "2d4+2" {
		t.Errorf("attacks = %+v, want the wolf's bite +4 2d4+2", wolf.Attacks)
	}
	if len(wolf.Spellcasting) != 0 || len(wolf.Spells) != 0 || wolf.PactMagic != nil {
		t.Error("a druid in beast form casts no spells")
	}
	if wolf.ProficiencyBonus != sal.ProficiencyBonus || wolf.TotalLevel != 5 || len(wolf.Classes) != 1 {
		t.Errorf("the character is still a level 5 druid: %+v", wolf.Classes)
	}
	if wolf.Hints[len(wolf.Hints)-1].Source != "wild_shape" || len(wolf.Hints) != len(sal.Hints)+1 {
		t.Error("the form adds a hint about spells")
	}
	if len(sal.Attacks) != len(Derive(salvia(5), c).Attacks) || len(sal.Hints) != len(Derive(salvia(5), c).Hints) {
		t.Error("WildShapeDerived changed the character's own Derived")
	}

	t.Run("a beast with darkvision lets the character's range count", func(t *testing.T) {
		t.Parallel()
		// A giant spider sees to 60 ft, and so does Sálvia: the larger range wins.
		sp, err := c.WildShapeDerived(sal, "monster:giant-spider")
		if err != nil {
			t.Fatal(err)
		}
		dv := map[string]int{}
		for _, s := range sp.Senses {
			dv[s.Key] = s.RangeFt
		}
		if dv["darkvision"] != 60 || dv["blindsight"] != 10 {
			t.Errorf("giant spider senses = %v, want darkvision 60 and blindsight 10", dv)
		}
		// A character with a bigger range keeps it in a beast that has some.
		big := sal
		big.Senses = []Sense{{Key: "darkvision", RangeFt: 120}}
		sp, _ = c.WildShapeDerived(big, "monster:giant-spider")
		if sp.Senses[0].Key != "darkvision" || sp.Senses[0].RangeFt != 120 {
			t.Errorf("senses = %+v, want the character's 120", sp.Senses)
		}
	})

	t.Run("the higher bonus wins where both are proficient", func(t *testing.T) {
		t.Parallel()
		// Sálvia is proficient in Perception (+3 Wis, +3 proficiency = +6): the
		// wolf's listed +3 is lower, hers stays. In Stealth she is not proficient
		// but the wolf is: +4 from the stat block.
		if got := skillOf(wolf, "skill:perception"); got.Bonus != 6 || got.Proficiency != ProficiencyFull {
			t.Errorf("perception as a wolf = %+v, want her +6", got)
		}
		if got := skillOf(wolf, "skill:stealth"); got.Bonus != 4 || got.Proficiency != ProficiencyFull {
			t.Errorf("stealth as a wolf = %+v, want the wolf's +4", got)
		}
		// Raise her Perception above the wolf's listed bonus and lower the wolf's:
		// a stat block bonus higher than hers wins. Use a beast whose Perception is
		// listed higher than +6: none of the SRD beasts, so check the rule on a
		// copy of the content with one.
		c2 := loadForTest(t)
		c2.c.monsters["monster:wolf"].Skills = map[string]int{"skill:perception": 9, "skill:stealth": 4}
		w2, _ := c2.WildShapeDerived(sal, "monster:wolf")
		if got := skillOf(w2, "skill:perception"); got.Bonus != 9 {
			t.Errorf("perception with the wolf's +9 = %+v, want 9 (the higher one)", got)
		}
		if w2.PassivePerception != 19 {
			t.Errorf("passive Perception = %d, want 19", w2.PassivePerception)
		}
	})

	t.Run("saves and the physical scores", func(t *testing.T) {
		t.Parallel()
		// Sálvia's druid saves are Int and Wis; the wolf lists none, so the Dex and
		// Con saves follow the wolf's scores, and Wis is hers.
		if got := saveOf(wolf, WIS); got != saveOf(sal, WIS) {
			t.Errorf("Wis save = %+v, was %+v", got, saveOf(sal, WIS))
		}
		if got := saveOf(wolf, CON); got.Bonus != 1 || got.Proficient {
			t.Errorf("Con save = %+v, want +1 (the wolf's Con 12)", got)
		}
	})

	if _, err := c.WildShapeDerived(sal, "monster:goblin"); err == nil {
		t.Error("a goblin is not a beast")
	}
	if _, err := c.WildShapeDerived(sal, "monster:nope"); err == nil {
		t.Error("an unknown creature is not a form")
	}
}

// TestWildShapeEffectIsClosed: the loader refuses a wild_shape effect without a
// challenge rating.
func TestWildShapeEffectIsClosed(t *testing.T) {
	t.Parallel()
	c := loadForTest(t).c
	for name, e := range map[string]*Effect{
		"no rating":      {Type: "wild_shape"},
		"a bad rating":   {Type: "wild_shape", MaxCR: "1/3"},
		"a rating of 31": {Type: "wild_shape", MaxCR: "31"},
	} {
		if err := c.compileEffect("feature:x", e); err == nil {
			t.Errorf("%s: the loader accepted it", name)
		}
	}
	if err := c.compileEffect("feature:x", &Effect{Type: "wild_shape", MaxCR: "1/4", NoFly: true}); err != nil {
		t.Errorf("a good effect: %v", err)
	}
	for name, e := range map[string]*Effect{
		"max_cr on a modifier":      {Type: "note", MaxCR: "1"},
		"no_fly on a resource":      {Type: "resource", Resource: "x", Max: "1", Recharge: "none", NoFly: true},
		"no_swim on a sense":        {Type: "sense", Sense: "darkvision", RangeFt: 60, NoSwim: true},
		"wild_shape with a target":  {Type: "wild_shape", MaxCR: "1", Target: "ac"},
		"wild_shape with a count":   {Type: "wild_shape", MaxCR: "1", Count: 2},
		"wild_shape with a handler": {Type: "wild_shape", MaxCR: "1", Handler: "monk.martial_arts"},
		"wild_shape with text":      {Type: "wild_shape", MaxCR: "1", TextPT: "x"},
	} {
		if err := c.compileEffect("feature:x", e); err == nil {
			t.Errorf("%s: the loader accepted it", name)
		}
	}
	// The druid file's three rows.
	for key, cr := range map[string]string{
		"feature:wild-shape-cr-1-4-or-below-no-flying-or-swim-speed": "1/4",
		"feature:wild-shape-cr-1-2-or-below-no-flying-speed":         "1/2",
		"feature:wild-shape-cr-1-or-below":                           "1",
	} {
		found := false
		for _, e := range c.effects[key] {
			found = found || (e.Type == "wild_shape" && e.MaxCR == cr)
		}
		if !found {
			t.Errorf("%s has no wild_shape effect of CR %s", key, cr)
		}
	}
}
