package characters

import (
	"testing"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
)

// A pick of a feature group is given up through the swap of the feature's options and the record, not through the
// warlock's invocation field: a request that names the pick there and takes the new option as a plain pick is refused, with
// nothing written and nobody told.
func TestAPickCannotBeGivenUpAroundTheSwapRecord(t *testing.T) {
	t.Parallel()
	f := newSwapFixture(t, 4, 6500, swapAt(5), nil)
	spy := &optionSwapSpy{}
	f.h.svc.SetOptionSwaps(spy)
	ch := f.choices(false)
	ch.SwapInvocationKey = f.heron
	ch.FeatureChoiceKeys = []string{f.crow}
	if r := f.swapRefusal(t, ch); r == nil {
		t.Error("preview: the request went around the swap record and was not refused")
	}
	_, err := f.levelUp(f.other, f.fighter, ch)
	if r := refusal(t, "LevelUpCharacter", err); r.GetReason() == charactersv1.LevelUpRefusalReason_LEVEL_UP_REFUSAL_REASON_UNSPECIFIED {
		t.Errorf("level-up refusal = %v", r)
	}
	again := f.other.get(t, f.campaign, f.fighter.GetId())
	if again.GetRevision() != f.fighter.GetRevision() || again.GetSheet().GetFull().GetClasses()[0].GetLevel() != 4 {
		t.Errorf("a refused level-up changed the sheet: revision %d (was %d)", again.GetRevision(), f.fighter.GetRevision())
	}
	if records, _ := f.levelUps(f.master, f.fighter.GetId()); len(records) != 0 {
		t.Errorf("level ups = %d, want none", len(records))
	}
	if len(spy.calls) != 0 {
		t.Errorf("the module was called %d times", len(spy.calls))
	}
}
