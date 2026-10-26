package rules

import (
	"strings"
	"testing"
)

// The fly, swim, climb and burrow speeds a table gives, and the effect that
// lets a character breathe water. Every name here is invented.

const (
	swimmerRace = "race:nadadora" + tableSuffix
	windFeat    = "feat:vento-leve" + tableSuffix
)

// trait is tf for a race's trait.
func trait(slug, name string, effects ...Effect) TableFeature {
	f := tf(slug, name, effects...)
	f.Key = "trait:" + slug + tableSuffix
	return f
}

func speedsOverlay(walkBonus int) Overlay {
	traits := []TableFeature{
		trait("nada-como-anda", "Nada como anda", Effect{Type: "modifier", Target: "speed.swim", Mode: "set", Value: "speedwalk()"}),
		trait("pulmao-de-peixe", "Pulmão de peixe", Effect{Type: "breathes_water"}),
		trait("sobe-devagar", "Sobe devagar", Effect{Type: "modifier", Target: "speed.climb", Mode: "add", Value: "speedwalk() / 2", When: "level() >= 3"}),
	}
	if walkBonus != 0 {
		traits = append(traits, trait("passo-largo", "Passo largo", Effect{Type: "modifier", Target: "speed.walk", Mode: "add", Value: "10"}))
	}
	return Overlay{
		Revision: 1,
		Races: []TableRace{{
			TableEntry: TableEntry{Key: swimmerRace, NamePT: "Nadadora"},
			Size:       "Medium", SpeedFt: 30, Traits: traits,
		}},
		Feats: []TableFeat{{
			TableEntry: TableEntry{Key: windFeat, NamePT: "Vento leve"},
			Effects:    []Effect{{Type: "modifier", Target: "speed.fly", Mode: "add", Value: "10"}},
		}},
	}
}

func swimmerBuild(level int, feats ...string) Build {
	return Build{
		BaseScores: map[Ability]int{STR: 15, DEX: 14, CON: 13, INT: 12, WIS: 10, CHA: 8},
		Race:       swimmerRace, Background: "background:acolyte", Feats: feats,
		Classes: []ClassLevel{{Class: "class:fighter", Level: level}},
	}
}

func TestOtherSpeedsFollowTheWalkingSpeed(t *testing.T) {
	c := withOverlay(t, speedsOverlay(0))
	d := Derive(swimmerBuild(1), c)
	if d.SpeedWalkFt != 30 || d.SpeedSwimFt != 30 {
		t.Errorf("walk %d swim %d, want 30 and 30", d.SpeedWalkFt, d.SpeedSwimFt)
	}
	if d.SpeedClimbFt != 0 || d.SpeedFlyFt != 0 || d.SpeedBurrowFt != 0 {
		t.Errorf("climb %d fly %d burrow %d, want none before the level the climb needs", d.SpeedClimbFt, d.SpeedFlyFt, d.SpeedBurrowFt)
	}
	if !d.BreathesWater {
		t.Error("the water breather does not breathe water")
	}
	if got := Derive(swimmerBuild(3), c).SpeedClimbFt; got != 15 {
		t.Errorf("climb at level 3 = %d, want half the walking speed", got)
	}

	// A change to the walking speed carries to every speed that reads it.
	c = withOverlay(t, speedsOverlay(10))
	d = Derive(swimmerBuild(3), c)
	if d.SpeedWalkFt != 40 || d.SpeedSwimFt != 40 || d.SpeedClimbFt != 20 {
		t.Errorf("walk %d swim %d climb %d, want 40, 40 and 20", d.SpeedWalkFt, d.SpeedSwimFt, d.SpeedClimbFt)
	}
}

func TestAFeatAddsAFlySpeed(t *testing.T) {
	c := withOverlay(t, speedsOverlay(0))
	if got := Derive(swimmerBuild(1, windFeat), c).SpeedFlyFt; got != 10 {
		t.Errorf("fly speed = %d, want 10", got)
	}
	if got := Derive(swimmerBuild(1), c).SpeedFlyFt; got != 0 {
		t.Errorf("fly speed without the feat = %d, want 0", got)
	}
}

func TestACharacterWithoutTheEffectsHasNoOtherSpeed(t *testing.T) {
	c := loadForTest(t)
	d := Derive(Build{
		BaseScores: map[Ability]int{STR: 15, DEX: 14, CON: 13, INT: 12, WIS: 10, CHA: 8},
		Race:       "race:human", Background: "background:acolyte",
		Classes: []ClassLevel{{Class: "class:fighter", Level: 1}},
	}, c)
	if d.SpeedFlyFt+d.SpeedSwimFt+d.SpeedClimbFt+d.SpeedBurrowFt != 0 || d.BreathesWater {
		t.Errorf("speeds %d/%d/%d/%d, breathes water %v, want none", d.SpeedFlyFt, d.SpeedSwimFt, d.SpeedClimbFt, d.SpeedBurrowFt, d.BreathesWater)
	}
}

func TestSpeedwalkBelongsToTheOtherSpeeds(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)
	cases := map[string]Effect{
		"the walking speed itself": {Type: "modifier", Target: "speed.walk", Mode: "add", Value: "speedwalk()"},
		"another target":           {Type: "modifier", Target: "ac", Mode: "add", Value: "speedwalk() / 10"},
		"a condition":              {Type: "modifier", Target: "ac", Mode: "add", Value: "1", When: "speedwalk() > 30"},
	}
	for name, e := range cases {
		_, err := srd.With(Overlay{Revision: 1, Classes: []TableClass{classWithEffects([]Effect{e})}})
		if err == nil || !strings.Contains(err.Error(), "speedwalk()") {
			t.Errorf("%s: err = %v, want a refusal naming speedwalk()", name, err)
		}
	}
	ok := Effect{Type: "modifier", Target: "speed.burrow", Mode: "max", Value: "speedwalk() / 2", When: "speedwalk() >= 30"}
	if _, err := srd.With(Overlay{Revision: 1, Classes: []TableClass{classWithEffects([]Effect{ok})}}); err != nil {
		t.Errorf("speedwalk() on the burrow speed: %v", err)
	}
}

func TestBreathesWaterTakesNoFields(t *testing.T) {
	t.Parallel()
	if got := unusedField(&Effect{Type: "breathes_water", Value: "1"}); got != "value" {
		t.Errorf("unusedField = %q, want the stray value", got)
	}
	if got := unusedField(&Effect{Type: "breathes_water", When: "level() > 1", TextPT: "Respira na água."}); got != "" {
		t.Errorf("unusedField = %q, want a when and a text to be fine", got)
	}
}
