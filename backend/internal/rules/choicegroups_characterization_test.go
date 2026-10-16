package rules

import (
	"encoding/json"
	"maps"
	"testing"
)

// The characterization test of the choice engine's builders split in E5 (race,
// features, featChoice, invocationPrerequisites, markFamilies) and of
// ChoiceSet.ChoiceProblems: the whole set each sheet of the corpus gets, and its
// problems with and without the open choices.

// choiceSetDump is a ChoiceSet and its problems.
func choiceSetDump(s ChoiceSet) (string, error) {
	out := struct {
		Set             ChoiceSet
		Problems        []ChoiceProblem
		ProblemsAndOpen []ChoiceProblem
	}{s, s.ChoiceProblems(false), s.ChoiceProblems(true)}
	raw, err := json.Marshal(out)
	return string(raw), err
}

// choiceSetCase is a sheet and the content it is read with.
type choiceSetCase struct {
	c *Content
	b Build
}

// choiceSetWithout is the SRD without the given traits and features, as a
// content whose race or class names a key it lacks.
func choiceSetWithout(srd *Content, traits, features []string) *Content {
	c := *srd.c
	c.traits = maps.Clone(srd.c.traits)
	c.features = maps.Clone(srd.c.features)
	for _, k := range traits {
		delete(c.traits, k)
	}
	for _, k := range features {
		delete(c.features, k)
	}
	return &Content{c: &c}
}

func choiceSetCases(t *testing.T) map[string]choiceSetCase {
	t.Helper()
	srd := loadForTest(t)
	cl := func(class, sub string, level int) ClassLevel {
		return ClassLevel{Class: class, Subclass: sub, Level: level}
	}
	fighter := cl("class:fighter", "", 1)
	enemy1, enemy2 := "feature:favored-enemy-1-type", "feature:favored-enemy-2-types"
	terrain1, terrain2 := "feature:natural-explorer-1-terrain-type", "feature:natural-explorer-2-terrain-types"
	inv := func(k string) string { return "feature:eldritch-invocation-" + k }
	cases := map[string]choiceSetCase{
		"no race":                      {srd, choiceBuild("", "", fighter)},
		"an unknown race":              {srd, choiceBuild("race:nope", "", fighter)},
		"a dragonborn, no ancestry":    {srd, choiceBuild("race:dragonborn", "", fighter)},
		"a dragonborn, red":            {srd, choiceBuild("race:dragonborn", "", fighter, "trait:draconic-ancestry-red")},
		"a dragonborn, two ancestries": {srd, choiceBuild("race:dragonborn", "", fighter, "trait:draconic-ancestry-red", "trait:draconic-ancestry-blue")},
		"a high elf wizard":            {srd, choiceBuild("race:elf", "subrace:high-elf", cl("class:wizard", "", 1))},
		"a hill dwarf":                 {srd, choiceBuild("race:dwarf", "subrace:hill-dwarf", fighter)},
		"a half-elf with two abilities": {srd, choiceBuild("race:half-elf", "", fighter,
			ScopedChoice(AbilityChoiceKey("race:half-elf"), abilityValue(DEX)), ScopedChoice(AbilityChoiceKey("race:half-elf"), abilityValue(CHA)))},
		"a dragonborn whose ancestry trait is gone": {
			choiceSetWithout(srd, []string{"trait:draconic-ancestry"}, nil),
			choiceBuild("race:dragonborn", "", fighter, "trait:draconic-ancestry-red"),
		},
		"a fighter whose style feature is gone": {
			choiceSetWithout(srd, nil, []string{"feature:fighter-fighting-style"}),
			choiceBuild("race:human", "", fighter, "feature:fighter-fighting-style-defense"),
		},
		"a champion 10 with three styles": {srd, choiceBuild("race:human", "", cl("class:fighter", "subclass:champion", 10),
			"feature:fighter-fighting-style-defense", "feature:fighter-fighting-style-archery", "feature:fighter-fighting-style-dueling")},
		"a pick not offered": {srd, choiceBuild("race:human", "", fighter, "feature:nope", "feature:fighter-fighting-style-defense")},
		"a ranger 6 repeating an enemy and a terrain": {srd, choiceBuild("race:human", "", cl("class:ranger", "subclass:hunter", 6),
			ScopedChoice(enemy1, "creature-type:giant"), ScopedChoice(enemy2, "creature-type:giant"),
			ScopedChoice(terrain1, "terrain:forest"), ScopedChoice(terrain2, "terrain:forest"), ScopedChoice(terrain2, "terrain:swamp"))},
		"a ranger 14 with every enemy": {srd, choiceBuild("race:human", "", cl("class:ranger", "subclass:hunter", 14),
			ScopedChoice(enemy1, "creature-type:giant"), ScopedChoice(enemy2, "creature-type:undead"),
			ScopedChoice("feature:favored-enemy-3-enemies", "creature-type:beast"))},
		"a warlock 2 without a boon": {srd, choiceBuild("race:human", "", cl("class:warlock", "subclass:fiend", 2),
			inv("agonizing-blast"), inv("voice-of-the-chain-master"))},
		"a warlock 5 of the chain": {srd, choiceBuild("race:human", "", cl("class:warlock", "subclass:fiend", 5),
			"feature:pact-of-the-chain", inv("voice-of-the-chain-master"), inv("thirsting-blade"), inv("mire-the-mind"))},
		"a warlock 5 of the tome": {srd, choiceBuild("race:human", "", cl("class:warlock", "subclass:fiend", 5),
			"feature:pact-of-the-tome", inv("book-of-ancient-secrets"), inv("agonizing-blast"),
			ScopedChoice("feature:pact-of-the-tome", "spell:eldritch-blast"))},
		"a warlock 11 with an arcanum": {srd, choiceBuild("race:human", "", cl("class:warlock", "subclass:fiend", 11), "feature:pact-of-the-blade")},
		"a sorcerer 10 with metamagic": {srd, choiceBuild("race:human", "", cl("class:sorcerer", "subclass:draconic", 10),
			"feature:metamagic-careful-spell", "feature:metamagic-twinned-spell", "feature:metamagic-quickened-spell", "feature:metamagic-subtle-spell")},
		"a wizard 18":  {srd, choiceBuild("race:human", "", cl("class:wizard", "subclass:evocation", 18))},
		"a wizard 20":  {srd, choiceBuild("race:human", "", cl("class:wizard", "subclass:evocation", 20))},
		"a land druid": {srd, choiceBuild("race:human", "", cl("class:druid", "subclass:land", 3))},
	}
	b := choiceBuild("race:human", "", cl("class:warlock", "subclass:fiend", 3), "feature:pact-of-the-chain", inv("agonizing-blast"))
	b.Cantrips = []string{"spell:eldritch-blast"}
	cases["a warlock 3 who knows the blast"] = choiceSetCase{srd, b}
	two := choiceBuild("race:human", "", fighter)
	two.Classes = append(two.Classes, cl("class:warlock", "", 2))
	two.FeatureChoices = []string{inv("agonizing-blast")}
	cases["a fighter and warlock"] = choiceSetCase{srd, two}
	// An invocation the prerequisites file has no row for asks nothing.
	noRow := *srd.c
	data := *srd.c.choices
	data.prerequisites = maps.Clone(srd.c.choices.prerequisites)
	delete(data.prerequisites, inv("agonizing-blast"))
	noRow.choices = &data
	cases["an invocation without a prerequisites row"] = choiceSetCase{
		&Content{c: &noRow},
		choiceBuild("race:human", "", cl("class:warlock", "", 2), inv("agonizing-blast")),
	}

	// The feat choices of a table race.
	feats := func(name string, from []string, edit func(b *Build), picks ...string) {
		c, race := featChoiceTable(t, from...)
		fb := featChoiceBuild(race, picks...)
		if edit != nil {
			edit(&fb)
		}
		cases["feat: "+name] = choiceSetCase{c, fb}
	}
	feats("any, none picked", nil, nil)
	feats("any, one picked", nil, nil, alertaFeat)
	feats("from a list", []string{alertaFeat, punhoFeat}, nil, alertaFeat)
	feats("an unmet prerequisite picked", nil, func(b *Build) { b.BaseScores[STR] = 10 }, punhoFeat)
	feats("one already on the sheet", nil, func(b *Build) { b.Feats = []string{livreFeat} }, alertaFeat)
	feats("two picks for one", nil, nil, alertaFeat, livreFeat)
	// Two feats from one trait, archived and switched-off feats.
	_, race := featChoiceTable(t)
	race.Traits[len(race.Traits)-1].Effects[0].Count = 2
	feat := func(key, name string, archived bool) TableFeat {
		return TableFeat{TableEntry: TableEntry{Key: key, NamePT: name, Archived: archived}, DescPT: []string{"Texto de " + name + "."}}
	}
	c2 := withOverlayOn(t, loadForTest(t), Overlay{Revision: 1, Races: []TableRace{race}, Off: []string{livreFeat}, Feats: []TableFeat{
		feat(alertaFeat, "Alerta", false), feat(punhoFeat, "Punho", true), feat(livreFeat, "Livre", false),
	}})
	cases["feat: two of them, one archived, one off"] = choiceSetCase{c2, featChoiceBuild(race)}
	cases["feat: the archived and the off one picked"] = choiceSetCase{c2, featChoiceBuild(race, punhoFeat, livreFeat)}
	return cases
}

// TestChoiceSetCharacterization freezes the choice set and the problems of
// each sheet of the corpus.
func TestChoiceSetCharacterization(t *testing.T) {
	t.Parallel()
	got := map[string]string{}
	for name, tc := range choiceSetCases(t) {
		text, err := choiceSetDump(tc.c.Choices(tc.b))
		if err != nil {
			t.Fatal(err)
		}
		got[name] = text
	}
	goldenText(t, "choicegroups-sets", got)
}

// TestChoiceProblemsCharacterization freezes the order ChoiceProblems lists a
// hand-made set in: every unmet pick first, then the picks not offered, then the
// overflow of each choice, then (asked for) the open choices.
func TestChoiceProblemsCharacterization(t *testing.T) {
	t.Parallel()
	ch := func(key string, picks int, picked, unmet, overflow []string) Choice {
		return Choice{Key: key, LabelPT: "Rótulo " + key, Picks: picks, Picked: picked, Unmet: unmet, Overflow: overflow}
	}
	s := ChoiceSet{
		Groups: []ChoiceGroup{
			{Choices: []Choice{
				ch("a", 2, []string{"a1"}, []string{"a1"}, []string{"a9"}),
				ch("b", 1, nil, nil, nil),
			}},
			{Choices: []Choice{
				ch("c", 1, []string{"c1"}, []string{"c1", "c2"}, []string{"c8", "c9"}),
				ch("d", 1, []string{"d1"}, nil, nil),
			}},
		},
		NotOffered: []string{"x1", "x2"},
	}
	got := map[string]string{}
	for name, set := range map[string]ChoiceSet{"empty": {}, "every kind": s} {
		text, err := choiceSetDump(set)
		if err != nil {
			t.Fatal(err)
		}
		got[name] = text
	}
	goldenText(t, "choicegroups-problems", got)
}
