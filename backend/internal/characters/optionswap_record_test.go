package characters

import (
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
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

// useResource spends n uses of the resource and starts the state, through the vitals as the live session does.
func (f *swapFixture) useResource(t *testing.T, resource, state string, n int32) {
	t.Helper()
	ctx := t.Context()
	err := db.InTx(ctx, f.h.pool, func(tx pgx.Tx) error {
		_, _, err := f.h.svc.AdjustVitals(ctx, tx, f.campaign, f.fighter.GetId(), &playv1.AdjustCharacterVitalsRequest{ResourcesUsed: []*playv1.ResourceUsed{{Key: resource, Used: n}}})
		if err != nil {
			return err
		}
		_, _, err = f.h.svc.SetActiveStates(ctx, tx, f.campaign, f.fighter.GetId(), []*playv1.ActiveState{{Key: state}})
		return err
	})
	if err != nil {
		t.Fatalf("spend %s and start %s: %v", resource, state, err)
	}
}

// stored reads the stored used counts of the resources and the states that are on, as the columns hold them.
func (f *swapFixture) stored(t *testing.T) (resources, states string) {
	t.Helper()
	err := f.h.pool.QueryRow(t.Context(), `SELECT COALESCE((SELECT resources_used::TEXT FROM character_vitals WHERE character_id = c.id), '{}'), COALESCE(c.active_states::TEXT, '[]')
		FROM characters AS c WHERE c.id = $1`, f.fighter.GetId()).Scan(&resources, &states)
	if err != nil {
		t.Fatalf("read the stored uses and states: %v", err)
	}
	return resources, states
}

// What the given-up option had stored goes with it: its resources' used counts and the state it started. The option picked again
// later has its uses fresh.
func TestAnOptionGivenUpTakesItsUsedCountsAndStatesWithIt(t *testing.T) {
	t.Parallel()
	f := newSwapFixture(t, 4, 14000, swapEvery(), nil)
	state := f.heron + "#garca"
	f.useResource(t, "garca", state, 1)
	if res, states := f.stored(t); !strings.Contains(res, `"garca": 1`) || !strings.Contains(states, state) {
		t.Fatalf("before the swap: resources %s, states %s, want the use and the state stored", res, states)
	}

	away := f.choices(false)
	away.SwapFeatureOptions = []*charactersv1.LevelUpPickSwap{featureSwap(f.postures.GetKey(), f.heron, f.crow)}
	fighter, err := f.levelUp(f.other, f.fighter, away)
	if err != nil {
		t.Fatalf("level 5, swapping the Heron away: %v", err)
	}
	if res, states := f.stored(t); strings.Contains(res, "garca") || strings.Contains(states, state) {
		t.Errorf("after the swap: resources %s, states %s, want neither the use nor the state of the Heron", res, states)
	}

	back := f.choices(false)
	back.SwapFeatureOptions = []*charactersv1.LevelUpPickSwap{featureSwap(f.postures.GetKey(), f.crow, f.heron)}
	if _, err := f.levelUp(f.other, fighter, back); err != nil {
		t.Fatalf("level 6, taking the Heron back: %v", err)
	}
	if res, states := f.stored(t); strings.Contains(res, "garca") || strings.Contains(states, state) {
		t.Errorf("after taking it back: resources %s, states %s, want it fresh", res, states)
	}
}
