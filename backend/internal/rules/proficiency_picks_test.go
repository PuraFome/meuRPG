package rules

import (
	"slices"
	"strings"
	"testing"
)

// The language and tool picks of a race, a trait, a background and a feature (the half-elf's extra language, the
// Acolyte's two, the dwarf's artisan's tools, a background of the table that gives one type of artisan's tools) are
// choices of the "Escolhas" step, like a feat or an ability: written "<owner>#language=<language key>" and
// "<owner>#tool=<proficiency key>", and shown on the sheet's languages and proficiencies.

func pickBuild(race, subrace string, picks ...string) Build {
	b := choiceBuild(race, subrace, ClassLevel{Class: "class:fighter", Level: 1}, picks...)
	b.Languages = nil // the free field stays empty: only the picks answer
	return b
}

// picksPending is how many language and tool picks the set still asks: the half-elf's abilities and the fighting
// style are other choices.
func picksPending(s ChoiceSet) int {
	n := 0
	for _, p := range s.Pending() {
		if strings.HasSuffix(p.ChoiceKey, languagePickSuffix) || strings.HasSuffix(p.ChoiceKey, toolPickSuffix) {
			n += p.Missing
		}
	}
	return n
}

func languageKeys(d Derived) []string {
	var out []string
	for _, l := range d.Languages {
		out = append(out, l.Key)
	}
	return out
}

func pickOption(t *testing.T, ch Choice, key string) ChoiceOption {
	t.Helper()
	for _, o := range ch.Options {
		if o.Key == key {
			return o
		}
	}
	t.Fatalf("no option %q in %q", key, ch.Key)
	return ChoiceOption{}
}

func TestLanguagePicksAreChoicesOfTheRaceAndTheBackground(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	half, acolyte := LanguagePickKey("race:half-elf"), LanguagePickKey("background:acolyte")

	s := c.Choices(pickBuild("race:half-elf", ""))
	race, bg := choiceOf(t, s, half), choiceOf(t, s, acolyte)
	if race.Picks != 1 || bg.Picks != 2 || race.Kind != ChoiceKindOptions {
		t.Fatalf("picks %d and %d, kind %v; want 1, 2, options", race.Picks, bg.Picks, race.Kind)
	}
	if race.LabelPT != "Idioma (Meio-elfo)" || bg.LabelPT != "Idiomas (Acólito)" {
		t.Errorf("labels %q and %q", race.LabelPT, bg.LabelPT)
	}
	if len(race.Options) != len(c.c.languages) {
		t.Errorf("%d options, want every language of the catalog (%d)", len(race.Options), len(c.c.languages))
	}
	for _, known := range []string{"language:common", "language:elvish"} {
		o := pickOption(t, race, known)
		if !o.Blocked() || o.ReasonPT != "Você já fala este idioma." {
			t.Errorf("%s: reason %q, want that the character already speaks it", known, o.ReasonPT)
		}
	}
	if pickOption(t, race, "language:dwarvish").Blocked() {
		t.Error("Dwarvish is blocked, but the half-elf does not speak it")
	}
	if got := picksPending(s); got != 3 {
		t.Errorf("pending = %d, want the 3 languages", got)
	}
}

func TestLanguagePicksShowOnTheSheet(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	b := pickBuild("race:half-elf", "",
		ScopedChoice(LanguagePickKey("race:half-elf"), "language:dwarvish"),
		ScopedChoice(LanguagePickKey("background:acolyte"), "language:giant"),
		ScopedChoice(LanguagePickKey("background:acolyte"), "language:goblin"))
	d := Derive(b, c)
	want := []string{"language:common", "language:elvish", "language:dwarvish", "language:giant", "language:goblin"}
	if got := languageKeys(d); !slices.Equal(got, want) {
		t.Errorf("languages = %v, want %v", got, want)
	}
	if got := openOf(d, OpenChoiceLanguages); got != 0 {
		t.Errorf("open languages = %d, want 0 (%v)", got, d.Issues)
	}
	for _, is := range d.Issues {
		if is.Code == IssueMissing {
			t.Errorf("issue %+v, want none", is)
		}
	}
	s := c.Choices(b)
	if got := picksPending(s); got != 0 || len(s.NotOffered) != 0 {
		t.Errorf("pending %d, not offered %v", got, s.NotOffered)
	}
	for _, p := range s.ChoiceProblems(false) {
		t.Errorf("problem %+v", p)
	}
}

func TestALanguagePickIsNotOneTheCharacterAlreadyHas(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	half, acolyte := LanguagePickKey("race:half-elf"), LanguagePickKey("background:acolyte")

	t.Run("a language of the race", func(t *testing.T) {
		t.Parallel()
		b := pickBuild("race:half-elf", "", ScopedChoice(half, "language:elvish"))
		problems := c.Choices(b).ChoiceProblems(false)
		if len(problems) != 1 || problems[0].Code != ChoiceProblemPrerequisite || problems[0].OptionKey != "language:elvish" {
			t.Errorf("problems = %+v, want the prerequisite of Elvish", problems)
		}
	})
	t.Run("a language another pick took", func(t *testing.T) {
		t.Parallel()
		b := pickBuild("race:half-elf", "", ScopedChoice(half, "language:dwarvish"), ScopedChoice(acolyte, "language:dwarvish"))
		s := c.Choices(b)
		bg := choiceOf(t, s, acolyte)
		if o := pickOption(t, bg, "language:dwarvish"); !o.Blocked() {
			t.Error("the Acolyte's list offers the Dwarvish the race took")
		}
		if len(bg.Unmet) != 1 {
			t.Errorf("unmet = %v, want the repeated Dwarvish", bg.Unmet)
		}
		if got := languageKeys(Derive(b, c)); strings.Count(strings.Join(got, ","), "dwarvish") != 1 {
			t.Errorf("languages = %v, want Dwarvish once", got)
		}
	})
	t.Run("a pick past the number the owner gives overflows", func(t *testing.T) {
		t.Parallel()
		b := pickBuild("race:half-elf", "", ScopedChoice(half, "language:dwarvish"), ScopedChoice(half, "language:giant"))
		ch := choiceOf(t, c.Choices(b), half)
		if len(ch.Picked) != 1 || len(ch.Overflow) != 1 {
			t.Errorf("picked %v, overflow %v", ch.Picked, ch.Overflow)
		}
		if got := languageKeys(Derive(b, c)); slices.Contains(got, "language:giant") {
			t.Errorf("languages = %v, the second pick of a single-pick race counts", got)
		}
	})
	t.Run("a pick of an owner the sheet does not have is not offered", func(t *testing.T) {
		t.Parallel()
		b := pickBuild("race:half-elf", "", ScopedChoice(LanguagePickKey("race:human"), "language:giant"))
		if got := c.Choices(b).NotOffered; len(got) != 1 {
			t.Errorf("not offered = %v, want the pick of the human's language", got)
		}
		if got := languageKeys(Derive(b, c)); slices.Contains(got, "language:giant") {
			t.Errorf("languages = %v, a pick of an owner the sheet lacks counts", got)
		}
	})
}

func TestTheDwarfsToolPickIsAChoice(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	key := ToolPickKey("trait:tool-proficiency")
	b := pickBuild("race:dwarf", "subrace:hill-dwarf")
	b.Languages = []string{"Gnômico", "Orc", "Dracônico"} // the Acolyte's two and a language of the dwarf's

	s := c.Choices(b)
	ch := choiceOf(t, s, key)
	if ch.Picks != 1 || len(ch.Options) != 3 || ch.LabelPT != "Ferramenta (Anão)" {
		t.Fatalf("picks %d, %d options, label %q; want 1, 3 and the dwarf's", ch.Picks, len(ch.Options), ch.LabelPT)
	}
	d := Derive(b, c)
	if got := openOf(d, OpenChoiceTools); got != 1 || !hasIssueAt(d, IssueMissing, choiceKeysField) {
		t.Fatalf("open tools %d, issues %v; want 1 and an issue that points at the choices", got, d.Issues)
	}
	var msg string
	for _, is := range d.Issues {
		if is.Field == choiceKeysField {
			msg = is.Message
		}
	}
	if !strings.Contains(msg, "Escolhas") {
		t.Errorf("message %q does not say where to choose", msg)
	}

	b.FeatureChoices = []string{ScopedChoice(key, "proficiency:brewers-supplies")}
	d = Derive(b, c)
	if got := openOf(d, OpenChoiceTools); got != 0 || hasIssueAt(d, IssueMissing, choiceKeysField) {
		t.Errorf("open tools %d, issues %v; want none", got, d.Issues)
	}
	if !slices.ContainsFunc(d.Proficiencies, func(p Proficiency) bool { return p.Key == "proficiency:brewers-supplies" && p.Kind == "tool" }) {
		t.Errorf("proficiencies = %v, want the brewer's supplies", d.Proficiencies)
	}

	b.FeatureChoices = []string{ScopedChoice(key, "proficiency:thieves-tools")}
	if got := c.Choices(b).NotOffered; len(got) != 1 {
		t.Errorf("not offered = %v, want a tool the trait does not list", got)
	}
}

// A background of the table that gives one type of artisan's tools ("Ferreiro da vila", written for the tests).
func TestABackgroundOfTheTableGivesAToolPick(t *testing.T) {
	t.Parallel()
	artisan := []string{"proficiency:smiths-tools", "proficiency:masons-tools", "proficiency:carpenters-tools"}
	bg := TableBackground{
		TableEntry: TableEntry{Key: "background:ferreiro-da-vila" + tableSuffix, NamePT: "Ferreiro da vila"},
		Skills:     []string{"skill:athletics", "skill:history"},
		Feature: TableFeature{
			Key: "background-feature:forja-aberta" + tableSuffix, NamePT: "Forja aberta",
			DescPT:  []string{"Texto de teste: a vila empresta a forja."},
			Effects: []Effect{{Type: "choice", Choice: "tool", Count: 1, From: artisan}},
		},
	}
	c := withOverlayOn(t, loadForTest(t), Overlay{Revision: 1, Backgrounds: []TableBackground{bg}})
	b := pickBuild("race:human", "")
	b.Background = bg.Key
	b.Languages = []string{"Anão"}
	key := ToolPickKey(bg.Feature.Key)

	s := c.Choices(b)
	ch := choiceOf(t, s, key)
	if !slices.ContainsFunc(s.Groups, func(g ChoiceGroup) bool {
		return g.Origin == ChoiceOriginBackground && slices.ContainsFunc(g.Choices, func(x Choice) bool { return x.Key == key })
	}) {
		t.Error("the tool pick is not in the background's group")
	}
	if len(ch.Options) != len(artisan) || ch.Picks != 1 {
		t.Fatalf("%d options for %d picks, want the %d artisan's tools for 1", len(ch.Options), ch.Picks, len(artisan))
	}
	if d := Derive(b, c); openOf(d, OpenChoiceTools) != 1 {
		t.Errorf("open tools = %d, want 1", openOf(d, OpenChoiceTools))
	}
	b.FeatureChoices = []string{ScopedChoice(key, "proficiency:masons-tools")}
	d := Derive(b, c)
	if openOf(d, OpenChoiceTools) != 0 || !slices.ContainsFunc(d.Proficiencies, func(p Proficiency) bool { return p.Key == "proficiency:masons-tools" }) {
		t.Errorf("open %d, proficiencies %v; want the mason's tools and no pick open", openOf(d, OpenChoiceTools), d.Proficiencies)
	}
}

// A tool the class or the background already gives is not one to pick again.
func TestAToolPickIsNotOneTheCharacterAlreadyHas(t *testing.T) {
	t.Parallel()
	bg := TableBackground{
		TableEntry: TableEntry{Key: "background:ferreiro-da-vila" + tableSuffix, NamePT: "Ferreiro da vila"},
		Skills:     []string{"skill:athletics", "skill:history"}, Tools: []string{"proficiency:smiths-tools"},
		Feature: TableFeature{
			Key: "background-feature:forja-aberta" + tableSuffix, NamePT: "Forja aberta",
			Effects: []Effect{{Type: "choice", Choice: "tool", Count: 1, From: []string{"proficiency:smiths-tools", "proficiency:masons-tools"}}},
		},
	}
	c := withOverlayOn(t, loadForTest(t), Overlay{Revision: 1, Backgrounds: []TableBackground{bg}})
	b := pickBuild("race:human", "")
	b.Background = bg.Key
	ch := choiceOf(t, c.Choices(b), ToolPickKey(bg.Feature.Key))
	if o := pickOption(t, ch, "proficiency:smiths-tools"); !o.Blocked() || o.ReasonPT != "Você já tem esta proficiência." {
		t.Errorf("smith's tools: reason %q", o.ReasonPT)
	}
	if pickOption(t, ch, "proficiency:masons-tools").Blocked() {
		t.Error("mason's tools are blocked")
	}
}

// A sheet that listed its languages in the free field before they were picked keeps them: they answer the picks the
// character is owed, and the sheet's languages show them.
func TestTheFreeLanguagesStayAndAnswerThePicksOwed(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	b := choiceBuild("race:half-elf", "", ClassLevel{Class: "class:fighter", Level: 1}) // testLanguages in the free field
	d := Derive(b, c)
	names := map[string]bool{}
	for _, l := range d.Languages {
		names[l.NamePT] = true
	}
	for _, typed := range testLanguages {
		if !names[typed] && typed != "Élfico" {
			t.Errorf("the typed %q is not on the sheet's languages (%v)", typed, d.Languages)
		}
	}
	if len(d.Languages) != 4 { // Common, Elvish, and the typed Anão and Gnômico (Élfico repeats the race's)
		t.Errorf("languages = %v, want four", d.Languages)
	}
	if got := openOf(d, OpenChoiceLanguages); got != 0 {
		t.Errorf("open languages = %d with three typed, want 0", got)
	}
	if got := picksPending(c.Choices(b)); got != 0 {
		t.Errorf("pending = %d, want 0: the typed languages answer the picks", got)
	}

	b.Languages = []string{"Anão"}
	d = Derive(b, c)
	if got := openOf(d, OpenChoiceLanguages); got != 2 {
		t.Errorf("open languages = %d with one typed of three owed, want 2", got)
	}
	b.FeatureChoices = []string{ScopedChoice(LanguagePickKey("race:half-elf"), "language:giant")}
	if got := openOf(Derive(b, c), OpenChoiceLanguages); got != 1 {
		t.Errorf("open languages = %d with one picked and one typed, want 1", got)
	}
}

func TestTheFreeToolsShowOnTheSheet(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	b := pickBuild("race:dwarf", "subrace:hill-dwarf")
	b.ToolProficiencies = []string{"Ferramentas de ferreiro"}
	d := Derive(b, c)
	if !slices.ContainsFunc(d.Proficiencies, func(p Proficiency) bool { return p.NamePT == "Ferramentas de ferreiro" && p.Kind == "tool" }) {
		t.Errorf("proficiencies = %v, want the typed tool", d.Proficiencies)
	}
	if got := openOf(d, OpenChoiceTools); got != 0 {
		t.Errorf("open tools = %d with one typed, want 0", got)
	}
}

// The picks only warn while they are open: pending, counted, marked, but no problem for the save and none asked
// again at a level-up, as before they were choices (RN-33: "These are warnings; they do not stop the sheet").
func TestOpenLanguageAndToolPicksWarnAndDoNotStopTheSheet(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	b := pickBuild("race:dwarf", "subrace:hill-dwarf")
	s := c.Choices(b)
	for _, key := range []string{ToolPickKey("trait:tool-proficiency"), LanguagePickKey("background:acolyte")} {
		ch := choiceOf(t, s, key)
		if !ch.Advisory || ch.Missing() == 0 {
			t.Errorf("%s: advisory %v, missing %d; want advisory and open", key, ch.Advisory, ch.Missing())
		}
	}
	if got := picksPending(s); got != 3 {
		t.Errorf("pending picks = %d, want 3 (two languages and the tool)", got)
	}
	for _, p := range s.ChoiceProblems(true) {
		if strings.HasSuffix(p.ChoiceKey, languagePickSuffix) || strings.HasSuffix(p.ChoiceKey, toolPickSuffix) {
			t.Errorf("problem %+v for an advisory pick", p)
		}
	}
	for _, g := range pendingGroups(s) {
		for _, ch := range g.Choices {
			if ch.Advisory {
				t.Errorf("a level-up asks the advisory %s again", ch.Key)
			}
		}
	}
	// A pick that breaks a rule is still a problem: a language the race already gives.
	b.FeatureChoices = []string{ScopedChoice(LanguagePickKey("background:acolyte"), "language:dwarvish")}
	if got := c.Choices(b).ChoiceProblems(false); len(got) != 1 || got[0].Code != ChoiceProblemPrerequisite {
		t.Errorf("problems = %+v, want the prerequisite of the Dwarvish the dwarf speaks", got)
	}
}
