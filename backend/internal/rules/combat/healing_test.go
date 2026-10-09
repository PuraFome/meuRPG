package combat

import (
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

func TestHealFeaturesOf(t *testing.T) {
	t.Parallel()
	feats := func(keys ...string) []rules.Feature {
		var out []rules.Feature
		for _, k := range keys {
			out = append(out, rules.Feature{Key: k})
		}
		return out
	}
	tests := []struct {
		name string
		in   []rules.Feature
		want HealFeatures
	}{
		{"none", feats("feature:channel-divinity"), HealFeatures{}},
		{"a cleric of the Life Domain at 1", feats(DiscipleOfLifeKey), HealFeatures{Disciple: true}},
		{"at 6", feats(DiscipleOfLifeKey, BlessedHealerKey), HealFeatures{Disciple: true, Blessed: true}},
		{"at 17", feats(DiscipleOfLifeKey, BlessedHealerKey, SupremeHealingKey), HealFeatures{true, true, true}},
	}
	for _, tt := range tests {
		if got := HealFeaturesOf(tt.in); got != tt.want {
			t.Errorf("%s: HealFeaturesOf = %+v, want %+v", tt.name, got, tt.want)
		}
	}
}

func TestHealFeaturesNumbers(t *testing.T) {
	t.Parallel()
	all := HealFeatures{Disciple: true, Blessed: true, Supreme: true}
	// Disciple of Life: 2 + the spell's level, for a spell of 1st level or higher (SRD 5.1).
	for slot, want := range map[int]int{0: 0, 1: 3, 2: 4, 5: 7, 9: 11} {
		if got := all.TargetExtra(slot); got != want {
			t.Errorf("TargetExtra(%d) = %d, want %d", slot, got, want)
		}
		if got := all.SelfExtra(slot, true); got != want {
			t.Errorf("SelfExtra(%d) = %d, want %d", slot, got, want)
		}
	}
	if got := (HealFeatures{Disciple: true}).SelfExtra(3, true); got != 0 {
		t.Errorf("Disciple alone heals the caster %d, want 0", got)
	}
	if got := (HealFeatures{Blessed: true}).TargetExtra(3); got != 0 {
		t.Errorf("Blessed Healer alone adds %d to the target, want 0", got)
	}
	// Blessed Healer needs a creature other than the caster to be healed.
	if got := all.SelfExtra(3, false); got != 0 {
		t.Errorf("SelfExtra when only the caster was healed = %d, want 0", got)
	}
	// Supreme Healing: the highest number of each die. Cure Wounds at the 2nd level is 2d8 + 3.
	if got := all.HealTotal(2, 8, 3, 5); got != 19 {
		t.Errorf("HealTotal with Supreme Healing = %d, want 19", got)
	}
	if got := (HealFeatures{}).HealTotal(2, 8, 3, 5); got != 8 {
		t.Errorf("HealTotal as rolled = %d, want 8", got)
	}
}

// TestLifeDomainFeaturesComeWithTheLevels reads a real sheet: the features appear at the class
// levels the SRD gives them (Disciple of Life 1, Blessed Healer 6, Supreme Healing 17).
func TestLifeDomainFeaturesComeWithTheLevels(t *testing.T) {
	t.Parallel()
	c, err := rules.LoadSRD()
	if err != nil {
		t.Fatal(err)
	}
	at := func(level int) HealFeatures {
		b := rules.Build{
			BaseScores: map[rules.Ability]int{rules.STR: 10, rules.DEX: 12, rules.CON: 14, rules.INT: 10, rules.WIS: 16, rules.CHA: 10},
			Race:       "race:human", Background: "background:acolyte",
			Classes: []rules.ClassLevel{{Class: "class:cleric", Subclass: "subclass:life", Level: level}},
		}
		return HealFeaturesOf(rules.Derive(b, c).Features)
	}
	for level, want := range map[int]HealFeatures{
		1: {Disciple: true}, 5: {Disciple: true}, 6: {Disciple: true, Blessed: true}, 16: {Disciple: true, Blessed: true}, 17: {Disciple: true, Blessed: true, Supreme: true},
	} {
		if got := at(level); got != want {
			t.Errorf("a Life cleric of level %d has %+v, want %+v", level, got, want)
		}
	}
}
