package rules

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// A choice that offers the options of another feature (from_feature), from the same entry, from another one and from
// the SRD. Every name here is invented.

const (
	// fromFeatureSubclass is a table subclass of the Barbarian, another entry than the Mestre das Posturas, whose level 3
	// feature asks for two of the postures.
	fromFeatureSubclass = "subclass:rumor-das-posturas" + tableSuffix
	fromFeatureAsk      = "feature:rumor" + tableSuffix
)

// fromFeatureOverlay is the postures overlay with a second subclass, of the Barbarian, whose feature offers the postures.
func fromFeatureOverlay(e Effect) Overlay {
	o := postureOverlay()
	o.Subclasses = append(o.Subclasses, TableSubclass{
		TableEntry: TableEntry{Key: fromFeatureSubclass, NamePT: "Rumor das Posturas"}, Class: "class:barbarian",
		Levels: []TableSubclassLevel{{Level: 3, Features: []TableFeature{{Key: fromFeatureAsk, NamePT: "Rumor", DescPT: []string{"Texto de teste."}, Effects: []Effect{e}}}}},
	})
	return o
}

func askPostures(count int) Effect {
	return Effect{Type: "choice", Choice: "feature", Count: count, FromFeature: postureFeature}
}

func rumorBuild(level int, picks ...string) Build {
	return Build{
		BaseScores: map[Ability]int{STR: 15, DEX: 14, CON: 13, INT: 12, WIS: 10, CHA: 8},
		Race:       "race:human", Background: "background:acolyte", Languages: testLanguages,
		Classes:        []ClassLevel{{Class: "class:barbarian", Level: level, Subclass: fromFeatureSubclass}},
		FeatureChoices: picks,
	}
}

var allPostures = []string{postureHeron, postureBull, postureWolf, postureCrow, postureBear}

func TestAChoiceOffersTheOptionsOfAFeatureOfAnotherEntry(t *testing.T) {
	t.Parallel()
	c := withOverlay(t, fromFeatureOverlay(askPostures(2)))
	ch := optionChoiceOf(t, c.Choices(rumorBuild(3)), fromFeatureAsk)
	got := optionKeys(ch)
	slices.Sort(got)
	want := slices.Clone(allPostures)
	slices.Sort(want)
	if ch.Picks != 2 || !slices.Equal(got, want) {
		t.Fatalf("%d picks of %v, want 2 of the five postures", ch.Picks, got)
	}
	picked := []string{postureHeron, postureWolf}
	d := Derive(rumorBuild(3, picked...), c)
	for _, k := range picked {
		if !slices.ContainsFunc(d.Features, func(f Feature) bool { return f.Key == k }) {
			t.Errorf("the picked %s is not on the sheet", k)
		}
	}
	for _, is := range d.Issues {
		if strings.Contains(is.Field, "feature_choice_keys") {
			t.Errorf("issue %v about the picks, want none", is)
		}
	}
	set := c.Choices(rumorBuild(3, picked...))
	if p := optionChoiceOf(t, set, fromFeatureAsk); !slices.Equal(p.Picked, picked) || p.Missing() != 0 || len(set.NotOffered) != 0 {
		t.Errorf("picked %v, missing %d, not offered %v; want the two taken", p.Picked, p.Missing(), set.NotOffered)
	}
}

// The choices that offer the same options are one pool: the picks of the first are not offered again to the second.
func TestTheChoicesOfTheSameOptionsShareOnePool(t *testing.T) {
	t.Parallel()
	c := withOverlay(t, fromFeatureOverlay(askPostures(2)))
	b := rumorBuild(3, postureHeron, postureBull, postureWolf)
	b.Classes = append([]ClassLevel{{Class: "class:fighter", Level: 3, Subclass: postureSubclass}}, b.Classes...)
	set := c.Choices(b)
	own, ask := optionChoiceOf(t, set, postureFeature), optionChoiceOf(t, set, fromFeatureAsk)
	if !slices.Equal(own.Picked, []string{postureHeron, postureBull, postureWolf}) || len(ask.Picked) != 0 {
		t.Fatalf("the feature took %v and the ask %v, want the three picks in the feature", own.Picked, ask.Picked)
	}
	for _, o := range ask.Options {
		if want := slices.Contains(own.Picked, o.Key); o.Blocked() != want {
			t.Errorf("option %s blocked = %v, want %v", o.Key, o.Blocked(), want)
		}
	}
}

func TestAChoiceOffersTheOptionsOfAnSRDFeature(t *testing.T) {
	t.Parallel()
	c := withOverlay(t, fromFeatureOverlay(Effect{Type: "choice", Choice: "feature", Count: 1, FromFeature: "feature:fighter-fighting-style"}))
	ch := optionChoiceOf(t, c.Choices(rumorBuild(3)), fromFeatureAsk)
	if want := c.c.features["feature:fighter-fighting-style"].Options; len(want) == 0 || !slices.Equal(optionKeys(ch), want) {
		t.Errorf("options = %v, want %v", optionKeys(ch), want)
	}
}

func TestAFromFeatureThatIsNotAFeatureWithOptionsIsRefused(t *testing.T) {
	t.Parallel()
	base := loadForTest(t)
	feature := func(e Effect) Effect { e.Type, e.Choice, e.Count = "choice", "feature", max(e.Count, 1); return e }
	cases := map[string]struct {
		e          Effect
		field, msg string
	}{
		"a key the content does not have": {feature(Effect{FromFeature: "feature:nao-existe" + tableSuffix}), ".from_feature", "is not a feature of the content"},
		"a feature with no options":       {feature(Effect{FromFeature: "feature:second-wind"}), ".from_feature", "has no options"},
		"an option":                       {feature(Effect{FromFeature: postureHeron}), ".from_feature", "is an option"},
		"a trait":                         {feature(Effect{FromFeature: "trait:darkvision"}), ".from_feature", "is not a feature of the content"},
		"a list as well":                  {feature(Effect{FromFeature: postureFeature, From: []string{postureHeron}}), ".from_feature", "not several"},
		"a set as well":                   {feature(Effect{FromFeature: postureFeature, Set: "fighting_style"}), ".from_feature", "not several"},
		"a filter as well":                {feature(Effect{FromFeature: postureFeature, Filter: toolFilter("tool")}), ".filter", "only a choice of tools"},
		"a choice of skills":              {Effect{Type: "choice", Choice: "skill", Count: 1, FromFeature: postureFeature}, ".from_feature", "only a choice of features"},
		"more than the feature has":       {feature(Effect{FromFeature: postureFeature, Count: 6}), ".count", "more than the 5 options"},
		"the invocations":                 {feature(Effect{FromFeature: "feature:eldritch-invocations"}), ".from_feature", "invocations"},
		"the feature that asks":           {feature(Effect{FromFeature: fromFeatureAsk}), ".from_feature", "its own options"},
		"a feature that offers another's": {feature(Effect{FromFeature: "feature:rumor-do-rumor" + tableSuffix}), ".from_feature", "another feature's"},
		"a daily pick":                    {feature(Effect{FromFeature: postureFeature, DailyChoice: true, Duration: "short_rest"}), ".daily_choice", ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			o := fromFeatureOverlay(tc.e)
			o.Subclasses[1].Levels = append(o.Subclasses[1].Levels, TableSubclassLevel{Level: 7, Features: []TableFeature{{
				Key: "feature:rumor-do-rumor" + tableSuffix, NamePT: "Rumor do rumor", Effects: []Effect{{Type: "choice", Choice: "feature", Count: 1, FromFeature: postureFeature}},
			}}})
			_, err := base.With(o)
			var oe *OverlayError
			if !errors.As(err, &oe) {
				t.Fatalf("With: %v, want an overlay error", err)
			}
			if !slices.ContainsFunc(oe.Violations(), func(v *OverlayError) bool {
				return strings.HasSuffix(v.Field, tc.field) && strings.Contains(v.Message, tc.msg) && strings.Contains(v.Key, "rumor@")
			}) {
				t.Errorf("no violation of %s with %q at the asking feature: %v", tc.field, tc.msg, err)
			}
		})
	}
}

// Only a class or a subclass feature offers the options of another feature.
func TestAFromFeatureOnAnotherKindOfOwnerIsRefused(t *testing.T) {
	t.Parallel()
	base := loadForTest(t)
	o := postureOverlay()
	o.Backgrounds = []TableBackground{{
		TableEntry: TableEntry{Key: "background:cais-do-farol" + tableSuffix, NamePT: "Cais do farol"}, Skills: []string{"skill:athletics", "skill:history"},
		Feature: TableFeature{Key: "background-feature:rede-do-cais" + tableSuffix, NamePT: "Rede do cais", DescPT: []string{"Texto de teste."}, Effects: []Effect{askPostures(1)}},
	}}
	_, err := base.With(o)
	var oe *OverlayError
	if !errors.As(err, &oe) || !strings.HasSuffix(oe.Field, ".from_feature") || !strings.Contains(oe.Message, "only a class or subclass feature") {
		t.Errorf("With: %v, want the background feature refused at its from_feature", err)
	}
}

// A feature's options that the table keeps out of a sheet's pools: the subclass they belong to is switched off or
// archived, and the sheet holds none of it.
func TestTheOptionsOfAFeatureTheTableKeepsOutAreInNoPool(t *testing.T) {
	t.Parallel()
	base := loadForTest(t)
	closed := map[string]func(*Overlay){
		"switched off": func(o *Overlay) { o.Off = []string{postureSubclass} },
		"archived":     func(o *Overlay) { o.Subclasses[0].Archived = true },
	}
	for name, close := range closed {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			o := fromFeatureOverlay(askPostures(2))
			control := withOverlayOn(t, base, o)
			if got := optionKeys(optionChoiceOf(t, control.Choices(rumorBuild(3)), fromFeatureAsk)); len(got) != len(allPostures) {
				t.Fatalf("before the table closes it, the pool has %v, want the five postures", got)
			}
			close(&o)
			c := withOverlayOn(t, base, o)
			ch := optionChoiceOf(t, c.Choices(rumorBuild(3)), fromFeatureAsk)
			if len(ch.Options) != 0 || ch.Missing() != 2 || !ch.Advisory {
				t.Errorf("the pool has %v and asks %d more (advisory %v), want it empty and asking nothing of the sheet", optionKeys(ch), ch.Missing(), ch.Advisory)
			}
			if problems := c.Choices(rumorBuild(3)).ChoiceProblems(true); len(problems) != 0 {
				t.Errorf("problems = %v, want none for an empty pool", problems)
			}
			// A sheet that holds the subclass keeps what it offers.
			b := rumorBuild(3)
			b.Classes = append([]ClassLevel{{Class: "class:fighter", Level: 3, Subclass: postureSubclass}}, b.Classes...)
			if got := optionKeys(optionChoiceOf(t, c.Choices(b), fromFeatureAsk)); len(got) != len(allPostures) {
				t.Errorf("a sheet that holds the subclass has %v, want the five postures", got)
			}
		})
	}
}

// A pick that names one of those options reads exactly as a key the content does not have: the choices, the issues of
// the sheet and the check of the build.
func TestAPickOfAnOptionTheTableKeepsOutReadsAsAKeyThatDoesNotExist(t *testing.T) {
	t.Parallel()
	o := fromFeatureOverlay(askPostures(2))
	o.Off = []string{postureSubclass}
	c := withOverlay(t, o)
	const missing = "feature:postura-que-nao-existe" + tableSuffix
	read := func(key string) (choices string, issues []Issue, err error) {
		b := rumorBuild(3, key)
		return strings.ReplaceAll(jsonText(c.Choices(b)), key, "K"), Derive(b, c).Issues, Validate(b, c)
	}
	for _, key := range allPostures {
		gotSet, gotIssues, gotErr := read(key)
		wantSet, wantIssues, wantErr := read(missing)
		if gotSet != wantSet {
			t.Errorf("%s: the choices differ from those of a key that does not exist:\n%s\n%s", key, gotSet, wantSet)
		}
		if len(gotIssues) != len(wantIssues) {
			t.Fatalf("%s: issues %v, want those of a key that does not exist: %v", key, gotIssues, wantIssues)
		}
		for i := range gotIssues {
			if g, w := strings.ReplaceAll(jsonText(gotIssues[i]), key, "K"), strings.ReplaceAll(jsonText(wantIssues[i]), missing, "K"); g != w {
				t.Errorf("%s: issue %s, want %s", key, g, w)
			}
		}
		if gotErr == nil || wantErr == nil || gotErr.Error() != wantErr.Error() {
			t.Errorf("%s: Validate = %v, want %v", key, gotErr, wantErr)
		}
	}
}

// The level-up lists the pool the same way.
func TestTheLevelUpOffersNoOptionTheTableKeepsOut(t *testing.T) {
	t.Parallel()
	base := loadForTest(t)
	o := fromFeatureOverlay(askPostures(2))
	open := withOverlayOn(t, base, o)
	o.Off = []string{postureSubclass}
	closed := withOverlayOn(t, base, o)
	due := func(c *Content) []LevelUpFeatureChoice {
		return c.c.namedChoices(rumorBuild(3), c.c.featureGains([]string{fromFeatureAsk}, fromFeatureSubclass).choices)
	}
	if got := due(open); len(got) != 1 || len(got[0].Options) != len(allPostures) {
		t.Fatalf("before the table closes it: %+v, want the five postures", got)
	}
	if got := due(closed); len(got) != 1 || len(got[0].Options) != 0 {
		t.Errorf("after: %+v, want the choice with no option", got)
	}
}
