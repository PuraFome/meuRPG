package rules

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// #580: an option of a table feature may ask for what a table option asks for. The posture overlay's Bull asks for
// level 5 of the Fighter, and Wolf asks for the Bull.

func gatedPostureOverlay() Overlay {
	o := postureOverlay()
	opts := o.Subclasses[0].Levels[0].Features[0].Options
	opts[1].Prerequisite = TableOptionPrerequisite{ClassKey: "class:fighter", ClassLevel: 5}
	opts[2].Prerequisite = TableOptionPrerequisite{RequiresOptions: []string{postureBull}}
	return o
}

func TestAnInlineOptionIsBlockedUntilItsPrerequisiteHolds(t *testing.T) {
	t.Parallel()
	c := withOverlay(t, gatedPostureOverlay())

	low := optionOf(t, optionChoiceOf(t, c.Choices(postureBuild(3)), postureFeature), postureBull)
	if !low.Blocked() || !strings.Contains(low.ReasonPT, "nível 5") {
		t.Errorf("Bull at level 3 = %+v, want blocked by the class level", low)
	}
	wolf := optionOf(t, optionChoiceOf(t, c.Choices(postureBuild(3)), postureFeature), postureWolf)
	if !wolf.Blocked() || !strings.Contains(wolf.ReasonPT, "Exige a opção") {
		t.Errorf("Wolf without Bull = %+v, want blocked by the option", wolf)
	}
	free := optionOf(t, optionChoiceOf(t, c.Choices(postureBuild(3)), postureFeature), postureHeron)
	if free.Blocked() {
		t.Errorf("Heron asks nothing but is blocked: %+v", free)
	}
	// At level 5 with Bull picked, both are open.
	ch := optionChoiceOf(t, c.Choices(postureBuild(5, postureBull)), postureFeature)
	if optionOf(t, ch, postureWolf).Blocked() {
		t.Errorf("Wolf is blocked with Bull picked: %+v", optionOf(t, ch, postureWolf))
	}
}

// An option the character picked and whose prerequisite is lost stays on the sheet with the "prerequisite" problem.
func TestAnInlineOptionWhoseLevelIsLostStaysWithTheProblem(t *testing.T) {
	t.Parallel()
	c := withOverlay(t, gatedPostureOverlay())
	set := c.Choices(postureBuild(3, postureBull))
	ch := optionChoiceOf(t, set, postureFeature)
	if !slices.Contains(ch.Picked, postureBull) {
		t.Fatalf("the pick left the choice: %+v", ch.Picked)
	}
	if !slices.Contains(ch.Unmet, postureBull) {
		t.Errorf("unmet = %v, want the Bull listed", ch.Unmet)
	}
	if !slices.ContainsFunc(set.ChoiceProblems(false), func(p ChoiceProblem) bool { return p.Code == ChoiceProblemPrerequisite && p.OptionKey == postureBull }) {
		t.Errorf("problems = %+v, want the prerequisite problem on the Bull", set.ChoiceProblems(false))
	}
	d := Derive(postureBuild(3, postureBull), c)
	if !slices.ContainsFunc(d.Features, func(f Feature) bool { return f.Key == postureBull }) {
		t.Errorf("the option left the sheet: %v", d.Features)
	}
}

func TestInlineOptionPrerequisiteRefusalsNameTheirField(t *testing.T) {
	t.Parallel()
	const opt0 = "subclasses[0].levels[0].features[0].options[0].prerequisite"
	cases := []struct {
		name  string
		edit  func(o *Overlay)
		field string
	}{
		{"a class that does not exist", func(o *Overlay) {
			o.Subclasses[0].Levels[0].Features[0].Options[0].Prerequisite = TableOptionPrerequisite{ClassKey: "class:fantasma", ClassLevel: 3}
		}, opt0 + ".class_level.class_key"},
		{"a pact that is not one", func(o *Overlay) {
			o.Subclasses[0].Levels[0].Features[0].Options[0].Prerequisite = TableOptionPrerequisite{RequiresPact: "feature:action-surge"}
		}, opt0 + ".requires_pact"},
		{"an option that does not exist", func(o *Overlay) {
			o.Subclasses[0].Levels[0].Features[0].Options[0].Prerequisite = TableOptionPrerequisite{RequiresOptions: []string{"feature:fantasma" + tableSuffix}}
		}, opt0 + ".requires_option_keys[0]"},
		{"an option that asks for itself", func(o *Overlay) {
			o.Subclasses[0].Levels[0].Features[0].Options[0].Prerequisite = TableOptionPrerequisite{RequiresOptions: []string{postureHeron}}
		}, opt0 + ".requires_option_keys[0]"},
		{"a prerequisite on the feature itself", func(o *Overlay) {
			o.Subclasses[0].Levels[0].Features[0].Prerequisite = TableOptionPrerequisite{ClassKey: "class:fighter", ClassLevel: 3}
		}, "subclasses[0].levels[0].features[0].prerequisite"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			o := postureOverlay()
			tc.edit(&o)
			_, err := loadForTest(t).With(o)
			var oe *OverlayError
			if !errors.As(err, &oe) {
				t.Fatalf("want *OverlayError, got %T: %v", err, err)
			}
			if !hasViolation(oe, tc.field) {
				t.Errorf("no violation at %s: %v", tc.field, err)
			}
		})
	}
}

func TestAFeatPrerequisiteRefusalsNameTheirField(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		p     FeatPrerequisite
		field string
	}{
		{"a class without a level", FeatPrerequisite{ClassKey: "class:warlock"}, "feats[0].prerequisite.class_level"},
		{"a class that does not exist", FeatPrerequisite{ClassKey: "class:fantasma", ClassLevel: 2}, "feats[0].prerequisite.class_level.class_key"},
		{"a pact that is not one", FeatPrerequisite{RequiresPact: "feature:action-surge"}, "feats[0].prerequisite.requires_pact"},
		{"a spell that does not exist", FeatPrerequisite{RequiresSpell: "spell:fantasma"}, "feats[0].prerequisite.requires_spell_key"},
		{"an option that does not exist", FeatPrerequisite{RequiresOptions: []string{"feature:fantasma"}}, "feats[0].prerequisite.requires_option_keys[0]"},
		{"eleven options", FeatPrerequisite{RequiresOptions: slices.Repeat([]string{"feature:fighting-style-defense"}, 11)}, "feats[0].prerequisite.requires_option_keys"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := loadForTest(t).With(Overlay{Revision: 1, Feats: []TableFeat{{TableEntry: TableEntry{Key: "feat:x" + tableSuffix, NamePT: "X"}, DescPT: []string{"Texto."}, Prerequisite: tc.p}}})
			var oe *OverlayError
			if !errors.As(err, &oe) {
				t.Fatalf("want *OverlayError, got %T: %v", err, err)
			}
			if !hasViolation(oe, tc.field) {
				t.Errorf("no violation at %s: %v", tc.field, err)
			}
		})
	}
}

func hasViolation(oe *OverlayError, field string) bool {
	for _, v := range oe.Violations() {
		if v.Field == field {
			return true
		}
	}
	return false
}
