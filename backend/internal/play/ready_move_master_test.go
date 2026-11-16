package play

import (
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// Who releases a readied move, and what a release that goes nowhere costs (ready_answer.go, readiedMoveOf and
// spendReadiedMove): the player's own off-turn walk is the release; the master's plain walk is not, unless the request says so
// (release_readied); a walk stopped before its first square spends nothing.

// release is the master's flag: this walk is the release of the readied move.
func release(r *playv1.MoveCombatantRequest) { r.ReleaseReadied = true }

// The master's plain walk of the readier is a token moved: free, no limit, the window untouched and the reaction free,
// and none of the release's own checks apply to it (the release's are in the next test).
func TestTheMastersPlainWalkOfTheReadierIsNotTheRelease(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.readyMoveFrom(t, 1, 5)
	a.mustFire(t, e, "Toren")
	if _, err := a.frozenMove(t, a.master, "Toren", 12, 5); err != nil { // 110 ft: far past the 30 ft of the readied move
		t.Fatalf("the master's long drag of the readier: %v", err)
	}
	if got := a.squareOf(t, "Toren"); got != [2]int32{12, 5} {
		t.Errorf("Toren after the drag is at %v, want (12,5)", got)
	}
	a.untouched(t, e, "the master's plain walk")
	if _, err := a.frozenMove(t, a.master, "Toren", 12, 5); err != nil { // to the square it stands on: a token put right
		t.Fatalf("the master's walk of the readier to its own square: %v", err)
	}
	a.untouched(t, e, "the master's walk to the readier's own square")
}

// With release_readied the master's walk is the release, made for the player: it spends the reaction and fires the row as the
// player's own move does, within the same budget (a walk past it is TOO_FAR and leaves the window), and its undo gives it
// all back. The release's own checks hold: a spent reaction refuses it.
func TestTheMastersWalkWithReleaseReadiedIsTheRelease(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.readyMoveFrom(t, 1, 5)
	a.mustFire(t, e, "Toren")
	_, err := a.frozenMove(t, a.master, "Toren", 8, 5, release) // 35 ft
	wantBlockedBy(t, "the master's release of 35 ft", err, reasonTooFar)
	a.untouched(t, e, "the refusal")
	a.execSQL(t, `UPDATE combatants SET reaction_used = true WHERE label = 'Toren'`)
	_, err = a.frozenMove(t, a.master, "Toren", 4, 5, release)
	wantBlockedBy(t, "the master's release with the reaction spent", err, blockedReactionUsed)
	a.execSQL(t, `UPDATE combatants SET reaction_used = false WHERE label = 'Toren'`)
	if _, err := a.frozenMove(t, a.master, "Toren", 4, 5, release); err != nil { // 15 ft
		t.Fatalf("the master's release: %v", err)
	}
	toren := byLabel(t, a.get(t, a.caio), "Toren")
	if !toren.GetReactionUsed() || toren.GetReactionKind() != kindReadied || a.readiedStatus(t, "Toren") != readiedFired || a.readiedOpen(t, "Toren") {
		t.Errorf("after the release: reaction %v (%v), row %q, window open %v", toren.GetReactionUsed(), toren.GetReactionKind(), a.readiedStatus(t, "Toren"), a.readiedOpen(t, "Toren"))
	}
	if lines := a.firedLines(t, a.master, e); len(lines) != 1 || lines[0].GetReadiedFired().GetKind() != playv1.ReadiedKind_READIED_KIND_MOVE {
		t.Errorf("the fired lines = %v", lines)
	}
	a.mustUndo(t, e)
	if a.squareOf(t, "Toren") != [2]int32{1, 5} {
		t.Errorf("Toren after the undo is at %v, want (1,5)", a.squareOf(t, "Toren"))
	}
	a.untouched(t, e, "the undo of the master's release")
}

// release_readied asks for a release: with no open window of a move for the combatant, or on a move that is no plain walk, it is
// refused as a release with no window is (NOT_YOUR_TURN) and writes nothing. A player sending it is harmless: their own move
// is the release anyway.
func TestReleaseReadiedNeedsAnOpenMoveWindow(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.readyMove(t) // the row waits, no window yet
	before := a.events(t)
	_, err := a.frozenMove(t, a.master, "Toren", 5, 3, release)
	wantBlockedBy(t, "the master's release with no window", err, blockedNotYourTurn)
	a.mustFire(t, e, "Toren")
	_, err = a.frozenMove(t, a.master, "Toren", 5, 5, release, forced)
	wantBlockedBy(t, "the master's forced move as a release", err, blockedNotYourTurn)
	if n := a.events(t); n != before+1 || a.squareOf(t, "Toren") != [2]int32{3, 3} { // the fire is the one event
		t.Errorf("after the refusals: events %d (want %d), at %v", n, before+1, a.squareOf(t, "Toren"))
	}
	a.untouched(t, e, "the refusals")
	if _, err := a.frozenMove(t, a.caio, "Toren", 3, 5, release); err != nil {
		t.Fatalf("the player's move with release_readied: %v", err)
	}
	if a.readiedStatus(t, "Toren") != readiedFired {
		t.Errorf("row after the player's move = %q, want fired", a.readiedStatus(t, "Toren"))
	}
}

// A release that moves nowhere spends nothing (SRD 5.1, Ready: the move is the reaction, and none was made): a walk stopped before
// its first square by a creature the player does not see tells "stopped early", spends no reaction, and leaves the row waiting
// with its window open. A walk cut short after a square is still the use.
func TestAReadiedMoveStoppedAtItsFirstStepSpendsNothing(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.readyMove(t)
	a.mustFire(t, e, "Toren")
	a.execSQL(t, `UPDATE combatants SET hidden = true WHERE label = 'Goblin'`) // east of Toren, on the way
	res, err := a.frozenMove(t, a.caio, "Toren", 6, 3)
	if err != nil || !res.GetStoppedEarly() {
		t.Fatalf("the walk into the hidden goblin = %v, %v; want stopped early", res, err)
	}
	if got := a.squareOf(t, "Toren"); got != [2]int32{3, 3} {
		t.Errorf("Toren is at %v, want (3,3)", got)
	}
	a.untouched(t, e, "the walk stopped at its first step")
	res, err = a.frozenMove(t, a.caio, "Toren", 3, 1) // an open lane: the use
	if err != nil || res.GetStoppedEarly() || a.readiedStatus(t, "Toren") != readiedFired {
		t.Errorf("the next walk = %v, %v, row %q; want the use", res, err, a.readiedStatus(t, "Toren"))
	}
}

func TestAReadiedMoveStoppedAfterASquareIsTheUse(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.readyMoveFrom(t, 1, 5)
	a.mustFire(t, e, "Toren")
	a.execSQL(t, `UPDATE combatants SET hidden = true, grid_col = 4, grid_row = 5 WHERE label = 'Goblin'`)
	res, err := a.frozenMove(t, a.caio, "Toren", 6, 5)
	if err != nil || !res.GetStoppedEarly() || a.squareOf(t, "Toren") != [2]int32{3, 5} {
		t.Fatalf("the walk into the hidden goblin = %v, %v at %v; want stopped early on (3,5)", res, err, a.squareOf(t, "Toren"))
	}
	if a.readiedStatus(t, "Toren") != readiedFired || a.readiedOpen(t, "Toren") || !byLabel(t, a.get(t, a.master), "Toren").GetReactionUsed() {
		t.Errorf("after the walk cut short: row %q, window open %v; want the use", a.readiedStatus(t, "Toren"), a.readiedOpen(t, "Toren"))
	}
}
