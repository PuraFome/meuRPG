package dungeon

import "math/bits"

// The random generator is ours, not math/rand's, whose stream is not
// guaranteed stable across Go releases (spec 4.1).
//
//   - splitMix64 is Sebastiano Vigna's SplitMix64 (the mixing function of
//     Steele, Lea and Flood, "Fast splittable pseudorandom number
//     generators", OOPSLA 2014). It expands one 64-bit seed into the state.
//   - rng is xoshiro256** by David Blackman and Sebastiano Vigna,
//     "Scrambled linear pseudorandom number generators", ACM TOMS 47(4),
//     2021.
//   - Bounded integers use Daniel Lemire's multiply-shift with rejection
//     ("Fast random integer generation in an interval", ACM TOMS 29(1), 2019),
//     never x % n.
//
// The test vector (a seed and its first 8 outputs) is pinned in rng_test.go.

// splitMix64 is the SplitMix64 generator.
type splitMix64 struct{ s uint64 }

func (m *splitMix64) next() uint64 {
	m.s += 0x9E3779B97F4A7C15
	z := m.s
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}

// rng is xoshiro256**.
type rng struct{ s [4]uint64 }

// newRNG expands a 64-bit seed into the 256-bit state with SplitMix64.
func newRNG(seed uint64) *rng {
	sm := splitMix64{s: seed}
	r := &rng{}
	for i := range r.s {
		r.s[i] = sm.next()
	}
	return r
}

// streamSeed is the seed of one phase's stream: SplitMix64 applied to the
// seed XOR the phase constant (spec 4.2).
func streamSeed(seed, phase uint64) uint64 {
	sm := splitMix64{s: seed ^ phase}
	return sm.next()
}

// stream returns the generator of one phase.
func stream(seed, phase uint64) *rng { return newRNG(streamSeed(seed, phase)) }

func (r *rng) next() uint64 {
	result := bits.RotateLeft64(r.s[1]*5, 7) * 9
	t := r.s[1] << 17
	r.s[2] ^= r.s[0]
	r.s[3] ^= r.s[1]
	r.s[1] ^= r.s[2]
	r.s[0] ^= r.s[3]
	r.s[2] ^= t
	r.s[3] = bits.RotateLeft64(r.s[3], 45)
	return result
}

// intn is an unbiased integer in [0, n) (Lemire). With n <= 1 the answer is
// 0 and no output is consumed: a choice among one thing is not a draw.
func (r *rng) intn(n int) int {
	if n <= 1 {
		return 0
	}
	un := uint64(n)
	hi, lo := bits.Mul64(r.next(), un)
	if lo < un {
		t := -un % un
		for lo < t {
			hi, lo = bits.Mul64(r.next(), un)
		}
	}
	return int(hi) //nolint:gosec // G115: hi < n, and n is an int
}

// chance is true with probability pct percent: an integer in [0, 100)
// strictly below pct. At 0 and at 100 nothing is drawn (spec 4.3).
func (r *rng) chance(pct int) bool {
	if pct <= 0 {
		return false
	}
	if pct >= 100 {
		return true
	}
	return r.intn(100) < pct
}

// shuffle is Fisher-Yates from the last index down to 1, one bounded draw
// in [0, i] per step (spec 4.4).
func shuffle[T any](r *rng, s []T) {
	for i := len(s) - 1; i >= 1; i-- {
		j := r.intn(i + 1)
		s[i], s[j] = s[j], s[i]
	}
}
