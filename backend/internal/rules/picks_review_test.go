package rules

import (
	"slices"
	"strings"
	"testing"
)

// A name is only for a choice of an ability or a damage type, and starts with a letter.
func TestAPickNameRules(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)
	for name, tc := range map[string]struct {
		e      Effect
		field  string
		reason string
	}{
		"a name on a skill choice":  {Effect{Choice: "skill", Count: 2, Name: "x"}, "feats[0].effects[0].name", ReasonValue},
		"a name on a choice of one": {Effect{Choice: "skill", Count: 1, Name: "x"}, "feats[0].effects[0].name", ReasonValue},
		"a digit first":             {Effect{Choice: "ability", Count: 1, Name: "1x"}, "feats[0].effects[0].name", ReasonValue},
		"an underscore first":       {Effect{Choice: "ability", Count: 1, Name: "_x"}, "feats[0].effects[0].name", ReasonValue},
		"a letter first":            {Effect{Choice: "ability", Count: 1, Name: "x1"}, "", ""},
	} {
		e := tc.e
		e.Type = "choice"
		_, err := srd.With(featWith("feat:nome"+tableSuffix, e))
		if tc.field == "" {
			if err != nil {
				t.Errorf("%s: %v", name, err)
			}
			continue
		}
		if err == nil {
			t.Errorf("%s: accepted", name)
			continue
		}
		violationAt(t, err, tc.field, tc.reason)
	}
	// "feat:x#1" is the text of a choice of the feat, never a pick.
	if validPickName("1") || !validPickName("a1") {
		t.Error("validPickName does not require a first letter")
	}
}

func TestASetChoiceCannotAskForMoreThanTheSet(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)
	big := MaxChoiceCount
	_, err := srd.With(featWith("feat:muitos"+tableSuffix, Effect{Type: "choice", Choice: "feature", Count: big, Set: "fighting_style"}))
	if err == nil {
		t.Fatal("a count above the size of the set was accepted")
	}
	violationAt(t, err, "feats[0].effects[0].count", ReasonValue)
	if _, err := srd.With(featWith("feat:um"+tableSuffix, Effect{Type: "choice", Choice: "feature", Count: 1, Set: "fighting_style"})); err != nil {
		t.Errorf("a count of 1: %v", err)
	}
}

func TestAFeatureWithOptionsAndASetChoice(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)
	f := TableFeat{TableEntry: TableEntry{Key: "feat:misto" + tableSuffix, NamePT: "Misto"}, DescPT: []string{"Texto."},
		Options: []TableFeature{{Key: "feature:o" + tableSuffix, NamePT: "O", DescPT: []string{"x"}}},
		Effects: []Effect{{Type: "choice", Choice: "feature", Count: 1, Set: "fighting_style"}}}
	_, err := srd.With(Overlay{Revision: 1, Feats: []TableFeat{f}})
	if err == nil {
		t.Fatal("a feat with options and only a set choice was accepted")
	}
	violationAt(t, err, "feats[0].effects[0].set", ReasonValue)
}

func TestAStalePickIsNotOffered(t *testing.T) {
	t.Parallel()
	c := withOverlay(t, resilientOverlay())
	b := resilientBuild(16, resilientPick("ability:dex"))
	b.Feats = nil
	problems := c.Choices(b).ChoiceProblems(false)
	if !slices.ContainsFunc(problems, func(p ChoiceProblem) bool { return p.Code == ChoiceProblemNotOffered }) {
		t.Errorf("problems = %+v, want the pick of a removed feat not offered", problems)
	}
	if d := Derive(b, c); abilityOf(d, DEX).Score != 16 || saveOf(d, DEX).Proficient {
		t.Error("the pick of a feat the sheet lacks applied")
	}
}

func TestAnInactiveFeatAppliesNoPick(t *testing.T) {
	t.Parallel()
	o := resilientOverlay()
	o.Feats[0].Prerequisite = FeatPrerequisite{Minimums: map[Ability]int{STR: 20}}
	c := withOverlay(t, o)
	d := Derive(resilientBuild(16, resilientPick("ability:dex")), c)
	if abilityOf(d, DEX).Score != 16 || saveOf(d, DEX).Proficient || hintWith(d, "CD ") {
		t.Errorf("an inactive feat applied its pick: %+v", d.Hints)
	}
	if !slices.ContainsFunc(d.Issues, func(is Issue) bool { return is.Code == IssueFeatPrerequisite }) {
		t.Errorf("issues = %+v, want the lost prerequisite", d.Issues)
	}
	if len(d.FeatPicks) != 0 {
		t.Errorf("FeatPicks = %v", d.FeatPicks)
	}
}

// A pick of a feat the master hid stays named for the player who holds it, and nothing hidden shows by name.
func TestForPlayerKeepsAHeldPickNamed(t *testing.T) {
	t.Parallel()
	const key = "feat:elemental" + tableSuffix
	o := featWith(key,
		Effect{Type: "choice", Choice: "damage_type", Name: "tipo", Count: 1, From: []string{"damage-type:fire", "damage-type:cold"}},
		Effect{Type: "resistance", Kind: "resist", DamageTypes: []string{"damage-type:@tipo"}},
	)
	o.Strict, o.Off = nil, []string{key}
	c := withOverlay(t, o)
	player := c.ForPlayer()
	b := pensantus()
	pick := ScopedChoice(PickChoiceKey(key, "tipo"), "damage-type:cold")
	b.Feats, b.FeatureChoices = []string{key}, []string{pick}
	ch := choiceOf(t, player.Choices(b), PickChoiceKey(key, "tipo"))
	if len(ch.Picked) != 1 || !slices.Contains(optionKeys(ch), "damage-type:cold") {
		t.Errorf("the held pick is not named: options %v picked %v", optionKeys(ch), ch.Picked)
	}
	if rs := player.Resistances(Derive(b, player)); len(rs) != 1 {
		t.Errorf("resistances = %+v", rs)
	}
	// Without the feat held, the hidden feat's picks are not offered to the player.
	b.Feats, b.FeatureChoices = nil, nil
	for _, g := range player.Choices(b).Groups {
		for _, ch := range g.Choices {
			if strings.HasPrefix(ch.Key, key) {
				t.Errorf("the player is offered %q of a hidden feat", ch.Key)
			}
		}
	}
}

// Resistances reads the picks Derive recorded: a Derived without them lists no resistance that names a pick.
func TestResistancesReadFeatPicks(t *testing.T) {
	t.Parallel()
	const key = "feat:elemental" + tableSuffix
	o := featWith(key,
		Effect{Type: "choice", Choice: "damage_type", Name: "tipo", Count: 1, From: []string{"damage-type:fire", "damage-type:cold"}},
		Effect{Type: "resistance", Kind: "resist", DamageTypes: []string{"damage-type:@tipo"}},
	)
	o.Strict = nil
	c := withOverlay(t, o)
	b := pensantus()
	b.Feats, b.FeatureChoices = []string{key}, []string{ScopedChoice(PickChoiceKey(key, "tipo"), "damage-type:fire")}
	d := Derive(b, c)
	if want := PickChoiceKey(key, "tipo"); d.FeatPicks[want] != "damage-type:fire" {
		t.Fatalf("FeatPicks = %v", d.FeatPicks)
	}
	if rs := c.Resistances(d); len(rs) != 1 || !slices.Equal(rs[0].DamageTypes, []string{"damage-type:fire"}) {
		t.Errorf("resistances = %+v", rs)
	}
	d.FeatPicks = nil
	if rs := c.Resistances(d); len(rs) != 0 {
		t.Errorf("a Derived with no picks lists %+v", rs)
	}
}
