// Package dicetest is the dice a test hands to a service that rolls (play,
// characters): the same dice.Roller as production's dice.Crypto, but
// deterministic, so a test never meets a roll it did not ask for.
//
// It exists because the app rolls an NPC's initiative itself (RN-19) and two
// creatures with the same total take one joint turn (MR-013): with random
// dice, a test that assumes one creature acts fails about one run in twenty
// for every pair of creatures with the same bonus.
//
// Only test files and test harnesses import this package. Production wires
// dice.Crypto, and the archtest suite fails the build of any non-test file
// outside this package that imports it.
package dicetest

import (
	"fmt"
	"sync"
)

const d20Sides = 20

// d20Cycle is the order the default d20 faces come out in: 20, 14, 8, 2, then
// 19, 13, 7, 1, then 18, 12, 6, and so on. It holds each face from 1 to 20
// once, and the first four are at least 6 apart.
var d20Cycle = func() []int {
	var out []int
	for start := d20Sides; start > d20Sides-6; start-- {
		for face := start; face >= 1; face -= 6 {
			out = append(out, face)
		}
	}
	return out
}()

// Roller gives the faces a test scripted, in order, and then a default
// sequence that never runs out. The zero value is ready to use, and a Roller
// is safe for concurrent use (the order faces go out in is the order the
// rolls happen in).
//
// What it guarantees:
//   - It is deterministic: two Rollers given the same calls give the same
//     faces, whatever the clock or the machine.
//   - It never fails because it ran out (unlike dice.Fixed).
//   - Default d20 faces repeat only after 20 d20 rolls, so up to 20 creatures
//     with the same initiative bonus never tie.
//   - The first four default d20 faces are at least 6 apart, so four
//     creatures whose bonuses are within 5 of one another never tie either.
//
// What it does not:
//   - Creatures with different bonuses can tie. With faces limited to 1..20,
//     no sequence can stop that for eight creatures with bonuses from -5 to
//     +10: it would need faces further apart than the 19 a d20 has room for.
//     A test that adds creatures with very different bonuses and needs a
//     strict order scripts their faces.
//   - Every default d20 roll takes the next face of the sequence, not only
//     the initiative's: an attack or a save rolled before the initiative moves
//     it. A test that needs a particular d20 scripts it.
//   - The default dice are not random and not fair: do not test odds with it.
type Roller struct {
	mu     sync.Mutex
	script []int  // faces queued by Script, used first
	d20    int    // how many default d20 faces went out
	other  uint64 // state of the default sequence of the other dice
}

// New returns a Roller with nothing scripted.
func New() *Roller { return &Roller{} }

// Script queues faces to come out next, in order, for whatever dice are rolled
// next: a face that does not fit its die makes that roll fail, as dice.Fixed
// does, so a test that scripts the wrong roll notices. Once they are used, the
// default sequence goes on where it was.
func (r *Roller) Script(faces ...int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.script = append(r.script, faces...)
}

// Roll implements dice.Roller.
func (r *Roller) Roll(sides int) (int, error) {
	if sides < 1 {
		return 0, fmt.Errorf("dicetest: a d%d does not exist", sides)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.script) > 0 {
		face := r.script[0]
		r.script = r.script[1:]
		if face < 1 || face > sides {
			return 0, fmt.Errorf("dicetest: scripted face %d does not fit a d%d", face, sides)
		}
		return face, nil
	}
	if sides == d20Sides {
		face := d20Cycle[r.d20%len(d20Cycle)]
		r.d20++
		return face, nil
	}
	return 1 + int(r.next()>>33)%sides, nil
}

// next steps the default sequence of the other dice (splitmix64 from a fixed
// seed): it needs no randomness, only faces that do not all look alike.
func (r *Roller) next() uint64 {
	r.other += 0x9e3779b97f4a7c15
	z := r.other
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}
