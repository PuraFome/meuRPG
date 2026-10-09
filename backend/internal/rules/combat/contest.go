package combat

import "slices"

// Contests and the special actions of a fight (SRD 5.1, "Contests", "Grappling",
// "Shoving a Creature", "Hide", "Help", "Working Together", "Surprise"): the pure
// arithmetic. The play module reads the combat, rolls the dice and keeps the state;
// what it asks of the rules is here, and tested without a database.

// ContestWinner is who won a contest.
type ContestWinner int

const (
	// ContestTie changes nothing: the situation stays as it was (SRD 5.1,
	// "Contests").
	ContestTie ContestWinner = iota
	// ContestInitiator is the one that started it.
	ContestInitiator
	// ContestDefender is the one that answered.
	ContestDefender
)

// CompareContest decides a contest by the two check totals: the higher wins, and a tie
// is nobody's (SRD 5.1, "Contests").
func CompareContest(initiator, defender int) ContestWinner {
	switch {
	case initiator > defender:
		return ContestInitiator
	case defender > initiator:
		return ContestDefender
	}
	return ContestTie
}

// MeetsEscapeDC says whether a check total gets a creature out of a grapple that has a
// fixed escape DC: a check succeeds when its total reaches the DC (SRD 5.1, "Ability
// Checks").
func MeetsEscapeDC(total, dc int) bool { return total >= dc }

// The sizes in order, smallest first (SRD 5.1, "Creature Size").
var sizeOrder = []string{"tiny", "small", "medium", "large", "huge", "gargantuan"}

// SizeRank is a size's place among the SRD's sizes (tiny 0 to gargantuan 5); a name the
// table does not know is medium.
func SizeRank(size string) int {
	if i := slices.Index(sizeOrder, size); i >= 0 {
		return i
	}
	return slices.Index(sizeOrder, "medium")
}

// CanGrappleOrShove says whether a creature of one size may grapple or shove another:
// the target is no more than one size larger (SRD 5.1, "Grappling" and "Shoving a
// Creature"). Any smaller target is allowed.
func CanGrappleOrShove(attackerSize, targetSize string) bool {
	return SizeRank(targetSize) <= SizeRank(attackerSize)+1
}

// DragHalvesSpeed says whether dragging or carrying a grappled creature halves the
// grappler's speed: it does, unless the creature is two or more sizes smaller (SRD 5.1,
// "Moving a Grappled Creature").
func DragHalvesSpeed(grapplerSize, grappledSize string) bool {
	return SizeRank(grappledSize) > SizeRank(grapplerSize)-2
}

// MeleeReachFt is the reach of a melee attack that says none, in feet: the reach a
// grapple and a shove have (SRD 5.1, "Melee Attacks": most creatures have 5 ft).
const MeleeReachFt = 5

// PushStep is the square 5 feet straight away from the shover: one step from the target
// along the line from the shover to the target, a diagonal when the two are diagonal
// (SRD 5.1, "Shoving a Creature").
func PushStep(shoverCol, shoverRow, targetCol, targetRow int) (col, row int) {
	return targetCol + sign(targetCol-shoverCol), targetRow + sign(targetRow-shoverRow)
}

func sign(n int) int {
	switch {
	case n > 0:
		return 1
	case n < 0:
		return -1
	}
	return 0
}

// CheckMode is how a d20 of a check is rolled (SRD 5.1, "Advantage and Disadvantage").
type CheckMode int

const (
	// CheckNormal is one d20.
	CheckNormal CheckMode = iota
	// CheckAdvantage rolls two d20 and keeps the higher.
	CheckAdvantage
	// CheckDisadvantage rolls two d20 and keeps the lower.
	CheckDisadvantage
)

// CheckSource is one circumstance that gives a check advantage or disadvantage.
type CheckSource struct {
	// Kind is a stable key ("help", "unseen_attacker", "poisoned").
	Kind string
	// Advantage is true for a source of advantage, false for one of disadvantage.
	Advantage bool
}

// ResolveCheckMode is the mode the sources make together: any number of one side and
// none of the other is that side; at least one of each is a normal roll, even if only
// one is on the other side (SRD 5.1, "Advantage and Disadvantage").
func ResolveCheckMode(sources []CheckSource) CheckMode {
	var adv, dis bool
	for _, s := range sources {
		if s.Advantage {
			adv = true
		} else {
			dis = true
		}
	}
	switch {
	case adv && !dis:
		return CheckAdvantage
	case dis && !adv:
		return CheckDisadvantage
	}
	return CheckNormal
}

// Dice is how many d20 the mode rolls: one for a normal roll, two for the others.
func (m CheckMode) Dice() int {
	if m == CheckNormal {
		return 1
	}
	return 2
}

// PickD20 is the d20 that counts among the faces rolled: the higher for advantage, the
// lower for disadvantage, the only one for a normal roll.
func (m CheckMode) PickD20(faces []int) int {
	if len(faces) == 0 {
		return 0
	}
	best := faces[0]
	for _, f := range faces[1:] {
		switch {
		case m == CheckAdvantage && f > best, m == CheckDisadvantage && f < best:
			best = f
		}
	}
	return best
}

// PassivePerceptionOf is a creature's passive Wisdom (Perception): 10 plus the check's
// modifiers, plus 5 for advantage and minus 5 for disadvantage (SRD 5.1, "Passive
// Checks"). perceptionBonus is the Perception bonus the sheet or the stat block gives.
func PassivePerceptionOf(perceptionBonus int, mode CheckMode) int {
	score := 10 + perceptionBonus
	switch mode {
	case CheckAdvantage:
		score += 5
	case CheckDisadvantage:
		score -= 5
	case CheckNormal:
	}
	return score
}

// NoticesHider says whether a creature notices a hider: the hider's Dexterity (Stealth)
// total has to beat the creature's passive Perception (SRD 5.1, "Hiding", "Passive
// Perception"). The SRD does not say what a tie does; the app keeps the status quo of
// the contests: a tie keeps the hider noticed.
func NoticesHider(stealthTotal, passivePerception int) bool {
	return stealthTotal <= passivePerception
}

// GroupCheckNeeded is how many characters must pass for the group to pass: at least
// half of the group (SRD 5.1, "Group Checks"), so the half rounded up. A group of none
// needs none.
func GroupCheckNeeded(asked int) int {
	return (asked + 1) / 2
}

// GroupCheckPasses says whether a group passes: at least half of the characters asked
// passed. The characters that did not answer count as failed.
func GroupCheckPasses(passed, asked int) bool {
	return asked > 0 && passed >= GroupCheckNeeded(asked)
}

// HideTotals is one hider's Stealth total, for the surprise suggestion.
type HideTotals struct {
	// ID identifies the hider.
	ID    string
	Total int
}

// NoticesNoThreat says whether a creature notices none of the hiders: every hider's
// Stealth beats its passive Perception (SRD 5.1, "Surprise": any creature that does not
// notice a threat is surprised). With nobody hiding, the creature notices them: it is
// not surprised.
func NoticesNoThreat(passivePerception int, hiders []HideTotals) bool {
	if len(hiders) == 0 {
		return false
	}
	for _, h := range hiders {
		if NoticesHider(h.Total, passivePerception) {
			return false
		}
	}
	return true
}

// HelpLastsThrough says whether a Help made in round `made` still holds when the combat
// is in round `round` and the turn that is running belongs to the combatant at `current`
// in the order, the helper being at `helper`. It lasts through the end of the helper's
// next turn: the turn of the round after the one it was made in (the app's reading of
// "until the end of the helper's next turn").
func HelpLastsThrough(made, round, current, helper int) bool {
	expires := made + 1
	switch {
	case round < expires:
		return true
	case round == expires:
		return current <= helper
	}
	return false
}

// SurprisedUntilTurnEnds says whether a surprised creature is still surprised: the
// combat is not running yet, or its turn of the first round has not ended (SRD 5.1,
// "Surprise": it cannot move, act or react until that turn ends). round is the combat's
// round, 0 while it is being set up; current and own are the places in the order of the
// turn that is running and of the creature.
func SurprisedUntilTurnEnds(running bool, round, current, own int) bool {
	switch {
	case !running:
		return true
	case round > 1:
		return false
	}
	return current <= own
}
