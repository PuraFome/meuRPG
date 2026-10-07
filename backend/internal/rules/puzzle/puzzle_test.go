package puzzle

import (
	"errors"
	"math/bits"
	"slices"
	"testing"
	"time"
)

// bruteLights is the fewest presses by trying every set of presses (2^(N*N)): the
// check of the GF(2) solution for the boards small enough to try them all.
func bruteLights(n int, state uint64) (best int, solvable bool) {
	masks := pressMasks(n)
	best = n*n + 1
	for set := range uint64(1) << (n * n) {
		lights := state
		for c := range n * n {
			if set>>c&1 == 1 {
				lights ^= masks[c]
			}
		}
		if lights == 0 {
			solvable = true
			best = min(best, bits.OnesCount64(set))
		}
	}
	return best, solvable
}

func TestLightsMinimumMatchesBruteForce(t *testing.T) {
	t.Parallel()
	for _, n := range []int{3, 4} {
		// Every board for N = 3 (512), and a spread of them for N = 4 (65 536 boards,
		// each tried against 65 536 sets: every 61st is plenty).
		step := 1
		if n == 4 {
			step = 61
		}
		for state := uint64(0); state < 1<<(n*n); state += uint64(step) {
			want, wantOK := bruteLights(n, state)
			got, ok, err := LightsMinimum(n, state)
			if err != nil || ok != wantOK {
				t.Fatalf("N=%d state=%#x: LightsMinimum() ok = %v, err = %v, want ok = %v", n, state, ok, err, wantOK)
			}
			if !ok {
				continue
			}
			if got.Presses != want {
				t.Fatalf("N=%d state=%#x: %d presses, brute force says %d", n, state, got.Presses, want)
			}
			// The cells it names really put every light out.
			lights := state
			for c := range n * n {
				if got.Cells>>c&1 == 1 {
					lights ^= pressMasks(n)[c]
				}
			}
			if lights != 0 || bits.OnesCount64(got.Cells) != got.Presses {
				t.Fatalf("N=%d state=%#x: the cells %#x leave %#x lit", n, state, got.Cells, lights)
			}
		}
	}
}

// For N = 4 and 5 the matrix is singular: some boards have no solution, and the
// ones that do have several, of which the search picks the shortest.
func TestLightsKernelCases(t *testing.T) {
	t.Parallel()
	kernel := map[int]int{3: 0, 4: 4, 5: 2, 6: 0, 7: 0}
	for n, want := range kernel {
		// The solved board needs no press; its kernel is the other solutions of it:
		// the sets of presses that cancel out. Count them by trying every set.
		canceling := 0
		if n <= 5 {
			masks := pressMasks(n)
			for set := uint64(1); set < 1<<(n*n); set++ {
				var lights uint64
				for c := range n * n {
					if set>>c&1 == 1 {
						lights ^= masks[c]
					}
				}
				if lights == 0 {
					canceling++
				}
			}
			if canceling != 1<<want-1 {
				t.Errorf("N=%d: %d sets of presses cancel out, want 2^%d - 1", n, canceling, want)
			}
		}
		sol, ok, err := LightsMinimum(n, 0)
		if err != nil || !ok || sol.Presses != 0 {
			t.Errorf("N=%d: the solved board takes %+v, %v, %v; want 0 presses", n, sol, ok, err)
		}
	}
	// A board with no solution at N = 4: the corner light alone (the image of the
	// press matrix lacks it).
	if _, ok, err := LightsMinimum(4, 1); err != nil || ok {
		t.Errorf("N=4, one light lit in the corner: ok = %v, err = %v; want no solution", ok, err)
	}
	// And an N = 5 board whose solutions are several: the starts the generator
	// draws always have one.
	for seed := range uint64(200) {
		start, err := LightsStart(5, seed)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok, err := LightsMinimum(5, start); err != nil || !ok {
			t.Fatalf("N=5 seed=%d: the start %#x has no solution (%v)", seed, start, err)
		}
	}
}

func TestLightsStartIsNeverSolvedAndAlwaysSolvable(t *testing.T) {
	t.Parallel()
	for n := MinLights; n <= MaxLights; n++ {
		for seed := range uint64(2000) {
			start, err := LightsStart(n, seed)
			if err != nil {
				t.Fatal(err)
			}
			if LightsSolved(start) {
				t.Fatalf("N=%d seed=%d: the start is already solved", n, seed)
			}
			sol, ok, err := LightsMinimum(n, start)
			if err != nil || !ok || sol.Presses < minStartPresses {
				t.Fatalf("N=%d seed=%d: the start has no solution (%+v, %v, %v)", n, seed, sol, ok, err)
			}
			again, _ := LightsStart(n, seed)
			if again != start {
				t.Fatalf("N=%d seed=%d: the same seed gave %#x and %#x", n, seed, start, again)
			}
		}
	}
}

func TestLightsPressToggles(t *testing.T) {
	t.Parallel()
	// The corner toggles three lights, an inner cell five, and two presses commute.
	next, changed, err := LightsPress(5, 0, 0, 0)
	if err != nil || bits.OnesCount64(changed) != 3 || next != changed {
		t.Fatalf("corner press = %#x, %#x, %v", next, changed, err)
	}
	_, changed, _ = LightsPress(5, 0, 2, 2)
	if bits.OnesCount64(changed) != 5 {
		t.Errorf("inner press changed %d lights, want 5", bits.OnesCount64(changed))
	}
	a, _, _ := LightsPress(5, 0, 1, 1)
	a, _, _ = LightsPress(5, a, 3, 2)
	b, _, _ := LightsPress(5, 0, 3, 2)
	b, _, _ = LightsPress(5, b, 1, 1)
	if a != b {
		t.Errorf("two presses in two orders gave %#x and %#x", a, b)
	}
	for _, c := range [][2]int{{-1, 0}, {0, 5}, {5, 0}} {
		if _, _, err := LightsPress(5, 0, c[0], c[1]); !errors.Is(err, ErrMove) {
			t.Errorf("press %v error = %v, want ErrMove", c, err)
		}
	}
	if _, _, err := LightsPress(2, 0, 0, 0); !errors.Is(err, ErrSize) {
		t.Errorf("a 2 x 2 board error = %v, want ErrSize", err)
	}
	if _, err := LightsStart(8, 1); !errors.Is(err, ErrSize) {
		t.Errorf("an 8 x 8 board error = %v, want ErrSize", err)
	}
}

func TestLightsBoolsRoundTrip(t *testing.T) {
	t.Parallel()
	start, _ := LightsStart(6, 9)
	got, err := LightsFromBools(6, LightsToBools(6, start))
	if err != nil || got != start {
		t.Errorf("round trip = %#x, %v; want %#x", got, err, start)
	}
	if _, err := LightsFromBools(6, make([]bool, 35)); !errors.Is(err, ErrMove) {
		t.Errorf("35 lights on a 6 x 6 board error = %v, want ErrMove", err)
	}
}

func TestLockWheelsWrapAround(t *testing.T) {
	t.Parallel()
	for _, a := range []Alphabet{Digits, Letters, Runes} {
		size := a.Size()
		up, err := LockTurn(a, []int{size - 1, 0}, 0, 1)
		if err != nil || up[0] != 0 || up[1] != 0 {
			t.Errorf("alphabet %d: the last face turned up = %v, %v; want the first", a, up, err)
		}
		down, err := LockTurn(a, []int{size - 1, 0}, 1, -1)
		if err != nil || down[1] != size-1 || down[0] != size-1 {
			t.Errorf("alphabet %d: the first face turned down = %v, %v; want the last", a, down, err)
		}
	}
	if Digits.Size() != 10 || Letters.Size() != 26 || Runes.Size() != 8 {
		t.Errorf("sizes = %d, %d, %d; want 10, 26, 8", Digits.Size(), Letters.Size(), Runes.Size())
	}
	state := []int{1, 2, 3}
	if _, err := LockTurn(Digits, state, 0, 1); err != nil || !slices.Equal(state, []int{1, 2, 3}) {
		t.Errorf("LockTurn changed its argument: %v, %v", state, err)
	}
	for _, bad := range [][2]int{{3, 1}, {-1, 1}, {0, 0}, {0, 2}} {
		if _, err := LockTurn(Digits, state, bad[0], bad[1]); !errors.Is(err, ErrMove) {
			t.Errorf("turn %v error = %v, want ErrMove", bad, err)
		}
	}
}

func TestLockSolvedAndMinimum(t *testing.T) {
	t.Parallel()
	solution := []int{3, 0, 7, 2}
	start := []int{3, 9, 1, 2}
	if LockSolved(start, solution) || !LockSolved(solution, solution) {
		t.Fatal("LockSolved is wrong")
	}
	moves, path := LockMinimum(Digits, start, solution)
	if moves != 1+4 || len(path) != moves {
		t.Fatalf("minimum = %d (%d steps), want 5: wheel 2 by one up, wheel 3 by four the short way (6 up or 4 down)", moves, len(path))
	}
	state := start
	for _, s := range path {
		state, _ = LockTurn(Digits, state, s.Wheel, s.Delta)
	}
	if !LockSolved(state, solution) {
		t.Errorf("the path leaves %v, want %v", state, solution)
	}
	// Two turns in two orders reach the same wheels.
	a, _ := LockTurn(Runes, []int{0, 0}, 0, 1)
	a, _ = LockTurn(Runes, a, 1, -1)
	b, _ := LockTurn(Runes, []int{0, 0}, 1, -1)
	b, _ = LockTurn(Runes, b, 0, 1)
	if !slices.Equal(a, b) {
		t.Errorf("two turns in two orders gave %v and %v", a, b)
	}
}

func TestValidateLock(t *testing.T) {
	t.Parallel()
	ok := func(wheels int, a Alphabet, solution, start []int, want error) {
		t.Helper()
		if err := ValidateLock(wheels, a, solution, start); !errors.Is(err, want) && (want != nil || err != nil) {
			t.Errorf("ValidateLock(%d, %d, %v, %v) = %v, want %v", wheels, a, solution, start, err, want)
		}
	}
	ok(4, Runes, []int{0, 1, 2, 3}, []int{1, 1, 2, 3}, nil)
	ok(1, Runes, []int{0}, []int{1}, ErrSize)
	ok(7, Runes, make([]int, 7), make([]int, 7), ErrSize)
	ok(2, Alphabet(9), []int{0, 0}, []int{0, 1}, ErrSymbols)
	ok(2, Runes, []int{0, 8}, []int{0, 1}, ErrSymbols)
	ok(2, Runes, []int{0, 1}, []int{0}, ErrSymbols)
	ok(2, Runes, []int{0, 1}, []int{0, 1}, ErrSolved)
	// Our eight runes are named in Portuguese, each with its own key.
	names := map[string]bool{}
	for _, r := range Runes.Symbols() {
		if r.NamePT == "" || r.Key == "" || names[r.Key] {
			t.Errorf("rune %+v has no name or repeats a key", r)
		}
		names[r.Key] = true
	}
}

func TestPillarsMinimumAndLinks(t *testing.T) {
	t.Parallel()
	// The artboard's example: four pillars of four symbols, each turning with its
	// left and right neighbor; start Serpente, Coruja, Serpente, Lobo (2, 3, 2, 1)
	// and mural Lobo, Corvo, Coruja, Serpente (1, 0, 3, 2): 8 turns.
	p := Pillars{Count: 4, Glyphs: 4, Links: [][]int{{1}, {0, 2}, {1, 3}, {2}}}
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	moves, path, ok, err := p.Minimum([]int{2, 3, 2, 1}, []int{1, 0, 3, 2})
	if err != nil || !ok {
		t.Fatalf("Minimum() ok = %v, err = %v", ok, err)
	}
	if moves != len(path) {
		t.Errorf("moves = %d, the path has %d steps", moves, len(path))
	}
	state := []int{2, 3, 2, 1}
	for _, s := range path {
		state, _, _ = p.Turn(state, s.Pillar, s.Delta)
	}
	if !PillarsSolved(state, []int{1, 0, 3, 2}) {
		t.Errorf("the path leaves %v", state)
	}
	// Turning pillar 1 turns 0, 1 and 2; turning it back undoes it.
	next, changed, err := p.Turn([]int{0, 0, 0, 0}, 1, 1)
	if err != nil || !slices.Equal(next, []int{1, 1, 1, 0}) || !slices.Equal(changed, []int{1, 0, 2}) {
		t.Errorf("Turn(1, +1) = %v, %v, %v", next, changed, err)
	}
	back, _, _ := p.Turn(next, 1, -1)
	if !slices.Equal(back, []int{0, 0, 0, 0}) {
		t.Errorf("turning back gave %v", back)
	}
	// Without links the minimum is the sum of the short ways round.
	free := Pillars{Count: 3, Glyphs: 5}
	moves, _, ok, _ = free.Minimum([]int{0, 0, 0}, []int{1, 4, 2})
	if !ok || moves != 1+1+2 {
		t.Errorf("free pillars minimum = %d, %v; want 4", moves, ok)
	}
	// The target itself takes no move.
	if moves, path, ok, _ := free.Minimum([]int{1, 1, 1}, []int{1, 1, 1}); !ok || moves != 0 || len(path) != 0 {
		t.Errorf("solved minimum = %d, %v, %v", moves, path, ok)
	}
}

// Links can make a target unreachable: two pillars of three symbols that always
// turn together keep their difference.
func TestPillarsUnreachableTarget(t *testing.T) {
	t.Parallel()
	p := Pillars{Count: 3, Glyphs: 3, Links: [][]int{{1, 2}, {0, 2}, {0, 1}}}
	if _, _, ok, err := p.Minimum([]int{0, 0, 0}, []int{1, 0, 0}); err != nil || ok {
		t.Errorf("with every pillar linked to every other, ok = %v, err = %v; want unreachable", ok, err)
	}
	if _, _, ok, err := p.Minimum([]int{0, 0, 0}, []int{1, 1, 1}); err != nil || !ok {
		t.Errorf("(1, 1, 1) is one turn away: ok = %v, err = %v", ok, err)
	}
}

func TestPillarsStartIsNeverSolvedAndReachable(t *testing.T) {
	t.Parallel()
	configs := []Pillars{
		{Count: 3, Glyphs: 3},
		{Count: 4, Glyphs: 4, Links: [][]int{{1}, {0, 2}, {1, 3}, {2}}},
		{Count: 6, Glyphs: 6, Links: [][]int{{1}, {0, 2}, {1, 3}, {2, 4}, {3, 5}, {4}}},
		{Count: 5, Glyphs: 3, Links: [][]int{{1, 2}, nil, {3}}},
	}
	for ci, p := range configs {
		for seed := range uint64(60) {
			target := make([]int, p.Count)
			for i := range target {
				target[i] = (i + int(seed)) % p.Glyphs
			}
			start, err := p.Start(target, seed)
			if err != nil {
				t.Fatal(err)
			}
			if PillarsSolved(start, target) {
				t.Fatalf("config %d seed %d: the start is the target", ci, seed)
			}
			moves, _, ok, err := p.Minimum(start, target)
			if err != nil || !ok || moves == 0 {
				t.Fatalf("config %d seed %d: the start cannot be solved (%d, %v, %v)", ci, seed, moves, ok, err)
			}
			again, _ := p.Start(target, seed)
			if !slices.Equal(again, start) {
				t.Fatalf("config %d seed %d: the same seed gave %v and %v", ci, seed, start, again)
			}
		}
	}
}

func TestValidatePillars(t *testing.T) {
	t.Parallel()
	bad := []struct {
		name string
		p    Pillars
		want error
	}{
		{"too few pillars", Pillars{Count: 2, Glyphs: 3}, ErrSize},
		{"too many pillars", Pillars{Count: 7, Glyphs: 3}, ErrSize},
		{"too few symbols", Pillars{Count: 3, Glyphs: 2}, ErrSize},
		{"too many symbols", Pillars{Count: 3, Glyphs: 7}, ErrSize},
		{"a link to itself", Pillars{Count: 3, Glyphs: 3, Links: [][]int{{0}}}, ErrLinks},
		{"a link out of range", Pillars{Count: 3, Glyphs: 3, Links: [][]int{{3}}}, ErrLinks},
		{"a repeated link", Pillars{Count: 3, Glyphs: 3, Links: [][]int{{1, 1}}}, ErrLinks},
		{"more links than pillars", Pillars{Count: 3, Glyphs: 3, Links: [][]int{nil, nil, nil, nil}}, ErrLinks},
	}
	for _, tc := range bad {
		if err := tc.p.Validate(); !errors.Is(err, tc.want) {
			t.Errorf("%s: Validate() = %v, want %v", tc.name, err, tc.want)
		}
	}
	if Glyphs(2) != nil || len(Glyphs(6)) != 6 || Glyphs(4)[0].NamePT != "Corvo" {
		t.Error("Glyphs() is wrong")
	}
	p := Pillars{Count: 3, Glyphs: 3}
	if _, _, err := p.Turn([]int{0, 0, 0}, 3, 1); !errors.Is(err, ErrMove) {
		t.Errorf("turning pillar 3 error = %v, want ErrMove", err)
	}
	if _, _, err := p.Turn([]int{0, 0, 3}, 0, 1); !errors.Is(err, ErrSymbols) {
		t.Errorf("a position out of range error = %v, want ErrSymbols", err)
	}
}

// Moves commute (the reason two players can move at once): every order of a set
// of moves ends in the same state.
func TestMovesCommute(t *testing.T) {
	t.Parallel()
	p := Pillars{Count: 4, Glyphs: 5, Links: [][]int{{1}, {0, 2}, {1, 3}, {2}}}
	moves := []PillarStep{{0, 1}, {2, -1}, {1, 1}, {3, 1}, {1, -1}}
	apply := func(order []int) []int {
		state := []int{0, 1, 2, 3}
		for _, i := range order {
			state, _, _ = p.Turn(state, moves[i].Pillar, moves[i].Delta)
		}
		return state
	}
	want := apply([]int{0, 1, 2, 3, 4})
	for _, order := range [][]int{{4, 3, 2, 1, 0}, {2, 0, 4, 1, 3}, {3, 4, 0, 2, 1}} {
		if got := apply(order); !slices.Equal(got, want) {
			t.Errorf("order %v ended in %v, want %v", order, got, want)
		}
	}
}

// The numbers the docs quote: how long the biggest problems take. Run with -v.
func TestTimings(t *testing.T) {
	t.Parallel()
	start := time.Now()
	for seed := range uint64(1000) {
		s, _ := LightsStart(7, seed)
		if _, ok, err := LightsMinimum(7, s); err != nil || !ok {
			t.Fatal("7 x 7 start without a solution")
		}
	}
	t.Logf("7 x 7: 1000 starts and minimums in %v", time.Since(start))
	p := Pillars{Count: 6, Glyphs: 6, Links: [][]int{{1}, {0, 2}, {1, 3}, {2, 4}, {3, 5}, {4}}}
	start = time.Now()
	for range 20 {
		if _, _, ok, err := p.Minimum([]int{0, 0, 0, 0, 0, 0}, []int{3, 3, 3, 3, 3, 3}); err != nil || !ok {
			t.Fatal("pillars unsolved")
		}
	}
	t.Logf("6 pillars x 6 symbols: a full BFS in %v", time.Since(start)/20)
}

func BenchmarkLightsMinimum7(b *testing.B) {
	s, _ := LightsStart(7, 7)
	for b.Loop() {
		_, _, _ = LightsMinimum(7, s)
	}
}

func BenchmarkPillarsMinimum6x6(b *testing.B) {
	p := Pillars{Count: 6, Glyphs: 6, Links: [][]int{{1}, {0, 2}, {1, 3}, {2, 4}, {3, 5}, {4}}}
	for b.Loop() {
		_, _, _, _ = p.Minimum([]int{0, 0, 0, 0, 0, 0}, []int{3, 3, 3, 3, 3, 3})
	}
}
