package rules

import (
	"slices"
	"strings"
	"testing"
)

const (
	styleDefense = "feature:fighter-fighting-style-defense"
	styleDueling = "feature:fighter-fighting-style-dueling"
)

// A fighter 3 reaching level 4 (an Ability Score Improvement) is offered a swap of its
// Fighting Style; the swap is one pick of the group for another of its pool, and the
// check refuses it for any other level, group or pick.
func TestAFighterSwapsItsFightingStyleAtAnAbilityScoreImprovement(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	b := sweepUp(t, c, sweepBase(t, c, "class:fighter", "subclass:champion"), "class:fighter", "subclass:champion", 3)
	b.FeatureChoices = slices.DeleteFunc(slices.Clone(b.FeatureChoices), func(k string) bool { return strings.HasPrefix(k, "feature:fighter-fighting-style-") })
	b.FeatureChoices = append(b.FeatureChoices, styleDefense)

	o, _ := LevelUpOptions(b, "class:fighter", c)
	var g *SwapGroup
	for i := range o.PickSwaps {
		if o.PickSwaps[i].Group == SwapGroupFightStyle {
			g = &o.PickSwaps[i]
		}
	}
	if !o.AbilityScoreImprovement || g == nil {
		t.Fatalf("level 4 offers swaps %v (ASI %v), want the fighting style", o.PickSwaps, o.AbilityScoreImprovement)
	}
	if len(g.Picks) != 1 || g.Picks[0].Key != styleDefense || slices.ContainsFunc(g.Pool, func(n NamedKey) bool { return n.Key == styleDefense }) ||
		!slices.ContainsFunc(g.Pool, func(n NamedKey) bool { return n.Key == styleDueling }) {
		t.Fatalf("group %+v", *g)
	}
	// A level with no Ability Score Improvement offers nothing.
	if o3, _ := LevelUpOptions(sweepUp(t, c, sweepBase(t, c, "class:fighter", "subclass:champion"), "class:fighter", "subclass:champion", 2), "class:fighter", c); len(o3.PickSwaps) != 0 {
		t.Errorf("level 3 offers swaps %v", o3.PickSwaps)
	}

	ch := satisfy(t, c, b, "class:fighter", "subclass:champion")
	ch.AbilityIncrease = map[Ability]int{STR: 2}
	plain := mustApply(t, c, b, ch)
	if err := CheckLevelUp(b, plain, c); err != nil {
		t.Fatalf("the level without a swap: %v", err)
	}

	swap := ch
	swap.Swap = PickSwap{Group: SwapGroupFightStyle, From: styleDefense, To: styleDueling}
	after := mustApply(t, c, b, swap)
	if err := CheckLevelUp(b, after, c); err != nil {
		t.Fatalf("a swap of the style: %v", err)
	}
	if slices.Contains(after.FeatureChoices, styleDefense) || !slices.Contains(after.FeatureChoices, styleDueling) {
		t.Errorf("after the swap the picks are %v", after.FeatureChoices)
	}

	// Giving up a pick with nothing in its place, or two of them, is refused.
	lost := plain
	lost.FeatureChoices = slices.DeleteFunc(slices.Clone(plain.FeatureChoices), func(k string) bool { return k == styleDefense })
	wantRefusal(t, CheckLevelUp(b, lost, c), LevelUpReasonFeatureChoice, "full.feature_choice_keys")
}

// A cantrip is swapped for another of the class's list.
func TestAWizardSwapsACantripAtAnAbilityScoreImprovement(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	b := sweepUp(t, c, sweepBase(t, c, "class:wizard", "subclass:evocation"), "class:wizard", "subclass:evocation", 3)
	o, _ := LevelUpOptions(b, "class:wizard", c)
	var g *SwapGroup
	for i := range o.PickSwaps {
		if o.PickSwaps[i].Group == SwapGroupCantrip {
			g = &o.PickSwaps[i]
		}
	}
	if g == nil || len(g.Picks) == 0 || len(g.Pool) == 0 {
		t.Fatalf("offer %v", o.PickSwaps)
	}
	ch := satisfy(t, c, b, "class:wizard", "subclass:evocation")
	ch.AbilityIncrease = map[Ability]int{INT: 2}
	to := g.Pool[slices.IndexFunc(g.Pool, func(n NamedKey) bool { return !slices.Contains(ch.Cantrips, n.Key) })].Key
	ch.Swap = PickSwap{Group: SwapGroupCantrip, From: g.Picks[0].Key, To: to}
	after := mustApply(t, c, b, ch)
	if err := CheckLevelUp(b, after, c); err != nil {
		t.Fatalf("a swap of a cantrip: %v", err)
	}
	if slices.Contains(after.Cantrips, ch.Swap.From) || !slices.Contains(after.Cantrips, ch.Swap.To) {
		t.Errorf("cantrips %v", after.Cantrips)
	}
}
