package rules

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// A subclass choice of the table: one option of a list at a class level, each
// option with effects of its own. The names are invented.

const (
	forkSubclass = "subclass:bifurcacao-de-teste" + tableSuffix
	forkFeature  = "feature:bifurcacao" + tableSuffix
	forkA        = "feature:caminho-a" + tableSuffix
	forkB        = "feature:caminho-b" + tableSuffix
	forkC        = "feature:caminho-c" + tableSuffix
)

// forkOverlay is a table subclass of the SRD Barbarian whose level 3 feature asks
// for one of three paths: A adds 2 to the initiative, B gives darkvision and C
// resists every damage type but one while raging.
func forkOverlay() Overlay {
	fork := TableFeature{
		Key: forkFeature, NamePT: "Bifurcação", DescPT: []string{"Escolha um caminho."},
		Effects: []Effect{{Type: "choice", Choice: "feature", Count: 1}},
		Options: []TableFeature{
			postureOption(forkA, "Caminho A", Effect{Type: "modifier", Target: "initiative", Mode: "add", Value: "2"}),
			postureOption(forkB, "Caminho B", Effect{Type: "sense", Sense: "darkvision", RangeFt: 60}),
			postureOption(forkC, "Caminho C", resistanceEffect()),
		},
	}
	return Overlay{Revision: 1, Subclasses: []TableSubclass{{
		Key: forkSubclass, NamePT: "Bifurcação de Teste", Class: "class:barbarian",
		Levels: []TableSubclassLevel{{Level: 3, Features: []TableFeature{fork}}},
	}}}
}

func forkBuild(level int, picks ...string) Build {
	b := pathBuild(level)
	b.Classes[0].Subclass = forkSubclass
	b.FeatureChoices = picks
	return b
}

func TestSubclassChoiceAppliesOnlyThePickedOption(t *testing.T) {
	c := withOverlay(t, forkOverlay())
	none := Derive(forkBuild(3), c)
	a := Derive(forkBuild(3, forkA), c)
	if a.Initiative != none.Initiative+2 {
		t.Errorf("initiative with path A = %d, without a pick %d", a.Initiative, none.Initiative)
	}
	if slices.ContainsFunc(a.Senses, func(s Sense) bool { return s.Key == "darkvision" }) {
		t.Error("path B's sense applies though only A is picked")
	}
	if _, ok := resistanceOf(c.Resistances(a), forkC); ok {
		t.Error("path C's resistance applies though only A is picked")
	}

	// Changing the pick moves the effects with it.
	b := Derive(forkBuild(3, forkB), c)
	if b.Initiative != none.Initiative || !slices.ContainsFunc(b.Senses, func(s Sense) bool { return s.Key == "darkvision" }) {
		t.Errorf("path B: initiative %d (want %d), senses %+v", b.Initiative, none.Initiative, b.Senses)
	}
	if _, ok := resistanceOf(c.Resistances(Derive(forkBuild(3, forkC), c)), forkC); !ok {
		t.Error("path C picked, no resistance")
	}
}

func TestSubclassChoiceIsOfferedAtItsLevelOnly(t *testing.T) {
	c := withOverlay(t, forkOverlay())
	ch := optionChoiceOf(t, c.Choices(forkBuild(3)), forkFeature)
	if ch.Picks != 1 || len(ch.Options) != 3 {
		t.Fatalf("choice = %d picks of %d options, want 1 of 3", ch.Picks, len(ch.Options))
	}
	d := Derive(forkBuild(2, forkA), c)
	if !slices.ContainsFunc(d.Issues, func(i Issue) bool { return i.Code == IssueUnknownKey }) {
		t.Errorf("issues = %+v, want the pick before the feature's level named as not valid", d.Issues)
	}
}

func TestSubclassChoiceRefusesWhatItDoesNotOffer(t *testing.T) {
	c := withOverlay(t, forkOverlay())
	d := Derive(forkBuild(3, "feature:caminho-d"+tableSuffix), c)
	if !slices.ContainsFunc(d.Issues, func(i Issue) bool { return i.Code == IssueUnknownKey }) {
		t.Errorf("issues = %+v, want an unknown pick named", d.Issues)
	}
	d = Derive(forkBuild(3, forkA, forkB), c)
	if !slices.ContainsFunc(d.Issues, func(i Issue) bool { return i.Code == IssueChoiceCount }) {
		t.Errorf("issues = %+v, want two picks of a choice of one refused", d.Issues)
	}
}

func TestSubclassChoiceLoaderRefusals(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		edit  func(o *Overlay)
		field string
	}{
		{"a level the class does not have", func(o *Overlay) { o.Subclasses[0].Levels[0].Level = MaxLevel + 1 }, "subclasses[0].levels[0].level"},
		{"an option with a bad effect", func(o *Overlay) {
			o.Subclasses[0].Levels[0].Features[0].Options[2].Effects[0].Except = "damage-type:fixture-none"
		}, "subclasses[0].levels[0].features[0].options[2].effects[0].except"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			o := forkOverlay()
			tc.edit(&o)
			_, err := loadForTest(t).With(o)
			var oe *OverlayError
			if !errors.As(err, &oe) {
				t.Fatalf("want an OverlayError, got %v", err)
			}
			if !slices.ContainsFunc(oe.Violations(), func(v *OverlayError) bool { return strings.HasSuffix(v.Field, tc.field) }) {
				t.Errorf("violations = %+v, want one at %s", oe.Violations(), tc.field)
			}
		})
	}
}
