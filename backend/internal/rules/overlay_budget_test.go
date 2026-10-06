package rules

import (
	"errors"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// budgetOverlay is the worst an overlay may be and still pass every limit: 300
// entries, 2,000 effects (20 in each of 100 features) and 500 distinct formulas
// (the other effects repeat them).
func budgetOverlay() Overlay {
	_, _, bg := tableMisc()
	o := Overlay{Revision: 3}
	n := 0
	for i := range 100 {
		b := bg
		b.Key = "background:antecedente-" + strconv.Itoa(i) + tableSuffix
		fx := make([]Effect, 0, MaxFeatureEffects)
		for range MaxFeatureEffects {
			// 500 distinct texts, each repeated four times over the overlay; the
			// formulas use every part of the grammar so each compile has work to do.
			fx = append(fx, Effect{Type: "modifier", Target: "ac", Mode: "add", Value: `max(1, mod("str") + ` + strconv.Itoa(n%MaxOverlayFormulas+1) + ")"})
			n++
		}
		b.Feature = TableFeature{Key: "background-feature:antecedente-" + strconv.Itoa(i) + tableSuffix, NamePT: "F", Effects: fx}
		o.Backgrounds = append(o.Backgrounds, b)
	}
	return o
}

// TestWithAtTheBudgets: an overlay at every budget (2,000 effects, 500 distinct
// formulas) is accepted and stays cheap: under 50 ms and under 8 MB kept. Without
// the budgets, 300 classes of 60 features of 20 effects took 24 s and kept 673 MB.
func TestWithAtTheBudgets(t *testing.T) {
	srd := loadForTest(t)
	o := budgetOverlay()
	best := time.Hour
	var kept *Content
	heap := func() uint64 {
		runtime.GC()
		runtime.GC()
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		return m.HeapAlloc
	}
	before := heap()
	for range 5 {
		start := time.Now()
		c, err := srd.With(o)
		if err != nil {
			t.Fatal(err)
		}
		best = min(best, time.Since(start))
		kept = c
	}
	after := heap()
	retained := float64(after-before) / 1e6
	t.Logf("at the budgets: %v best of 5, %.2f MB kept (one content)", best, retained)
	if raceEnabled {
		t.Log("-race: the time is not held")
	} else if best > 50*time.Millisecond {
		t.Errorf("With at the budgets took %v, want under 50 ms", best)
	}
	if retained > 8 {
		t.Errorf("a content at the budgets keeps %.2f MB, want under 8", retained)
	}
	runtime.KeepAlive(kept)
}

// TestBudgetRefusalsHappenBeforeAnyCompile: the 2,001st effect and the 501st
// distinct formula are refused with the limit named, and fast (nothing was
// compiled).
func TestBudgetRefusalsHappenBeforeAnyCompile(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)

	moreEffects := budgetOverlay()
	bg := moreEffects.Backgrounds[0]
	bg.Key = "background:um-a-mais" + tableSuffix
	bg.Feature = TableFeature{Key: "background-feature:um-a-mais" + tableSuffix, NamePT: "F", Effects: []Effect{{Type: "note"}}}
	moreEffects.Backgrounds = append(moreEffects.Backgrounds, bg)

	moreFormulas := budgetOverlay()
	moreFormulas.Backgrounds[0].Feature.Effects[0].Value = "max(2, 3)"

	for name, tc := range map[string]struct {
		o    Overlay
		want string
	}{
		"effects":  {moreEffects, "more than 2000 effects"},
		"formulas": {moreFormulas, "more than 500 distinct formulas"},
	} {
		start := time.Now()
		_, err := srd.With(tc.o)
		var oe *OverlayError
		if !errors.As(err, &oe) || oe.Reason != ReasonLimit || !strings.Contains(oe.Message, tc.want) {
			t.Errorf("%s: err = %v", name, err)
		}
		if d := time.Since(start); d > 20*time.Millisecond && !raceEnabled {
			t.Errorf("%s: the refusal took %v: it must not compile anything", name, d)
		}
	}
}
