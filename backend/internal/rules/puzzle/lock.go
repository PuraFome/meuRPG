package puzzle

import "fmt"

// The combination lock (MR-038): 2 to 6 wheels, each turning through an alphabet
// of digits, letters or our own runes. The master sets the solution and where
// the wheels start. A move turns one wheel one position up or down, wrapping
// around the alphabet. It is solved when every wheel shows its solution.

// The numbers of wheels.
const (
	MinWheels = 2
	MaxWheels = 6
)

// Alphabet is what a wheel shows.
type Alphabet int

// The alphabets.
const (
	Digits  Alphabet = iota + 1 // 0 to 9
	Letters                     // A to Z
	Runes                       // our own eight runes
)

// runes are the eight runes of the table: ours, drawn for this app, named in
// Portuguese. They copy no published game's alphabet.
var runes = []Symbol{
	{"moon", "Lua"},
	{"flame", "Chama"},
	{"wave", "Onda"},
	{"root", "Raiz"},
	{"eye", "Olho"},
	{"star", "Estrela"},
	{"stone", "Pedra"},
	{"leaf", "Folha"},
}

var (
	digits  = makeSymbols("0123456789")
	letters = makeSymbols("ABCDEFGHIJKLMNOPQRSTUVWXYZ")
)

func makeSymbols(chars string) []Symbol {
	out := make([]Symbol, 0, len(chars))
	for _, c := range chars {
		out = append(out, Symbol{Key: string(c), NamePT: string(c)})
	}
	return out
}

// Symbols returns the alphabet's faces in the order a wheel turns through them,
// nil for an alphabet that does not exist. Do not change the slice.
func (a Alphabet) Symbols() []Symbol {
	switch a {
	case Digits:
		return digits
	case Letters:
		return letters
	case Runes:
		return runes
	}
	return nil
}

// Size is how many faces the alphabet has: 10, 26 or 8.
func (a Alphabet) Size() int { return len(a.Symbols()) }

// ValidateLock checks a lock: its wheels, its alphabet, the solution and the
// start (one position per wheel, inside the alphabet), and that the start is
// not already the solution.
func ValidateLock(wheels int, a Alphabet, solution, start []int) error {
	if wheels < MinWheels || wheels > MaxWheels {
		return sizeError("the lock's wheels", wheels, MinWheels, MaxWheels)
	}
	if a.Size() == 0 {
		return fmt.Errorf("%w: unknown alphabet %d", ErrSymbols, a)
	}
	for _, p := range [][]int{solution, start} {
		if len(p) != wheels {
			return fmt.Errorf("%w: %d positions for %d wheels", ErrSymbols, len(p), wheels)
		}
		for _, v := range p {
			if v < 0 || v >= a.Size() {
				return fmt.Errorf("%w: position %d of an alphabet of %d", ErrSymbols, v, a.Size())
			}
		}
	}
	if LockSolved(start, solution) {
		return ErrSolved
	}
	return nil
}

// LockTurn turns the wheel one position (delta +1 or -1), wrapping around the
// alphabet, and returns the new wheels; the argument is not changed.
func LockTurn(a Alphabet, state []int, wheel, delta int) ([]int, error) {
	if wheel < 0 || wheel >= len(state) || (delta != 1 && delta != -1) || a.Size() == 0 {
		return nil, fmt.Errorf("%w: wheel %d by %d", ErrMove, wheel, delta)
	}
	next := append([]int(nil), state...)
	next[wheel] = mod(next[wheel]+delta, a.Size())
	return next, nil
}

// LockSolved says whether every wheel shows its solution.
func LockSolved(state, solution []int) bool {
	if len(state) != len(solution) {
		return false
	}
	for i := range state {
		if state[i] != solution[i] {
			return false
		}
	}
	return true
}

// LockMinimum is the fewest turns that solve the lock, and one way to make them:
// each wheel goes the short way round (up on a tie).
func LockMinimum(a Alphabet, state, solution []int) (moves int, path []LockStep) {
	size := a.Size()
	for w := range min(len(state), len(solution)) {
		up := mod(solution[w]-state[w], size)
		down := size - up
		switch {
		case up == 0:
		case up <= down:
			moves += up
			for range up {
				path = append(path, LockStep{Wheel: w, Delta: 1})
			}
		default:
			moves += down
			for range down {
				path = append(path, LockStep{Wheel: w, Delta: -1})
			}
		}
	}
	return moves, path
}

// LockStep is one turn of a wheel.
type LockStep struct {
	Wheel int
	Delta int
}
