// Package dicetest is the dice a test hands to a service that rolls (play, characters).
// It is a dice.Roller like production's dice.Crypto, but deterministic, so a test never meets a roll it did not ask for.
//
// It exists because the app rolls an NPC's initiative itself (RN-19).
// Two creatures with the same total take one joint turn (MR-013).
// With random dice, a test that assumes one creature acts fails about one run in twenty for every pair with the same bonus.
//
// Only test files import this package.
// Production wires dice.Crypto.
// The archtest suite fails for any non-test file outside this package that imports it.
package dicetest

import (
	"fmt"
	"maps"
	"slices"
	"sync"
	"testing"
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

// Roller gives the faces a test scripted with ScriptFor, in order, and then a default sequence that never runs out.
// The zero value is ready to use.
// A face scripted and never rolled fails the test when it ends, so a script cannot reach a roll it was not meant for.
// A Roller is safe for concurrent use, and the order faces go out in is the order the rolls happen in.
//
// What it guarantees:
//   - It is deterministic: the same calls give the same faces, whatever the clock or the machine.
//   - It never fails because it ran out, unlike dice.Fixed.
//   - On a Roller that has rolled no other d20 yet, up to 20 creatures with the same initiative bonus never tie.
//   - On such a Roller, four creatures whose bonuses are within 5 of one another never tie either.
//
// What it does not guarantee:
//   - Every default d20 roll takes the next face of the sequence, not only the initiative's.
//     An attack or a save rolled before the initiative moves the sequence and voids the two guarantees above.
//   - Creatures with different bonuses can tie.
//     Faces run from 1 to 20, so for arbitrary bonuses from -5 to +10 no sequence keeps eight creatures apart.
//     A test that needs a strict order scripts the faces.
//   - The default dice are not random and not fair, so do not test odds with them.
type Roller struct {
	mu      sync.Mutex
	script  map[int][]int // faces queued by ScriptFor, by die, used first
	watched testing.TB    // the test whose end checks for faces left over
	d20     int           // how many default d20 faces went out
	other   uint64        // state of the default sequence of the other dice
}

// New returns a Roller with nothing scripted.
func New() *Roller { return &Roller{} }

// ScriptFor queues faces for the next rolls of a die with that many sides, in order.
// A roll of another die does not use them.
// A face that does not fit the die fails the test at once.
// When they are used up, the default sequence goes on where it was.
// A face still queued when the test ends fails it, naming the die and the faces.
func (r *Roller) ScriptFor(t testing.TB, sides int, faces ...int) {
	t.Helper()
	if sides < 1 {
		t.Fatalf("dicetest: a d%d does not exist", sides)
	}
	for _, face := range faces {
		if face < 1 || face > sides {
			t.Fatalf("dicetest: face %d does not fit a d%d", face, sides)
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.script == nil {
		r.script = map[int][]int{}
	}
	r.script[sides] = append(r.script[sides], faces...)
	if r.watched != t {
		r.watched = t
		t.Cleanup(func() { r.failIfLeftOver(t) })
	}
}

// failIfLeftOver fails t for each die whose scripted faces were not all rolled.
func (r *Roller) failIfLeftOver(t testing.TB) {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, sides := range slices.Sorted(maps.Keys(r.script)) {
		if left := r.script[sides]; len(left) > 0 {
			t.Errorf("dicetest: the faces %v scripted for the d%d were never rolled: script only the rolls the test makes", left, sides)
		}
	}
}

// Roll implements dice.Roller.
func (r *Roller) Roll(sides int) (int, error) {
	if sides < 1 {
		return 0, fmt.Errorf("dicetest: a d%d does not exist", sides)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if queue := r.script[sides]; len(queue) > 0 {
		r.script[sides] = queue[1:]
		return queue[0], nil
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
