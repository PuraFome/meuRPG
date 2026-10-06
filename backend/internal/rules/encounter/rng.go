package encounter

// rng is a small deterministic generator (splitmix64) over a 32-bit seed. The
// package needs a handful of draws per encounter, so a few lines are enough, and
// the same seed always gives the same sequence on every machine.
type rng struct{ s uint64 }

func newRNG(seed uint32) rng {
	// Mixing in a constant keeps seed 0 from starting at state 0.
	return rng{s: uint64(seed) + 0x9e3779b97f4a7c15}
}

func (r *rng) next() uint64 {
	r.s += 0x9e3779b97f4a7c15
	z := r.s
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

// intn is a number from 0 to n-1 (n > 0). The tiny bias of the remainder is
// harmless for choosing a creature.
func (r *rng) intn(n int) int { return int(r.next() % uint64(n)) } //nolint:gosec // n is a small count
