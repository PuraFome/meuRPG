package dicetest

import (
	"slices"
	"sync"
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/platform/dice"
)

var d20 = dice.Expr{Count: 1, Sides: 20}

// totals rolls one initiative per bonus, in order, as the service does.
func totals(t *testing.T, r dice.Roller, bonuses []int) []int {
	t.Helper()
	var out []int
	for _, b := range bonuses {
		res, err := dice.Roll(r, dice.Expr{Count: 1, Sides: 20, Modifier: b})
		if err != nil {
			t.Fatalf("dice.Roll() error = %v", err)
		}
		out = append(out, res.Total)
	}
	return out
}

func distinct(xs []int) bool {
	s := slices.Clone(xs)
	slices.Sort(s)
	return len(slices.Compact(s)) == len(xs)
}

func TestTenCreaturesRollTenDistinctTotals(t *testing.T) {
	t.Parallel()
	for _, bonus := range []int{-5, 0, 2, 10} {
		bonuses := slices.Repeat([]int{bonus}, 10)
		if got := totals(t, New(), bonuses); !distinct(got) {
			t.Errorf("ten creatures with bonus %d rolled %v: two tie", bonus, got)
		}
	}
	// The whole cycle: 20 creatures, 20 faces.
	if got := totals(t, New(), slices.Repeat([]int{3}, 20)); !distinct(got) {
		t.Errorf("twenty creatures rolled %v: two tie", got)
	}
}

func TestFourCreaturesWithBonusesWithinFiveNeverTie(t *testing.T) {
	t.Parallel()
	for _, low := range []int{-5, 0, 5} {
		for code := range 6 * 6 * 6 * 6 {
			bonuses := make([]int, 4)
			for i, n := 0, code; i < 4; i, n = i+1, n/6 {
				bonuses[i] = low + n%6
			}
			if got := totals(t, New(), bonuses); !distinct(got) {
				t.Fatalf("bonuses %v rolled the totals %v: two tie", bonuses, got)
			}
		}
	}
}

func TestScriptedFacesAreUsedFirstThenTheDefault(t *testing.T) {
	t.Parallel()
	plain, scripted := New(), New()
	scripted.ScriptFor(t, 20, 7, 3)
	want := []int{7, 3}
	for range 5 {
		face, err := plain.Roll(20)
		if err != nil {
			t.Fatalf("Roll() error = %v", err)
		}
		want = append(want, face)
	}
	for i, w := range want {
		got, err := scripted.Roll(20)
		if err != nil || got != w {
			t.Fatalf("roll %d = %d, %v; want %d", i, got, err, w)
		}
	}
	// Faces scripted later come out next, before the default goes on.
	scripted.ScriptFor(t, 20, 20)
	if got, err := scripted.Roll(20); err != nil || got != 20 {
		t.Errorf("a face scripted later = %d, %v; want 20", got, err)
	}
}

func TestAScriptedFaceIsForItsDieOnly(t *testing.T) {
	t.Parallel()
	plain, r := New(), New()
	r.ScriptFor(t, 6, 3)
	// A d20 roll leaves the d6 script alone and takes the default.
	want, _ := plain.Roll(20)
	if got, err := r.Roll(20); err != nil || got != want {
		t.Fatalf("a d20 after a script for the d6 = %d, %v; want the default %d", got, err, want)
	}
	if got, err := r.Roll(6); err != nil || got != 3 {
		t.Errorf("the d6 = %d, %v; want the scripted 3", got, err)
	}
	if got, err := r.Roll(6); err != nil || got < 1 || got > 6 {
		t.Errorf("the d6 after the script = %d, %v; want the default", got, err)
	}
}

// spy records what a failed ScriptFor tells the test.
type spy struct {
	testing.TB
	failed bool
}

func (s *spy) Helper() {}

func (s *spy) Fatalf(string, ...any) {
	s.failed = true
	panic(s) // a real Fatalf stops the test
}

func TestAScriptedFaceThatDoesNotFitFailsAtOnce(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		sides int
		faces []int
	}{
		"9 on a d6":  {6, []int{2, 9}},
		"0 on a d20": {20, []int{0}},
		"a d0":       {0, []int{1}},
	} {
		s := &spy{TB: t}
		func() {
			defer func() { _ = recover() }()
			New().ScriptFor(s, c.sides, c.faces...)
		}()
		if !s.failed {
			t.Errorf("%s: ScriptFor did not fail the test", name)
		}
	}
	if _, err := New().Roll(0); err == nil {
		t.Error("a d0 roll did not fail")
	}
}

func TestTheDefaultFacesFitTheirDieAndAreTheSameEveryRun(t *testing.T) {
	t.Parallel()
	sides := []int{4, 6, 8, 10, 12, 20, 100}
	run := func() []int {
		r := New()
		var out []int
		for range 500 {
			for _, s := range sides {
				face, err := r.Roll(s)
				if err != nil || face < 1 || face > s {
					t.Fatalf("Roll(%d) = %d, %v", s, face, err)
				}
				out = append(out, face)
			}
		}
		return out
	}
	first := run()
	if !slices.Equal(first, run()) {
		t.Error("two runs of the same rolls gave different faces")
	}
	if len(slices.Compact(slices.Sorted(slices.Values(first[:200])))) < 10 {
		t.Error("the default faces are all alike")
	}
}

func TestTheDefaultNeverRunsOutAndIsSafeToShare(t *testing.T) {
	t.Parallel()
	r := New()
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 1000 {
				if _, err := dice.Roll(r, d20); err != nil {
					t.Errorf("dice.Roll() error = %v", err)
					return
				}
			}
		})
	}
	wg.Wait()
}
