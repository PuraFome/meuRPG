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
	race.LanguageChoices = 0
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

// bonusPicks are the picks of the race's placed bonuses (+2 on Strength, +1 on Intelligence).
func (f replaceFixture) bonusPicks() []string {
	return []string{
		ScopedChoice(BonusChoiceKey(f.race.Key, 2), abilityValue(STR)),
		ScopedChoice(BonusChoiceKey(f.race.Key, 1), abilityValue(INT)),
	}
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
// subrace's own, then the picked options; the replaced trait offers no choice, its option is not honored,
// and the subrace's bonuses stand alone.
func TestASubraceReplacesTraitsAndBonusesAndKeepsItsOwn(t *testing.T) {
	t.Parallel()
	f := newReplaceFixture(t, true, true)
	b := f.build(true, append([]string{f.keptB, f.replacedX}, f.bonusPicks()...)...)
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
	if scoreBonus(d, STR) != 0 || scoreBonus(d, INT) != 0 || hasIssue(d, IssueRaceBonus) {
		t.Errorf("the race's placed bonuses apply or are asked for: STR %d INT %d, issues %v", scoreBonus(d, STR), scoreBonus(d, INT), issueCodes(d))
	}
	if n := f.c.FreeAbilityPoints(b); n != 0 {
		t.Errorf("free ability points = %d, want 0: the race's placed bonuses are gone", n)
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
	if wantOut := append([]string{f.replacedX}, f.bonusPicks()...); !slices.Equal(slices.Sorted(slices.Values(s.NotOffered)), slices.Sorted(slices.Values(wantOut))) {
		t.Errorf("not offered = %v, want %v", s.NotOffered, wantOut)
	}
	p := s.ChoiceProblems(false)
	var gotKeys []string
	for _, q := range p {
		if q.Code != ChoiceProblemNotOffered {
			t.Errorf("problem %+v, want code %s", q, ChoiceProblemNotOffered)
		}
		gotKeys = append(gotKeys, q.OptionKey)
	}
	if wantKeys := append([]string{f.replacedX}, f.bonusPicks()...); !slices.Equal(slices.Sorted(slices.Values(gotKeys)), slices.Sorted(slices.Values(wantKeys))) {
		t.Errorf("not offered keys = %v, want %v", gotKeys, wantKeys)
	}
}

// TestReplacingTraitsWithoutTheFlagKeepsTheBonusesAdditive: the bonuses of the race and of the subrace add up.
func TestReplacingTraitsWithoutTheFlagKeepsTheBonusesAdditive(t *testing.T) {
	t.Parallel()
	f := newReplaceFixture(t, true, false)
	b := f.build(true, f.bonusPicks()...)
	d := Derive(b, f.c)
	if scoreBonus(d, CON) != 1 || scoreBonus(d, DEX) != 2 || scoreBonus(d, STR) != 2 || scoreBonus(d, INT) != 1 {
		t.Errorf("bonuses CON %d DEX %d STR %d INT %d, want 1, 2, 2 and 1", scoreBonus(d, CON), scoreBonus(d, DEX), scoreBonus(d, STR), scoreBonus(d, INT))
	}
	if n := len(f.c.Choices(b).NotOffered); n != 0 {
		t.Errorf("%d picks not offered without the flag, want none", n)
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

// TestASubraceThatReplacesTheBonusesDropsTheHalfElfsPicks: the half-elf's +1 to two abilities go with
// the race's bonuses; without the flag they stay.
func TestASubraceThatReplacesTheBonusesDropsTheHalfElfsPicks(t *testing.T) {
	t.Parallel()
	_, sub, _ := tableMisc()
	sub.Race, sub.AbilityBonuses = "race:half-elf", map[Ability]int{WIS: 1}
	key := AbilityChoiceKey("race:half-elf")
	picks := []string{ScopedChoice(key, abilityValue(DEX)), ScopedChoice(key, abilityValue(INT))}
	build := func() Build {
		return choiceBuild("race:half-elf", sub.Key, ClassLevel{Class: "class:fighter", Level: 1}, picks...)
	}
	run := func(replaces bool) (Derived, ChoiceSet) {
		sub.ReplacesAbilityBonuses = replaces
		c := withOverlay(t, Overlay{Subraces: []TableSubrace{sub}})
		return Derive(build(), c), c.Choices(build())
	}
	d, s := run(true)
	if scoreBonus(d, CHA) != 0 || scoreBonus(d, WIS) != 1 || scoreBonus(d, DEX) != 0 || scoreBonus(d, INT) != 0 {
		t.Errorf("bonuses CHA %d WIS %d DEX %d INT %d, want the subrace's WIS +1 alone", scoreBonus(d, CHA), scoreBonus(d, WIS), scoreBonus(d, DEX), scoreBonus(d, INT))
	}
	if slices.ContainsFunc(s.Groups, func(g ChoiceGroup) bool {
		return slices.ContainsFunc(g.Choices, func(ch Choice) bool { return ch.Key == key })
	}) || !slices.Equal(slices.Sorted(slices.Values(s.NotOffered)), slices.Sorted(slices.Values(picks))) {
		t.Errorf("the abilities are offered, or not offered = %v, want %v", s.NotOffered, picks)
	}
	if slices.ContainsFunc(d.Hints, func(h Hint) bool { return h.Source == "race:half-elf" }) {
		t.Errorf("a hint asks for the half-elf's picks: %v", d.Hints)
	}

	d, s = run(false)
	if scoreBonus(d, CHA) != 2 || scoreBonus(d, WIS) != 1 || scoreBonus(d, DEX) != 1 || scoreBonus(d, INT) != 1 || len(s.NotOffered) != 0 {
		t.Errorf("without the flag: CHA %d WIS %d DEX %d INT %d, not offered %v, want them all", scoreBonus(d, CHA), scoreBonus(d, WIS), scoreBonus(d, DEX), scoreBonus(d, INT), s.NotOffered)
	}
}

// TestAReplacingSubraceAsksForNoPlacedBonusesWhenNoneArePicked: the race's placed bonuses are gone, so a sheet
// with no picks for them is complete; without the flag the same sheet is told to place them.
func TestAReplacingSubraceAsksForNoPlacedBonusesWhenNoneArePicked(t *testing.T) {
	t.Parallel()
	with := newReplaceFixture(t, false, true)
	if d := Derive(with.build(true), with.c); hasIssue(d, IssueRaceBonus) {
		t.Errorf("issues = %v, want no race bonus issue", issueCodes(d))
	}
	without := newReplaceFixture(t, false, false)
	if d := Derive(without.build(true), without.c); !hasIssue(d, IssueRaceBonus) {
		t.Errorf("issues = %v, want a race bonus issue without the flag", issueCodes(d))
	}
}

// TestTheCatalogSaysWhatASubraceReplaces: the subrace entry names the replaced traits, the race's skills that go
// with them (Keen Senses' Perception) and the swap of the bonuses.
func TestTheCatalogSaysWhatASubraceReplaces(t *testing.T) {
	t.Parallel()
	_, sub, _ := tableMisc()
	sub.Race, sub.ReplacesTraitKeys, sub.ReplacesAbilityBonuses = "race:elf", []string{"trait:keen-senses"}, true
	c := withOverlay(t, Overlay{Subraces: []TableSubrace{sub}})
	var got SubraceEntry
	for _, e := range c.Catalog().Subraces {
		if e.Key == sub.Key {
			got = e
		}
	}
	if !slices.Equal(got.ReplacesTraitKeys, sub.ReplacesTraitKeys) || !got.ReplacesAbilityBonuses {
		t.Errorf("entry = %+v, want Keen Senses replaced and the bonuses swapped", got)
	}
	if !slices.Equal(got.ReplacedSkillProficiencies, []string{"skill:perception"}) {
		t.Errorf("removed skills = %v, want Perception", got.ReplacedSkillProficiencies)
	}
	sub.ReplacesTraitKeys, sub.ReplacesAbilityBonuses = nil, false
	for _, e := range withOverlay(t, Overlay{Subraces: []TableSubrace{sub}}).Catalog().Subraces {
		if e.Key == sub.Key && (len(e.ReplacesTraitKeys) != 0 || len(e.ReplacedSkillProficiencies) != 0 || e.ReplacesAbilityBonuses) {
			t.Errorf("entry = %+v, want no replacement", e)
		}
	}
}
