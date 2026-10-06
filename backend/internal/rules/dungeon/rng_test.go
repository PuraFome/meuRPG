package dungeon

import (
	"slices"
	"testing"
)

// The generator's test vector (spec 4.1): a change that alters the stream
// fails here, and must bump Version.
func TestSplitMix64Vector(t *testing.T) {
	// The reference output of SplitMix64 from state 0.
	sm := splitMix64{}
	if got := sm.next(); got != 0xe220a8397b1dcdaf {
		t.Fatalf("splitmix64(0) = %#x", got)
	}
}

func TestRNGVector(t *testing.T) {
	vectors := map[uint64][8]uint64{
		0: {
			0x99ec5f36cb75f2b4, 0xbf6e1f784956452a, 0x1a5f849d4933e6e0, 0x6aa594f1262d2d2c,
			0xbba5ad4a1f842e59, 0xffef8375d9ebcaca, 0x6c160deed2f54c98, 0x8920ad648fc30a3f,
		},
		48213: {
			0x79d66433dc65aec8, 0xe32b5a5fa2294994, 0x2517b270cf10bcd2, 0xfb1cd25443f45179,
			0xb3dfcb92c42eee23, 0x2e9f1b59c00c3f43, 0xddc30a7c06eb5566, 0xb8e0d0f480a6ec3b,
		},
	}
	for seed, want := range vectors {
		r := newRNG(seed)
		for i, w := range want {
			if got := r.next(); got != w {
				t.Fatalf("seed %d output %d = %#x, want %#x", seed, i, got, w)
			}
		}
	}
}

func TestPhaseStreamsDiffer(t *testing.T) {
	phases := []uint64{phaseMask, phaseRooms, phaseDoors, phaseCorridor, phaseConnect, phaseStairs, phaseDeadend}
	seen := map[uint64]bool{}
	for _, p := range phases {
		s := streamSeed(7, p)
		if seen[s] {
			t.Fatalf("two phases share a stream seed")
		}
		seen[s] = true
	}
}

func TestIntnBoundsAndUniformity(t *testing.T) {
	r := newRNG(1)
	for _, n := range []int{1, 2, 3, 7, 10, 100, 1 << 20} {
		for i := 0; i < 2000; i++ {
			if v := r.intn(n); v < 0 || v >= n {
				t.Fatalf("intn(%d) = %d", n, v)
			}
		}
	}
	// A rough uniformity check on a non-power of two.
	var count [6]int
	for i := 0; i < 60000; i++ {
		count[r.intn(6)]++
	}
	for k, c := range count {
		if c < 9000 || c > 11000 {
			t.Fatalf("intn(6) bucket %d = %d of 60000", k, c)
		}
	}
}

func TestIntnOneConsumesNothing(t *testing.T) {
	a, b := newRNG(5), newRNG(5)
	a.intn(1)
	a.intn(0)
	if a.next() != b.next() {
		t.Fatal("intn(1) consumed an output")
	}
}

func TestChanceDrawsNothingAtTheEnds(t *testing.T) {
	a, b := newRNG(9), newRNG(9)
	for i := 0; i < 5; i++ {
		if a.chance(0) || !a.chance(100) {
			t.Fatal("0 must never succeed and 100 must always")
		}
	}
	if a.next() != b.next() {
		t.Fatal("a 0 or 100 percent chance drew")
	}
	// strictly less than: p percent succeeds on [0, p)
	hits := 0
	r := newRNG(3)
	for i := 0; i < 20000; i++ {
		if r.chance(25) {
			hits++
		}
	}
	if hits < 4500 || hits > 5500 {
		t.Fatalf("chance(25) hit %d of 20000", hits)
	}
}

func TestShuffleIsAPermutationAndDeterministic(t *testing.T) {
	mk := func() []int {
		s := make([]int, 50)
		for i := range s {
			s[i] = i
		}
		shuffle(newRNG(11), s)
		return s
	}
	a, b := mk(), mk()
	if !slices.Equal(a, b) {
		t.Fatal("shuffle is not deterministic")
	}
	sorted := slices.Clone(a)
	slices.Sort(sorted)
	for i, v := range sorted {
		if v != i {
			t.Fatal("shuffle lost an element")
		}
	}
	if slices.IsSorted(a) {
		t.Fatal("shuffle did nothing")
	}
	// it draws len-1 bounded integers, none for lengths 0 and 1
	r, ref := newRNG(2), newRNG(2)
	shuffle(r, []int{})
	shuffle(r, []int{1})
	if r.next() != ref.next() {
		t.Fatal("a shuffle of fewer than 2 elements drew")
	}
}
