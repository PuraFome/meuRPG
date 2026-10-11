package combat

import "github.com/PuraFome/meuRPG/backend/internal/rules"

// Jumping (MR-034, Etapa 9, D3), SRD 5.1: a long jump covers as many feet as
// the Força score with a running start (a move of at least 10 ft on foot just
// before) and half that standing; a high jump reaches 3 feet plus the Força
// modifier with a running start and half that standing. The grid, the line and
// what a long jump costs are in package rules/grid (Terrain.Jump); this file
// is what the sheet says about the jumper.
//
// Two features change the numbers (SRD 5.1): the Thief's Second-Story Work adds
// the Dexterity modifier, in feet, to a long jump with a running start (climbing
// already costs no extra movement here, since the app does not price climbing at
// all), and the monk's Step of the Wind doubles the jump distance, long and high,
// for the turn it is used in.
//
// All lengths are in tenths of a foot, like the movement.

// Feature keys the jump reads.
const (
	secondStoryWorkFeature = "feature:second-story-work"
	// StepOfTheWindFeature is the monk's feature whose Disengage or Dash (a bonus
	// action, 1 ki) doubles the jump distance for the turn.
	StepOfTheWindFeature = "feature:step-of-the-wind"
)

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
	// LongRunningBonus is what a long jump with a running start adds to
	// LongRunning (Second-Story Work); a standing jump never has it.
	LongRunningBonus int
}

// JumpLimits is what the sheet's Força gives (SRD): a long jump of Força
// feet running and half standing; a high jump of 3 + the Força modifier feet
// running (never below 0) and half standing. A Thief with Second-Story Work adds
// the Destreza modifier in feet (at least 0) to the running long jump, as
// LongRunningBonus. A sheet with no Força gives no jump.
func JumpLimits(d rules.Derived) Jumps {
	var score, mod, dexMod int
	found := false
	for _, a := range d.Abilities {
		switch a.Ability {
		case rules.STR:
			score, mod, found = a.Score, a.Modifier, true
		case rules.DEX:
			dexMod = a.Modifier
		}
	}
	if !found {
		return Jumps{}
	}
	high := max(3+mod, 0) * 10
	j := Jumps{
		LongRunning:  score * 10,
		LongStanding: score * 5,
		HighRunning:  high,
		HighStanding: high / 2,
	}
	if rules.HasFeature(d, secondStoryWorkFeature) {
		j.LongRunningBonus = max(dexMod, 0) * 10
	}
	return j
}

// Doubled is the jump distances with Step of the Wind in effect: every length
// is twice as long (the running bonus included).
func (l Jumps) Doubled() Jumps {
	return Jumps{
		LongRunning: l.LongRunning * stepOfTheWindFactor, LongStanding: l.LongStanding * stepOfTheWindFactor,
		HighRunning: l.HighRunning * stepOfTheWindFactor, HighStanding: l.HighStanding * stepOfTheWindFactor,
		LongRunningBonus: l.LongRunningBonus * stepOfTheWindFactor,
	}
}

// stepOfTheWindFactor: Step of the Wind doubles the monk's jump distance for the turn (SRD 5.1).
const stepOfTheWindFactor = 2

// standingJumpDivisor: a jump without a running start covers half the distance (SRD 5.1, Jumping).
const standingJumpDivisor = 2

// FromRunning is the jump limits a combatant carries from its sheet: the running long and high
// jumps and the Thief's running bonus, with each standing jump half of its running one.
func FromRunning(longRunning, longRunningBonus, highRunning int) Jumps {
	return Jumps{
		LongRunning: longRunning, LongStanding: longRunning / standingJumpDivisor, LongRunningBonus: longRunningBonus,
		HighRunning: highRunning, HighStanding: highRunning / standingJumpDivisor,
	}
}

// WithStepOfTheWind is Doubled when the monk used Step of the Wind this turn
// (doubled), and the jumps as they are otherwise.
func (l Jumps) WithStepOfTheWind(doubled bool) Jumps {
	if doubled {
		return l.Doubled()
	}
	return l
}

// Long is the distance of a long jump, with or without a running start.
func (l Jumps) Long(runningStart bool) int {
	if runningStart {
		return l.LongRunning + l.LongRunningBonus
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
