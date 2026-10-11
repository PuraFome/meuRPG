package combat

import (
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

func strength(score int) rules.Derived {
	mod := (score - 10) / 2
	if score < 10 && score%2 == 1 {
		mod-- // floor, not truncation, for an odd score below 10
	}
	return rules.Derived{Abilities: []rules.AbilityScore{
		{Ability: rules.DEX, Score: 18, Modifier: 4},
		{Ability: rules.STR, Score: score, Modifier: mod},
	}}
}

func TestJumpLimits(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		score int
		want  Jumps
	}{
		// Toren, Força 16 (+3): 16 ft running, 8 standing; 6 ft high running, 3 standing.
		{"Toren", 16, Jumps{LongRunning: 160, LongStanding: 80, HighRunning: 60, HighStanding: 30}},
		// Brisa, Força 10 (+0): 10 and 5 ft; 3 and 1,5 ft.
		{"Brisa", 10, Jumps{LongRunning: 100, LongStanding: 50, HighRunning: 30, HighStanding: 15}},
		// Força 6 (-2): the high jump is 1 ft.
		{"Força 6", 6, Jumps{LongRunning: 60, LongStanding: 30, HighRunning: 10, HighStanding: 5}},
		// Força 1 (-5): 3 - 5 is below 0, and a high jump is never negative.
		{"Força 1", 1, Jumps{LongRunning: 10, LongStanding: 5, HighRunning: 0, HighStanding: 0}},
	}
	for _, c := range cases {
		if got := JumpLimits(strength(c.score)); got != c.want {
			t.Errorf("%s: JumpLimits = %+v, want %+v", c.name, got, c.want)
		}
	}
	if got := JumpLimits(rules.Derived{}); got != (Jumps{}) {
		t.Errorf("a sheet with no Força: %+v", got)
	}
}

func TestJumpsWithAndWithoutARunningStart(t *testing.T) {
	t.Parallel()
	j := JumpLimits(strength(16))
	if j.Long(true) != 160 || j.Long(false) != 80 || j.High(true) != 60 || j.High(false) != 30 {
		t.Errorf("Long and High pick the wrong number: %+v", j)
	}
	// The move right before must be at least 10 ft.
	for _, c := range []struct {
		prev int
		want bool
	}{{0, false}, {99, false}, {100, true}, {300, true}} {
		if got := HasRunningStart(c.prev); got != c.want {
			t.Errorf("HasRunningStart(%d) = %v, want %v", c.prev, got, c.want)
		}
	}
}

func withFeature(d rules.Derived, key string) rules.Derived {
	d.Features = append(d.Features, rules.Feature{Key: key})
	return d
}

// Second-Story Work (SRD 5.1, Thief 3): a running long jump covers extra feet equal
// to the Dexterity modifier; the standing jump and the high jump do not change, and
// nobody else has it.
func TestSecondStoryWorkAddsTheDexterityModifierToTheRunningLongJump(t *testing.T) {
	t.Parallel()
	thief := withFeature(strength(10), "feature:second-story-work") // Destreza 18: +4
	got := JumpLimits(thief)
	want := Jumps{LongRunning: 100, LongStanding: 50, HighRunning: 30, HighStanding: 15, LongRunningBonus: 40}
	if got != want {
		t.Fatalf("thief: JumpLimits = %+v, want %+v", got, want)
	}
	if got.Long(true) != 140 || got.Long(false) != 50 || got.High(true) != 30 {
		t.Errorf("thief: running %d, standing %d, high %d, want 140, 50 and 30", got.Long(true), got.Long(false), got.High(true))
	}
	if plain := JumpLimits(strength(10)); plain.LongRunningBonus != 0 || plain.Long(true) != 100 {
		t.Errorf("without the feature: %+v", plain)
	}
	// A negative Dexterity modifier takes nothing away.
	clumsy := withFeature(rules.Derived{Abilities: []rules.AbilityScore{
		{Ability: rules.STR, Score: 10, Modifier: 0}, {Ability: rules.DEX, Score: 6, Modifier: -2},
	}}, "feature:second-story-work")
	if got := JumpLimits(clumsy); got.LongRunningBonus != 0 || got.Long(true) != 100 {
		t.Errorf("Destreza 6: %+v, want no bonus", got)
	}
}

// Step of the Wind (SRD 5.1, Monk 2) doubles the jump distance for the turn: every
// length, running or standing, long or high, and the Thief's bonus with it.
func TestStepOfTheWindDoublesTheJumpDistance(t *testing.T) {
	t.Parallel()
	j := JumpLimits(withFeature(strength(16), "feature:second-story-work"))
	if got := j.WithStepOfTheWind(false); got != j {
		t.Errorf("not used: %+v, want it unchanged", got)
	}
	d := j.WithStepOfTheWind(true)
	if d.Long(true) != 2*j.Long(true) || d.Long(false) != 2*j.Long(false) || d.High(true) != 2*j.High(true) || d.High(false) != 2*j.High(false) {
		t.Errorf("doubled: %+v from %+v", d, j)
	}
	if d.Long(true) != 2*(160+40) {
		t.Errorf("running long jump = %d, want 400", d.Long(true))
	}
}

// A real Thief of the SRD content has Second-Story Work from level 3, and a real
// monk has Step of the Wind from level 2.
func TestJumpFeaturesComeFromTheSRDContent(t *testing.T) {
	t.Parallel()
	c, err := rules.LoadSRD()
	if err != nil {
		t.Fatalf("LoadSRD: %v", err)
	}
	build := func(class, subclass string, level int) rules.Derived {
		return rules.Derive(rules.Build{
			BaseScores: map[rules.Ability]int{rules.STR: 10, rules.DEX: 16, rules.CON: 12, rules.INT: 10, rules.WIS: 12, rules.CHA: 10},
			Race:       "race:human", Background: "background:acolyte",
			Classes: []rules.ClassLevel{{Class: class, Subclass: subclass, Level: level}},
		}, c)
	}
	thief := JumpLimits(build("class:rogue", "subclass:thief", 3))
	if thief.LongRunningBonus != 30 {
		t.Errorf("Thief 3 (Destreza 16 +1 of the human = 17, +3): LongRunningBonus = %d, want 30", thief.LongRunningBonus)
	}
	if got := JumpLimits(build("class:rogue", "", 2)).LongRunningBonus; got != 0 {
		t.Errorf("Rogue 2: LongRunningBonus = %d, want 0", got)
	}
	if !rules.HasFeature(build("class:monk", "", 2), StepOfTheWindFeature) {
		t.Errorf("Monk 2 lacks %s", StepOfTheWindFeature)
	}
}
