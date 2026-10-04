package combat

import "github.com/PuraFome/meuRPG/backend/internal/rules"

// Jumping (MR-034, Etapa 9, D3), SRD 5.1: a long jump covers as many feet as
// the Força score with a running start (a move of at least 10 ft on foot just
// before) and half that standing; a high jump reaches 3 feet plus the Força
// modifier with a running start and half that standing. The grid, the line and
// what a long jump costs are in package rules/grid (Terrain.Jump); this file
// is what the sheet says about the jumper.
//
// All lengths are in tenths of a foot, like the movement.

// RunningStartDFt is the move right before a jump, in tenths of a foot, that
// counts as a running start: 10 ft (SRD).
const RunningStartDFt = 100

// Jumps are how far and how high a character can jump, in tenths of a
// foot.
type Jumps struct {
	// LongRunning and LongStanding are the distance of a long jump with and
	// without a running start.
	LongRunning, LongStanding int
	// HighRunning and HighStanding are the height of a high jump with and
	// without a running start. Never below 0.
	HighRunning, HighStanding int
}

// JumpLimits is what the sheet's Força gives (SRD): a long jump of Força
// feet running and half standing; a high jump of 3 + the Força modifier feet
// running (never below 0) and half standing. A sheet with no Força gives no
// jump.
func JumpLimits(d rules.Derived) Jumps {
	var score, mod int
	found := false
	for _, a := range d.Abilities {
		if a.Ability == rules.STR {
			score, mod, found = a.Score, a.Modifier, true
		}
	}
	if !found {
		return Jumps{}
	}
	high := max(3+mod, 0) * 10
	return Jumps{
		LongRunning:  score * 10,
		LongStanding: score * 5,
		HighRunning:  high,
		HighStanding: high / 2,
	}
}

// Long is the distance of a long jump, with or without a running start.
func (l Jumps) Long(runningStart bool) int {
	if runningStart {
		return l.LongRunning
	}
	return l.LongStanding
}

// High is the height of a high jump, with or without a running start.
func (l Jumps) High(runningStart bool) int {
	if runningStart {
		return l.HighRunning
	}
	return l.HighStanding
}

// HasRunningStart says whether the move right before a jump, this turn, was
// long enough (10 ft or more) and on foot, so the jump may use the running
// numbers. The caller says how far that move was; a jump that follows a jump,
// a fly or no move at all passes 0.
func HasRunningStart(previousMoveDFt int) bool {
	return previousMoveDFt >= RunningStartDFt
}
