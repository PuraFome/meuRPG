package rules

import (
	"slices"
	"testing"
)

// traitOptionsOverlay is the table race of tableMisc with a "one of" choice on its trait: two
// option traits, one of them with an effect.
func traitOptionsOverlay() (Overlay, string, []string) {
	race, sub, bg := tableMisc()
	race.Traits[0].Options = []TableFeature{
		{Key: "trait:cor-a" + tableSuffix, NamePT: "Cor A", DescPT: []string{"A."}},
		{Key: "trait:cor-b" + tableSuffix, NamePT: "Cor B", Effects: []Effect{{Type: "note", TextPT: "B."}}},
	}
	opts := []string{"trait:cor-a" + tableSuffix, "trait:cor-b" + tableSuffix}
	return Overlay{Races: []TableRace{race}, Subraces: []TableSubrace{sub}, Backgrounds: []TableBackground{bg}}, race.Traits[0].Key, opts
}

// TestTableTraitOptionsAreRegisteredLikeTheSRDs: a table trait with options is the shape of the
// SRD's Draconic Ancestry: the trait lists the options, each option trait has the trait as its
// parent and the owner, and only the trait is in the race's list (an option is inactive until picked).
func TestTableTraitOptionsAreRegisteredLikeTheSRDs(t *testing.T) {
	t.Parallel()
	o, traitKey, opts := traitOptionsOverlay()
	c := withOverlay(t, o).c
	tr := c.traits[traitKey]
	if tr == nil || !slices.Equal(tr.Options, opts) || tr.Parent != "" {
		t.Fatalf("trait = %+v, want options %v and no parent", tr, opts)
	}
	raceKey := o.Races[0].Key
	for _, k := range opts {
		ot := c.traits[k]
		if ot == nil || ot.Parent != traitKey || !slices.Equal(ot.Races, []string{raceKey}) || len(ot.Options) != 0 {
			t.Errorf("option %s = %+v, want parent %s of %s", k, ot, traitKey, raceKey)
		}
		if slices.Contains(c.races[raceKey].Traits, k) {
			t.Errorf("option %s is in the race's traits %v", k, c.races[raceKey].Traits)
		}
	}
	if !slices.Equal(c.races[raceKey].Traits, []string{traitKey}) {
		t.Errorf("race traits = %v, want only %s", c.races[raceKey].Traits, traitKey)
	}
	if len(c.effects[opts[1]]) != 1 {
		t.Errorf("option effects = %v, want the note", c.effects[opts[1]])
	}
}

// TestATableTraitWithOptionsAsksForOneOfThem: the trait yields a choice of one over its options,
// and a pick that is not one of them is a problem the rules refuse (a trait's picks are bare option keys,
// as the Draconic Ancestry's, so the choice engine says "not offered"; choiceValueOK is for scoped picks).
func TestATableTraitWithOptionsAsksForOneOfThem(t *testing.T) {
	t.Parallel()
	o, traitKey, opts := traitOptionsOverlay()
	c := withOverlay(t, o)
	cl := ClassLevel{Class: "class:fighter", Level: 1}
	raceKey := o.Races[0].Key
	s := c.Choices(choiceBuild(raceKey, "", cl))
	ch := choiceOf(t, s, traitKey)
	if ch.Picks != 1 || !slices.Equal(optionKeys(ch), opts) {
		t.Fatalf("choice picks %d options %v, want 1 of %v", ch.Picks, optionKeys(ch), opts)
	}
	if p := c.Choices(choiceBuild(raceKey, "", cl, opts[0])).ChoiceProblems(false); len(p) != 0 {
		t.Errorf("a valid pick has problems %v", p)
	}
	outside := "trait:olhar-firme" + tableSuffix
	p := c.Choices(choiceBuild(raceKey, "", cl, outside)).ChoiceProblems(false)
	if len(p) != 1 || p[0].Code != ChoiceProblemNotOffered || p[0].OptionKey != outside {
		t.Errorf("problems = %+v, want %q not offered", p, outside)
	}
}
