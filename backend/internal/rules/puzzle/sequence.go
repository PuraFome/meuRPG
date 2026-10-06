package puzzle

import (
	"errors"
	"fmt"
	"time"
)

// The limits of a sequence, and how fast it is played.
const (
	MinBells = 3
	MaxBells = 8
	MinSteps = 3
	MaxSteps = 12
	// SequenceStep is how long each step of a sequence is shown when the master plays
	// it ("Tocar a sequência"): step 1 at once, step 2 a step later, and so on, and the
	// last one stays lit for one more step before the players may repeat it.
	SequenceStep = 1200 * time.Millisecond
)

// ErrSequence means the sequence's bells or steps are not valid.
var ErrSequence = errors.New("puzzle: the sequence is not valid")

// bells are our own eight bells (a drawing for each key, the name written beside it).
var bells = []Symbol{
	{Key: "round", NamePT: "Sino redondo"},
	{Key: "tall", NamePT: "Sino alto"},
	{Key: "wide", NamePT: "Sino largo"},
	{Key: "small", NamePT: "Sino pequeno"},
	{Key: "cracked", NamePT: "Sino rachado"},
	{Key: "thin", NamePT: "Sino fino"},
	{Key: "bent", NamePT: "Sino torto"},
	{Key: "deep", NamePT: "Sino grave"},
}

// Bells returns the first n bells, or none for an n out of range.
func Bells(n int) []Symbol {
	if n < MinBells || n > MaxBells {
		return nil
	}
	return append([]Symbol(nil), bells[:n]...)
}

// ValidateSequence checks the number of bells (MinBells to MaxBells), the steps
// (MinSteps to MaxSteps, each a bell from 0) and that at least two different bells
// ring: a sequence of one bell struck over and over is no sequence.
func ValidateSequence(nBells int, steps []int) error {
	if nBells < MinBells || nBells > MaxBells {
		return sizeError("bells", nBells, MinBells, MaxBells)
	}
	if len(steps) < MinSteps || len(steps) > MaxSteps {
		return sizeError("steps", len(steps), MinSteps, MaxSteps)
	}
	distinct := map[int]bool{}
	for _, s := range steps {
		if s < 0 || s >= nBells {
			return fmt.Errorf("%w: a step names bell %d of %d", ErrSymbols, s, nBells)
		}
		distinct[s] = true
	}
	if len(distinct) < 2 {
		return fmt.Errorf("%w: every step is the same bell", ErrSequence)
	}
	return nil
}

// SequenceStrike is one bell struck by a player, as the sequence reads it. progress
// is how many steps the attempt has right so far: the bell is the next step when it
// is steps[progress]. A right bell moves on (and the last one solves it); a wrong
// bell sends the attempt back to the first step, whatever the progress was (RN-27).
// step is the 1-based number of the step the bell was struck for.
func SequenceStrike(steps []int, progress, bell int) (next int, wrong, solved bool, step int, err error) {
	if progress < 0 || progress >= len(steps) {
		return 0, false, false, 0, fmt.Errorf("%w: the progress is %d of %d", ErrMove, progress, len(steps))
	}
	if bell < 0 {
		return 0, false, false, 0, fmt.Errorf("%w: the bell is %d", ErrMove, bell)
	}
	step = progress + 1
	if steps[progress] != bell {
		return 0, true, false, step, nil
	}
	next = progress + 1
	return next, false, next == len(steps), step, nil
}

// SequencePlayback says how much of an n-step sequence a play shows elapsed after it
// began: how many steps have been revealed (the first at once), whether it is still
// playing, and how long until the next change (the next step, or the end).
func SequencePlayback(n int, elapsed time.Duration) (shown int, playing bool, nextIn time.Duration) {
	if elapsed < 0 {
		elapsed = 0
	}
	total := time.Duration(n) * SequenceStep
	if elapsed >= total {
		return n, false, 0
	}
	shown = min(int(elapsed/SequenceStep)+1, n)
	if shown < n {
		return shown, true, time.Duration(shown)*SequenceStep - elapsed
	}
	return shown, true, total - elapsed
}
