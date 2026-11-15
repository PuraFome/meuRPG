package rules

import (
	"slices"
	"testing"
)

// Preflight #253, A4: a lost feat taken through a choice is named at its pick, "full.feature_choice_keys[n]", so the
// issue carries the feat's key (the changed notice reads it) and the sheet marks the feat's row.
func TestALostFeatTakenThroughAChoiceIsNamedAtItsPick(t *testing.T) {
	t.Parallel()
	c, race := featChoiceTable(t)
	b := featChoiceBuild(race, punhoFeat)
	b.BaseScores = map[Ability]int{STR: 8, DEX: 14, CON: 13, INT: 12, WIS: 10, CHA: 15}
	at := slices.Index(b.FeatureChoices, ScopedChoice(FeatChoiceKey(talentoTrait), punhoFeat))
	if at != 1 {
		t.Fatalf("the pick is at %d, want 1 (after the fighting style): %v", at, b.FeatureChoices)
	}
	is := lostIssue(t, Derive(b, c), IssueFeatPrerequisite)
	if want := "full.feature_choice_keys[1]"; is.Field != want {
		t.Errorf("field = %q, want %q", is.Field, want)
	}
	if !slices.Contains(is.Keys, punhoFeat) {
		t.Errorf("keys = %v, want the feat %q among them", is.Keys, punhoFeat)
	}
	if want := IssueFeatPrerequisite + "|full.feature_choice_keys[1]|lost:" + punhoFeat; is.ID() != want {
		t.Errorf("ID = %q, want %q", is.ID(), want)
	}
	if !is.LegacyLostID(IssueFeatPrerequisite + "|full.feature_choice_keys|O personagem não cumpre mais o pré-requisito de Punho: Força 15.") {
		t.Error("the identity stored before the pick had an index is not read as this issue")
	}
}
