// Package puzzle is the pure rules of the table's puzzles (MR-038, RN-27):
// "Apagar as luzes", the combination lock and the turning symbols. Like the rest
// of package rules it has no database, no clock and no randomness: a seed is an
// input, and the same seed always gives the same start.
//
// Every move is relative (press a cell, turn a wheel or a pillar by one), never
// "set this to that", so two players' moves made at the same time commute: the
// state after both is the same whichever the server applied first. That is what
// lets the server take them without asking anyone to wait.
//
// What a player never receives (RN-10) is decided by the service in package
// play; this package only computes: the minimum number of moves and one way to
// make them are for the master's eyes.
package puzzle

import (
	"errors"
	"fmt"
)

// The reasons a puzzle's configuration or a move is refused. They are errors.Is
// targets: the service turns each into the typed reason the app reads.
var (
	// ErrSize: the board, the wheels or the pillars are out of the range allowed.
	ErrSize = errors.New("puzzle: the size is out of range")
	// ErrSymbols: the number of symbols is out of range, or a position is not one of them.
	ErrSymbols = errors.New("puzzle: a symbol is out of range")
	// ErrLinks: a pillar's links name a pillar that does not exist, itself, or twice the same.
	ErrLinks = errors.New("puzzle: a link is not valid")
	// ErrSolved: the start is already the solution.
	ErrSolved = errors.New("puzzle: the start is already solved")
	// ErrUnsolvable: no sequence of moves reaches the solution.
	ErrUnsolvable = errors.New("puzzle: the start cannot be solved")
	// ErrMove: a move names a cell, a wheel or a pillar that does not exist, or turns by something else than one.
	ErrMove = errors.New("puzzle: the move is not valid")
)

func sizeError(what string, got, lo, hi int) error {
	return fmt.Errorf("%w: %s is %d, want %d to %d", ErrSize, what, got, lo, hi)
}

// Symbol is one face of a wheel or a pillar: a stable key the app draws a picture
// for, and our own Portuguese name (the name is always written beside the picture).
type Symbol struct {
	Key    string
	NamePT string
}

// rng is a splitmix64 generator: small, fast and the same everywhere, so a seed
// reproduces a start on any machine and any Go version.
type rng struct{ s uint64 }

func newRNG(seed uint64) *rng { return &rng{s: seed} }

func (r *rng) next() uint64 {
	r.s += 0x9e3779b97f4a7c15
	z := r.s
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

// intn returns a number from 0 to n-1 (n > 0). The tiny bias of a remainder is
// harmless here: it picks a start, not a prize.
func (r *rng) intn(n int) int {
	return int(r.next() % uint64(n)) //nolint:gosec // n is a small positive number, the remainder fits an int
}

// mod is a % n, never negative, for wrapping a position around a wheel.
func mod(a, n int) int { return ((a % n) + n) % n }
