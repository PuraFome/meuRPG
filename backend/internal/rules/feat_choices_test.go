package rules

import (
	"slices"
	"strings"
	"testing"
)

const (
	talentoTrait = "trait:talento" + tableSuffix
	alertaFeat   = "feat:alerta" + tableSuffix
	punhoFeat    = "feat:punho" + tableSuffix
	livreFeat    = "feat:livre" + tableSuffix
)

// featChoiceTable is a table with a race whose trait "Talento" grants a feat of the
// player's choice (from the given list; empty offers every feat) and three table
// feats: Alerta (Initiative +5, no prerequisite), Punho (Strength 15) and Livre.
func featChoiceTable(t *testing.T, from ...string) (*Content, TableRace) {
	t.Helper()
	race, _, _ := tableMisc()
	race.ChoiceBonuses = nil
	race.Traits = append(race.Traits, TableFeature{
		Key: talentoTrait, NamePT: "Talento", DescPT: []string{"Um talento à escolha."},
		Effects: []Effect{{Type: "choice", Choice: "feat", Count: 1, From: from}},
	})
	feat := func(key, name string, p FeatPrerequisite, effects ...Effect) TableFeat {
		return TableFeat{TableEntry: TableEntry{Key: key, NamePT: name}, DescPT: []string{"Texto de " + name + "."}, Prerequisite: p, Effects: effects}
	}
	c := withOverlayOn(t, loadForTest(t), Overlay{Revision: 1, Races: []TableRace{race}, Feats: []TableFeat{
		feat(alertaFeat, "Alerta", FeatPrerequisite{}, Effect{Type: "modifier", Target: "initiative", Mode: "add", Value: "5"}),
		feat(punhoFeat, "Punho", FeatPrerequisite{Minimums: map[Ability]int{STR: 15}}),
		feat(livreFeat, "Livre", FeatPrerequisite{}),
	}})
	return c, race
}

func featChoiceBuild(race TableRace, picks ...string) Build {
	b := torenLevelUp()
	b.Race = race.Key
	b.Classes[0].Level = 1
	b.Classes[0].Subclass = ""
	for _, p := range picks {
		b.FeatureChoices = append(b.FeatureChoices, ScopedChoice(FeatChoiceKey(talentoTrait), p))
	}
	return b
}

func findFeatChoice(t *testing.T, s ChoiceSet) Choice {
	t.Helper()
	for _, g := range s.Groups {
		for _, ch := range g.Choices {
			if ch.Key == FeatChoiceKey(talentoTrait) {
				return ch
			}
		}
	}
	t.Fatalf("no feat choice in %+v", s.Groups)
	return Choice{}
}

func choiceOptionKeys(ch Choice) []string {
	var out []string
	for _, o := range ch.Options {
		out = append(out, o.Key)
	}
	return out
}

func TestFeatChoiceIsOfferedWithTheFeatsOfTheContent(t *testing.T) {
	t.Parallel()
	c, race := featChoiceTable(t)
	ch := findFeatChoice(t, c.Choices(featChoiceBuild(race)))
	if ch.Picks != 1 || ch.Missing() != 1 {
		t.Errorf("picks %d, missing %d; want 1 and 1", ch.Picks, ch.Missing())
	}
	keys := choiceOptionKeys(ch)
	for _, want := range []string{"feat:grappler", alertaFeat, punhoFeat, livreFeat} {
		if !slices.Contains(keys, want) {
			t.Errorf("option %s missing from %v", want, keys)
		}
	}
	for _, o := range ch.Options {
		if o.Key == alertaFeat && (o.NamePT != "Alerta" || !strings.Contains(o.SummaryPT, "Texto de Alerta.")) {
			t.Errorf("option Alerta: name %q summary %q", o.NamePT, o.SummaryPT)
		}
	}
	if got := ch.LabelPT; !strings.Contains(got, "Talento (") {
		t.Errorf("label %q", got)
	}
}

func TestFeatChoiceFromNarrowsTheOptions(t *testing.T) {
	t.Parallel()
	c, race := featChoiceTable(t, alertaFeat, livreFeat)
	ch := findFeatChoice(t, c.Choices(featChoiceBuild(race)))
	if got := choiceOptionKeys(ch); !slices.Equal(got, []string{alertaFeat, livreFeat}) {
		t.Errorf("options %v, want only the two in from", got)
	}
	// A pick outside the list is not offered and does nothing.
	b := featChoiceBuild(race, punhoFeat)
	set := c.Choices(b)
	if len(set.NotOffered) != 1 {
		t.Errorf("NotOffered = %v, want the pick outside the list", set.NotOffered)
	}
	if d := Derive(b, c); slices.ContainsFunc(d.Features, func(f Feature) bool { return f.Key == punhoFeat }) {
		t.Error("a feat outside from was applied")
	}
}

func TestAFeatPickedThroughAChoiceApplies(t *testing.T) {
	t.Parallel()
	c, race := featChoiceTable(t)
	base := Derive(featChoiceBuild(race), c)
	b := featChoiceBuild(race, alertaFeat)
	d := Derive(b, c)
	if d.Initiative != base.Initiative+5 {
		t.Errorf("initiative %d, want %d", d.Initiative, base.Initiative+5)
	}
	var feat *Feature
	for i := range d.Features {
		if d.Features[i].Key == alertaFeat {
			feat = &d.Features[i]
		}
	}
	if feat == nil {
		t.Fatal("the feat is not listed with the features")
	}
	if !strings.HasPrefix(feat.SourcePT, "Talento · ") {
		t.Errorf("source %q, want it to start with %q", feat.SourcePT, "Talento · ")
	}
	set := c.Choices(b)
	if ch := findFeatChoice(t, set); ch.Missing() != 0 || !slices.Equal(ch.Picked, []string{alertaFeat}) {
		t.Errorf("picked %v, missing %d", ch.Picked, ch.Missing())
	}
	if len(set.NotOffered) != 0 || len(set.ChoiceProblems(true)) != 0 {
		t.Errorf("a valid pick has problems: %v %v", set.NotOffered, set.ChoiceProblems(true))
	}
	if err := Validate(b, c); err != nil {
		t.Errorf("Validate: %v", err)
	}
}

func TestAFeatAlreadyTakenIsNotOfferedAgain(t *testing.T) {
	t.Parallel()
	c, race := featChoiceTable(t)
	b := featChoiceBuild(race)
	b.Feats = []string{alertaFeat}
	if keys := choiceOptionKeys(findFeatChoice(t, c.Choices(b))); slices.Contains(keys, alertaFeat) {
		t.Errorf("a feat in feat_keys is offered by the choice: %v", keys)
	}
	// A pick of the same feat does not apply it twice.
	b.FeatureChoices = append(b.FeatureChoices, ScopedChoice(FeatChoiceKey(talentoTrait), alertaFeat))
	base := b
	base.FeatureChoices = nil
	if got, want := Derive(b, c).Initiative, Derive(base, c).Initiative; got != want {
		t.Errorf("initiative %d with the repeated pick, want %d", got, want)
	}
	if n := len(c.Choices(b).ChoiceProblems(false)); n == 0 {
		t.Error("the repeated pick is not reported")
	}
	// And the level-up's feat list leaves out a feat picked through a choice.
	picked := featChoiceBuild(race, alertaFeat)
	for _, o := range FeatOptions(picked, c) {
		if o.Key == alertaFeat {
			t.Error("the level-up offers a feat the sheet got from a choice")
		}
	}
}

func TestAPickForAnOwnerThatGrantsNothingDoesNothing(t *testing.T) {
	t.Parallel()
	c, race := featChoiceTable(t)
	b := featChoiceBuild(race, alertaFeat)
	b.Background = "background:acolyte"
	// A second owner: the same pick written for a feat that is not granted is ignored.
	b.FeatureChoices = append(b.FeatureChoices, ScopedChoice(FeatChoiceKey("trait:outro"+tableSuffix), alertaFeat))
	base := Derive(featChoiceBuild(race), c).Initiative
	if got := Derive(b, c).Initiative; got != base+5 {
		t.Errorf("initiative %d, want %d", got, base+5)
	}
}

func TestAFeatPickDropsWhenTheRaceChanges(t *testing.T) {
	t.Parallel()
	c, race := featChoiceTable(t)
	b := featChoiceBuild(race, alertaFeat)
	plain := Derive(featChoiceBuild(race), c).Initiative
	b.Race = "race:human"
	d := Derive(b, c)
	if slices.ContainsFunc(d.Features, func(f Feature) bool { return f.Key == alertaFeat }) {
		t.Error("the feat stayed after the race that granted it went")
	}
	if d.Initiative >= plain+5 {
		t.Errorf("initiative %d still has the feat's bonus", d.Initiative)
	}
	if set := c.Choices(b); !slices.Contains(set.NotOffered, ScopedChoice(FeatChoiceKey(talentoTrait), alertaFeat)) {
		t.Errorf("the orphan pick is not listed as not offered: %v", set.NotOffered)
	}
}

func TestAnUnmetFeatPrerequisiteRefusesThePick(t *testing.T) {
	t.Parallel()
	c, race := featChoiceTable(t)
	b := featChoiceBuild(race)
	b.BaseScores[STR] = 8
	ch := findFeatChoice(t, c.Choices(b))
	var punho ChoiceOption
	for _, o := range ch.Options {
		if o.Key == punhoFeat {
			punho = o
		}
	}
	if !punho.Blocked() || !strings.Contains(punho.ReasonPT, "Força 15") {
		t.Fatalf("Punho should be shown and blocked, with the reason: %+v", punho)
	}
	b.FeatureChoices = append(b.FeatureChoices, punho.Stored)
	set := c.Choices(b)
	problems := set.ChoiceProblems(true)
	if len(problems) == 0 || problems[0].Code != ChoiceProblemPrerequisite || problems[0].OptionKey != punhoFeat {
		t.Fatalf("problems = %+v, want a prerequisite refusal of Punho", problems)
	}
	// The feat is listed and does not apply while the character lacks the prerequisite.
	d := Derive(b, c)
	if !slices.ContainsFunc(d.Issues, func(is Issue) bool { return is.Code == IssueFeatPrerequisite }) {
		t.Errorf("no prerequisite issue: %v", d.Issues)
	}
}

func TestAFeatChoiceOnABackgroundAndOnAFeat(t *testing.T) {
	t.Parallel()
	bgKey := "background:oficio" + tableSuffix
	grantor := "feat:mestre" + tableSuffix
	feat := func(key, name string, effects ...Effect) TableFeat {
		return TableFeat{TableEntry: TableEntry{Key: key, NamePT: name}, DescPT: []string{"Texto."}, Effects: effects}
	}
	_, _, bg := tableMisc()
	bg.Key = bgKey
	bg.Feature.Effects = append(bg.Feature.Effects, Effect{Type: "choice", Choice: "feat", Count: 1, From: []string{alertaFeat}})
	c := withOverlayOn(t, loadForTest(t), Overlay{Revision: 1, Backgrounds: []TableBackground{bg}, Feats: []TableFeat{
		feat(alertaFeat, "Alerta", Effect{Type: "modifier", Target: "initiative", Mode: "add", Value: "5"}),
		feat(grantor, "Mestre", Effect{Type: "choice", Choice: "feat", Count: 1, From: []string{livreFeat}}),
		feat(livreFeat, "Livre"),
	}})
	b := torenLevelUp()
	b.Background = bgKey
	set := c.Choices(b)
	var origins []ChoiceOrigin
	for _, g := range set.Groups {
		origins = append(origins, g.Origin)
	}
	if !slices.Contains(origins, ChoiceOriginBackground) {
		t.Fatalf("no background group: %v", origins)
	}
	b.FeatureChoices = append(b.FeatureChoices, ScopedChoice(FeatChoiceKey(bg.Feature.Key), alertaFeat))
	if got, want := Derive(b, c).Initiative, Derive(torenLevelUp(), c).Initiative+5; got != want {
		t.Errorf("initiative %d, want %d", got, want)
	}
	b.Feats = []string{grantor}
	if !slices.Contains(originsOf(c.Choices(b)), ChoiceOriginFeat) {
		t.Error("a feat that grants a feat asks no choice")
	}
}

func originsOf(s ChoiceSet) []ChoiceOrigin {
	var out []ChoiceOrigin
	for _, g := range s.Groups {
		out = append(out, g.Origin)
	}
	return out
}
