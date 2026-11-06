package rules

import (
	"slices"
	"testing"
)

// replaceFixture is a table race with two traits that carry options (the first one's option
// B gives a skill), a subrace with a trait of its own, and the keys the tests pick.
type replaceFixture struct {
	c                                 *Content
	race                              TableRace
	subrace                           TableSubrace
	kept, replaced, own               string
	keptA, keptB, replacedX, replaceY string
}

func newReplaceFixture(t *testing.T, replaces, swapBonuses bool) replaceFixture {
	t.Helper()
	race, sub, _ := tableMisc()
	race.ChoiceBonuses, race.LanguageChoices = nil, 0
	f := replaceFixture{race: race, subrace: sub}
	f.kept, f.replaced, f.own = "trait:forca-do-rio"+tableSuffix, "trait:marca-do-vale"+tableSuffix, sub.Traits[0].Key
	f.keptA, f.keptB = "trait:rio-a"+tableSuffix, "trait:rio-b"+tableSuffix
	f.replacedX, f.replaceY = "trait:vale-x"+tableSuffix, "trait:vale-y"+tableSuffix
	f.race.Traits = []TableFeature{
		{Key: f.kept, NamePT: "Força do rio", Options: []TableFeature{
			{Key: f.keptA, NamePT: "Rio A"},
			{Key: f.keptB, NamePT: "Rio B", Effects: []Effect{{Type: "proficiency", Proficiency: "skill:stealth"}}},
		}},
		{Key: f.replaced, NamePT: "Marca do vale", Options: []TableFeature{
			{Key: f.replacedX, NamePT: "Vale X", Effects: []Effect{{Type: "proficiency", Proficiency: "skill:arcana"}}},
			{Key: f.replaceY, NamePT: "Vale Y"},
		}},
	}
	f.subrace.Race = f.race.Key
	f.subrace.AbilityBonuses = map[Ability]int{DEX: 2}
	f.subrace.ReplacesAbilityBonuses = swapBonuses
	if replaces {
		f.subrace.ReplacesTraitKeys = []string{f.replaced}
	}
	f.c = withOverlay(t, Overlay{Races: []TableRace{f.race}, Subraces: []TableSubrace{f.subrace}})
	return f
}

func (f replaceFixture) build(subrace bool, picks ...string) Build {
	sub := ""
	if subrace {
		sub = f.subrace.Key
	}
	return choiceBuild(f.race.Key, sub, ClassLevel{Class: "class:fighter", Level: 1}, picks...)
}

func scoreBonus(d Derived, a Ability) int {
	for _, s := range d.Abilities {
		if s.Ability == a {
			return s.RaceBonus
		}
	}
	return -1
}

// TestASubraceReplacesTraitsAndBonusesAndKeepsItsOwn: the parent's traits minus the replaced one, plus the
// subrace's own, then the picked options; the replaced trait offers no choice, its option is not honoured,
// and the subrace's bonuses stand alone.
func TestASubraceReplacesTraitsAndBonusesAndKeepsItsOwn(t *testing.T) {
	t.Parallel()
	f := newReplaceFixture(t, true, true)
	b := f.build(true, f.keptB, f.replacedX)
	d := Derive(b, f.c)

	var got []string
	for _, k := range featureKeys(d) {
		if slices.Contains([]string{f.race.Key, f.kept, f.keptB, f.subrace.Key, f.own, f.replaced, f.replacedX}, k) {
			got = append(got, k)
		}
	}
	want := []string{f.kept, f.own, f.keptB}
	if !slices.Equal(got, want) {
		t.Errorf("traits on the sheet = %v, want %v", got, want)
	}
	if hasFeature(d, f.replaced) || hasFeature(d, f.replacedX) {
		t.Errorf("the replaced trait or its option is on the sheet: %v", featureKeys(d))
	}
	if skillLevel(d, "skill:stealth") == 0 || skillLevel(d, "skill:arcana") != 0 {
		t.Errorf("stealth %v, arcana %v, want the picked option's skill only", skillLevel(d, "skill:stealth"), skillLevel(d, "skill:arcana"))
	}
	if scoreBonus(d, CON) != 0 || scoreBonus(d, WIS) != 0 || scoreBonus(d, DEX) != 2 {
		t.Errorf("bonuses CON %d WIS %d DEX %d, want 0, 0 and 2: the subrace's replace the race's", scoreBonus(d, CON), scoreBonus(d, WIS), scoreBonus(d, DEX))
	}
	if !hasIssueAt(d, IssueUnknownKey, "full.feature_choice_keys[1]") {
		t.Errorf("no issue for the pick of the replaced trait: %v", issueCodes(d))
	}

	s := f.c.Choices(b)
	var keys []string
	for _, g := range s.Groups {
		if g.Origin != ChoiceOriginRace {
			continue
		}
		for _, ch := range g.Choices {
			keys = append(keys, ch.Key)
		}
	}
	if !slices.Equal(keys, []string{f.kept}) {
		t.Errorf("the race's choices = %v, want only %s", keys, f.kept)
	}
	if !slices.Equal(s.NotOffered, []string{f.replacedX}) {
		t.Errorf("not offered = %v, want %s", s.NotOffered, f.replacedX)
	}
	p := s.ChoiceProblems(false)
	if len(p) != 1 || p[0].Code != ChoiceProblemNotOffered || p[0].OptionKey != f.replacedX {
		t.Errorf("problems = %+v, want %s not offered", p, f.replacedX)
	}
}

// TestReplacingTraitsWithoutTheFlagKeepsTheBonusesAdditive: the bonuses of the race and of the subrace add up.
func TestReplacingTraitsWithoutTheFlagKeepsTheBonusesAdditive(t *testing.T) {
	t.Parallel()
	f := newReplaceFixture(t, true, false)
	d := Derive(f.build(true), f.c)
	if scoreBonus(d, CON) != 1 || scoreBonus(d, DEX) != 2 {
		t.Errorf("bonuses CON %d DEX %d, want 1 and 2", scoreBonus(d, CON), scoreBonus(d, DEX))
	}
	if hasFeature(d, f.replaced) {
		t.Errorf("the replaced trait is on the sheet")
	}
}

// TestAPickedTableTraitOptionIsOwnedAndApplies: the option the character picked is on the sheet and its
// effects apply; an option nobody picked does not, and without the subrace nothing is replaced.
func TestAPickedTableTraitOptionIsOwnedAndApplies(t *testing.T) {
	t.Parallel()
	f := newReplaceFixture(t, true, true)
	d := Derive(f.build(false, f.keptB, f.replacedX), f.c)
	for _, k := range []string{f.kept, f.keptB, f.replaced, f.replacedX} {
		if !hasFeature(d, k) {
			t.Errorf("%s is not on the sheet without the subrace: %v", k, featureKeys(d))
		}
	}
	if hasFeature(d, f.keptA) || hasFeature(d, f.replaceY) {
		t.Errorf("an option nobody picked is on the sheet: %v", featureKeys(d))
	}
	if skillLevel(d, "skill:stealth") == 0 || skillLevel(d, "skill:arcana") == 0 {
		t.Errorf("the picked options' skills do not apply")
	}
	if scoreBonus(d, CON) != 1 || scoreBonus(d, DEX) != 0 {
		t.Errorf("bonuses CON %d DEX %d, want the race's only", scoreBonus(d, CON), scoreBonus(d, DEX))
	}

	none := Derive(f.build(false), f.c)
	if hasFeature(none, f.keptB) || skillLevel(none, "skill:stealth") != 0 {
		t.Errorf("an unpicked option applies")
	}
}

// TestTheOptionsOfAReplacedTraitCannotBePicked: with the subrace, the replaced trait offers no choice and
// its options are not pickable; without the subrace they are.
func TestTheOptionsOfAReplacedTraitCannotBePicked(t *testing.T) {
	t.Parallel()
	f := newReplaceFixture(t, true, false)
	with := f.c.Choices(f.build(true, f.replaceY))
	if p := with.ChoiceProblems(false); !slices.ContainsFunc(p, func(p ChoiceProblem) bool {
		return p.Code == ChoiceProblemNotOffered && p.OptionKey == f.replaceY
	}) {
		t.Errorf("problems = %+v, want %s not offered", p, f.replaceY)
	}
	if d := Derive(f.build(true, f.replaceY), f.c); hasFeature(d, f.replaceY) {
		t.Errorf("the option of a replaced trait is on the sheet")
	}
	without := f.c.Choices(f.build(false, f.replaceY))
	if ch := choiceOf(t, without, f.replaced); !slices.Contains(optionKeys(ch), f.replaceY) {
		t.Errorf("options = %v, want %s", optionKeys(ch), f.replaceY)
	}
	if len(without.NotOffered) != 0 {
		t.Errorf("not offered = %v without the subrace, want none", without.NotOffered)
	}
}
