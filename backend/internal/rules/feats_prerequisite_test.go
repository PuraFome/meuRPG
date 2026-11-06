package rules

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// The richer prerequisites of a table feat: any of several races, a size and a feature the
// character must not have.

const (
	featKeyElfOrDwarf = "feat:elfo-ou-anao" + tableSuffix
	featKeyHighElf    = "feat:alto-elfo" + tableSuffix
	featKeyOldRace    = "feat:so-elfo" + tableSuffix
	featKeySmall      = "feat:pequeno" + tableSuffix
	featKeyNoDark     = "feat:sem-visao" + tableSuffix
	featKeyNoGrappler = "feat:sem-agarrador" + tableSuffix
	featKeyAll        = "feat:tudo" + tableSuffix
)

// richPrerequisiteFeats is a table with a feat for each new condition, and a table race
// that lets the player pick Small or Medium.
func richPrerequisiteFeats(t *testing.T) *Content {
	t.Helper()
	feat := func(key, name string, p FeatPrerequisite) TableFeat {
		return TableFeat{TableEntry: TableEntry{Key: key, NamePT: name}, DescPT: []string{"Texto."}, Prerequisite: p}
	}
	race, sub, _ := tableMisc()
	race.Sizes = []string{"Small", "Medium"}
	c, err := loadForTest(t).With(Overlay{
		Revision: 1, Races: []TableRace{race}, Subraces: []TableSubrace{sub},
		Feats: []TableFeat{
			feat(featKeyElfOrDwarf, "Elfo ou anão", FeatPrerequisite{Races: []string{"race:elf", "race:dwarf"}}),
			feat(featKeyHighElf, "Alto elfo", FeatPrerequisite{Races: []string{"subrace:high-elf", "subrace:da-colina-nevoenta" + tableSuffix}}),
			feat(featKeyOldRace, "Só elfo", FeatPrerequisite{Race: "race:elf"}),
			feat(featKeySmall, "Pequeno", FeatPrerequisite{Size: "Small"}),
			feat(featKeyNoDark, "Sem visão no escuro", FeatPrerequisite{LacksFeature: "trait:darkvision"}),
			feat(featKeyNoGrappler, "Sem agarrador", FeatPrerequisite{LacksFeature: "feat:grappler"}),
			feat(featKeyAll, "Tudo", FeatPrerequisite{Races: []string{"race:elf", "race:halfling"}, Size: "Medium", LacksFeature: "feat:grappler", Level: 2}),
		},
	})
	if err != nil {
		t.Fatalf("With() error = %v", err)
	}
	return c
}

// unmetOf is what the build lacks for the feat.
func unmetOf(t *testing.T, c *Content, b Build, key string) []FeatUnmet {
	t.Helper()
	unmet, err := CheckFeat(b, key, c)
	if err != nil {
		t.Fatalf("CheckFeat(%s) error = %v", key, err)
	}
	return unmet
}

// TestFeatPrerequisitesOfRacesSizeAndLackedFeatures: each condition is judged on the derived sheet and says
// what failed; a subrace satisfies its parent race's key and its own.
func TestFeatPrerequisitesOfRacesSizeAndLackedFeatures(t *testing.T) {
	t.Parallel()
	c := richPrerequisiteFeats(t)
	human := torenLevelUp()
	highElf := torenLevelUp()
	highElf.Race, highElf.Subrace = "race:elf", "subrace:high-elf"
	plainElf := torenLevelUp()
	plainElf.Race = "race:elf"
	halfling := torenLevelUp()
	halfling.Race = "race:halfling"
	grappler := torenLevelUp()
	grappler.Feats = []string{"feat:grappler"}
	mist := torenLevelUp()
	mist.Race, mist.Subrace = "race:anao-das-brumas"+tableSuffix, "subrace:da-colina-nevoenta"+tableSuffix
	small := mist
	small.Size = "Small"

	cases := []struct {
		name string
		b    Build
		feat string
		want []FeatUnmet
	}{
		{"any of: a human is neither", human, featKeyElfOrDwarf, []FeatUnmet{{Kind: FeatUnmetRace, Keys: []string{"race:elf", "race:dwarf"}}}},
		{"any of: an elf", plainElf, featKeyElfOrDwarf, nil},
		{"any of: a high elf is an elf too", highElf, featKeyElfOrDwarf, nil},
		{"any of subraces: a high elf", highElf, featKeyHighElf, nil},
		{"any of subraces: an elf with no subrace", plainElf, featKeyHighElf, []FeatUnmet{{Kind: FeatUnmetRace, Keys: []string{"subrace:high-elf", "subrace:da-colina-nevoenta" + tableSuffix}}}},
		{"any of subraces: the table subrace", mist, featKeyHighElf, nil},
		{"the single race_key still works", highElf, featKeyOldRace, nil},
		{"the single race_key names the one race", human, featKeyOldRace, []FeatUnmet{{Kind: FeatUnmetRace, Key: "race:elf"}}},
		{"size: a human is Medium", human, featKeySmall, []FeatUnmet{{Kind: FeatUnmetSize, Key: "Small"}}},
		{"size: a halfling is Small", halfling, featKeySmall, nil},
		{"size: the size the player picked", small, featKeySmall, nil},
		{"size: the table race's own size", mist, featKeySmall, []FeatUnmet{{Kind: FeatUnmetSize, Key: "Small"}}},
		{"lacks: a human has no darkvision", human, featKeyNoDark, nil},
		{"lacks: a high elf has it", highElf, featKeyNoDark, []FeatUnmet{{Kind: FeatUnmetLacksFeature, Key: "trait:darkvision"}}},
		{"lacks: a feat the character took", grappler, featKeyNoGrappler, []FeatUnmet{{Kind: FeatUnmetLacksFeature, Key: "feat:grappler"}}},
		{"lacks: a feat the character did not take", human, featKeyNoGrappler, nil},
		{"a human meets the size, the feature and the level, and fails only the races", human, featKeyAll, []FeatUnmet{{Kind: FeatUnmetRace, Keys: []string{"race:elf", "race:halfling"}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := unmetOf(t, c, tc.b, tc.feat)
			if !slices.EqualFunc(got, tc.want, func(a, b FeatUnmet) bool {
				return a.Kind == b.Kind && a.Key == b.Key && slices.Equal(a.Keys, b.Keys) && a.Value == b.Value
			}) {
				t.Errorf("unmet = %+v, want %+v", got, tc.want)
			}
		})
	}

	// Every condition together: a Medium elf with Grappler, level 3, fails only the feature.
	all := plainElf
	all.Feats = []string{"feat:grappler"}
	if got := unmetKinds(unmetOf(t, c, all, featKeyAll)); !slices.Equal(got, []string{FeatUnmetLacksFeature}) {
		t.Errorf("a Medium elf with Grappler: unmet %v, want only the feature it must lack", got)
	}
	// ...and the Portuguese sentences say what failed.
	mixed := unmetOf(t, c, human, featKeyAll)
	if text := c.c.featUnmetPT(mixed[0]); text != "Só para Elfo ou Halfling." {
		t.Errorf("race sentence = %q", text)
	}
	if text := c.c.featUnmetPT(FeatUnmet{Kind: FeatUnmetSize, Key: "Small"}); text != "Só para tamanho pequeno." {
		t.Errorf("size sentence = %q", text)
	}
	if text := c.c.featUnmetPT(FeatUnmet{Kind: FeatUnmetLacksFeature, Key: "trait:darkvision"}); !strings.HasPrefix(text, "Não pode ter ") || !strings.Contains(text, c.c.namePT("trait:darkvision")) {
		t.Errorf("feature sentence = %q", text)
	}
	if got := c.c.racesPT([]string{"race:elf", "race:dwarf", "race:halfling"}); got != "Elfo, Anão ou Halfling" {
		t.Errorf("three races = %q, want \"A, B ou C\"", got)
	}
	e, _ := c.Feat(featKeyAll)
	if got := c.c.featAsksPT(e.Prerequisite); got != "ser Elfo ou Halfling e ter tamanho médio e não ter "+c.c.namePT("feat:grappler")+" e nível 2 ou mais" {
		t.Errorf("what the feat asks = %q", got)
	}
}

// TestFeatOptionsAndLevelUpUseTheRichPrerequisites: the options list says what the character lacks and the level-up
// refuses a feat whose new condition is unmet.
func TestFeatOptionsAndLevelUpUseTheRichPrerequisites(t *testing.T) {
	t.Parallel()
	c := richPrerequisiteFeats(t)
	before := torenLevelUp() // a Medium human fighter 3
	opts := FeatOptions(before, c)
	for key, want := range map[string][]string{
		featKeyElfOrDwarf: {FeatUnmetRace}, featKeySmall: {FeatUnmetSize}, featKeyNoDark: nil, featKeyNoGrappler: nil,
	} {
		if got := unmetKinds(featOption(t, opts, key).Unmet); !slices.Equal(got, want) {
			t.Errorf("%s: unmet %v, want %v", key, got, want)
		}
	}
	avg := LevelUpHitPoints{Average: true}
	for _, key := range []string{featKeyElfOrDwarf, featKeySmall} {
		after := mustApply(t, c, before, LevelUpChoices{Class: "class:fighter", Feat: key, HitPoints: avg})
		wantRefusal(t, CheckLevelUp(before, after, c), LevelUpReasonFeatPrerequisite, "")
	}
	taken := mustApply(t, c, before, LevelUpChoices{Class: "class:fighter", Feat: featKeyNoDark, HitPoints: avg})
	if err := CheckLevelUp(before, taken, c); err != nil {
		t.Errorf("a feat whose lacked feature the character does not have: %v", err)
	}
	// The same feat is refused to a character that has the feature (a high elf has darkvision).
	elf := torenLevelUp()
	elf.Race, elf.Subrace = "race:elf", "subrace:high-elf"
	after := mustApply(t, c, elf, LevelUpChoices{Class: "class:fighter", Feat: featKeyNoDark, HitPoints: avg})
	wantRefusal(t, CheckLevelUp(elf, after, c), LevelUpReasonFeatPrerequisite, "")
}

// TestAFeatThatLacksAFeatureStopsWorkingWhenTheCharacterGainsIt: the feat stays listed and is not applied, as for any
// prerequisite the character no longer meets.
func TestAFeatThatLacksAFeatureStopsWorkingWhenTheCharacterGainsIt(t *testing.T) {
	t.Parallel()
	c := richPrerequisiteFeats(t)
	b := torenLevelUp()
	b.Feats = []string{featKeyNoGrappler}
	if d := Derive(b, c); slices.ContainsFunc(d.Issues, func(i Issue) bool { return i.Code == IssueFeatPrerequisite }) {
		t.Fatalf("issues = %v, want none while the character lacks Grappler", d.Issues)
	}
	b.Feats = []string{featKeyNoGrappler, "feat:grappler"}
	if d := Derive(b, c); !slices.ContainsFunc(d.Issues, func(i Issue) bool { return i.Code == IssueFeatPrerequisite }) {
		t.Errorf("issues = %v, want the feat prerequisite issue once Grappler is taken", d.Issues)
	}
}

// TestTableFeatRichPrerequisiteShape: the overlay accepts the new conditions and refuses each wrong one at its own field.
func TestTableFeatRichPrerequisiteShape(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)
	race, sub, bg := tableMisc()
	otherFeat := TableFeat{TableEntry: TableEntry{Key: "feat:outro" + tableSuffix, NamePT: "Outro"}}
	many := make([]string, maxEffectList+1)
	for i := range many {
		many[i] = "race:human"
	}
	cases := []struct {
		name       string
		p          FeatPrerequisite
		field, why string
	}{
		{name: "races of the SRD and the table", p: FeatPrerequisite{Races: []string{"race:elf", race.Key, "subrace:high-elf", sub.Key}}},
		{name: "a size", p: FeatPrerequisite{Size: "Large"}},
		{name: "a trait of the SRD", p: FeatPrerequisite{LacksFeature: "trait:darkvision"}},
		{name: "a trait of a table race", p: FeatPrerequisite{LacksFeature: race.Traits[0].Key}},
		{name: "a trait of a table subrace", p: FeatPrerequisite{LacksFeature: sub.Traits[0].Key}},
		{name: "a background feature of the table", p: FeatPrerequisite{LacksFeature: bg.Feature.Key}},
		{name: "a feat of the SRD", p: FeatPrerequisite{LacksFeature: "feat:grappler"}},
		{name: "a feat of the table", p: FeatPrerequisite{LacksFeature: otherFeat.Key}},
		{name: "every new condition with level", p: FeatPrerequisite{Races: []string{"race:elf"}, Size: "Medium", LacksFeature: "trait:darkvision", Level: 3}},
		{"race_key and races together", FeatPrerequisite{Race: "race:elf", Races: []string{"race:dwarf"}}, "feats[0].prerequisite.races", ReasonValue},
		{"an unknown race in the list", FeatPrerequisite{Races: []string{"race:elf", "race:nada"}}, "feats[0].prerequisite.races[1]", ReasonReference},
		{"a class in the list", FeatPrerequisite{Races: []string{"class:fighter"}}, "feats[0].prerequisite.races[0]", ReasonReference},
		{"a race listed twice", FeatPrerequisite{Races: []string{"race:elf", "race:dwarf", "race:elf"}}, "feats[0].prerequisite.races[2]", ReasonValue},
		{"too many races", FeatPrerequisite{Races: many}, "feats[0].prerequisite.races", ReasonLimit},
		{"a size that does not exist", FeatPrerequisite{Size: "Huge"}, "feats[0].prerequisite.size", ReasonValue},
		{"a size in lower case", FeatPrerequisite{Size: "small"}, "feats[0].prerequisite.size", ReasonValue},
		{"a feature that does not exist", FeatPrerequisite{LacksFeature: "trait:nada"}, "feats[0].prerequisite.lacks_feature_key", ReasonReference},
		{"a table feature that does not exist", FeatPrerequisite{LacksFeature: "feature:nada" + tableSuffix}, "feats[0].prerequisite.lacks_feature_key", ReasonReference},
		{"a race as the feature", FeatPrerequisite{LacksFeature: "race:elf"}, "feats[0].prerequisite.lacks_feature_key", ReasonReference},
		{"the feat itself", FeatPrerequisite{LacksFeature: "feat:teste" + tableSuffix}, "feats[0].prerequisite.lacks_feature_key", ReasonValue},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := TableFeat{TableEntry: TableEntry{Key: "feat:teste" + tableSuffix, NamePT: "Teste"}, Prerequisite: tc.p}
			// The table's entries come after the feat in the overlay and in key order, so a reference to one is
			// judged once all exist.
			_, err := srd.With(Overlay{Revision: 1, Feats: []TableFeat{f, otherFeat}, Races: []TableRace{race}, Subraces: []TableSubrace{sub}, Backgrounds: []TableBackground{bg}})
			if tc.field == "" {
				if err != nil {
					t.Fatalf("With() error = %v", err)
				}
				return
			}
			oe, ok := errors.AsType[*OverlayError](err)
			if !ok {
				t.Fatalf("With() error = %v, want an OverlayError at %s", err, tc.field)
			}
			if !slices.ContainsFunc(oe.Violations(), func(v *OverlayError) bool { return v.Field == tc.field && v.Reason == tc.why }) {
				t.Errorf("violations = %+v, want one at %s (%s)", oe.Violations(), tc.field, tc.why)
			}
		})
	}
}

// TestTableFeatRacesRefusalNamesBothFields: the refusal of race_key with races names both fields.
func TestTableFeatRacesRefusalNamesBothFields(t *testing.T) {
	t.Parallel()
	f := TableFeat{TableEntry: TableEntry{Key: "feat:teste" + tableSuffix, NamePT: "Teste"}, Prerequisite: FeatPrerequisite{Race: "race:elf", Races: []string{"race:dwarf"}}}
	_, err := loadForTest(t).With(Overlay{Revision: 1, Feats: []TableFeat{f}})
	if err == nil || !strings.Contains(err.Error(), "races") || !strings.Contains(err.Error(), "race_key") {
		t.Errorf("With() error = %v, want it to name races and race_key", err)
	}
}

// TestAFeatWithTheNewFieldsIsNotZero: IsZero sees each new condition.
func TestAFeatWithTheNewFieldsIsNotZero(t *testing.T) {
	t.Parallel()
	for name, p := range map[string]FeatPrerequisite{
		"races": {Races: []string{"race:elf"}}, "size": {Size: "Small"}, "lacks": {LacksFeature: "trait:darkvision"},
	} {
		if p.IsZero() {
			t.Errorf("%s: IsZero() = true", name)
		}
	}
}

// TestAFeatPickWithRacesAnyOfIsJudgedAtCharacterCreation: a feat a trait lets the player choose is blocked,
// with the reason in Portuguese, when the character is none of the races, and the pick is refused; a feat
// that lists the character's race is open.
func TestAFeatPickWithRacesAnyOfIsJudgedAtCharacterCreation(t *testing.T) {
	t.Parallel()
	_, race := featChoiceTable(t)
	const anyRace, mine = "feat:de-raca" + tableSuffix, "feat:da-minha" + tableSuffix
	feat := func(key, name string, races ...string) TableFeat {
		return TableFeat{TableEntry: TableEntry{Key: key, NamePT: name}, DescPT: []string{"Texto."}, Prerequisite: FeatPrerequisite{Races: races}}
	}
	c := withOverlayOn(t, loadForTest(t), Overlay{Revision: 1, Races: []TableRace{race}, Feats: []TableFeat{
		feat(anyRace, "De raça", "race:elf", "race:dwarf"), feat(mine, "Da minha", "race:elf", race.Key),
	}})
	b := featChoiceBuild(race)
	options := map[string]ChoiceOption{}
	for _, o := range findFeatChoice(t, c.Choices(b)).Options {
		options[o.Key] = o
	}
	opt := options[anyRace]
	if !opt.Blocked() || !strings.Contains(opt.ReasonPT, "Só para Elfo ou Anão.") || !strings.Contains(opt.SummaryPT, "Pede: ser Elfo ou Anão.") {
		t.Fatalf("the feat should be shown and blocked, with the races in the reason and in what it asks: %+v", opt)
	}
	if options[mine].Blocked() {
		t.Errorf("a feat that lists the character's race is blocked: %+v", options[mine])
	}
	b.FeatureChoices = append(b.FeatureChoices, opt.Stored)
	problems := c.Choices(b).ChoiceProblems(true)
	if len(problems) == 0 || problems[0].Code != ChoiceProblemPrerequisite || problems[0].OptionKey != anyRace {
		t.Fatalf("problems = %+v, want a prerequisite refusal of the feat", problems)
	}
}
