package play

import (
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The release of a readied move (SRD 5.1 Ready, "move up to your speed in response to a trigger"): MoveCombatant by the
// readier, or the master for it, while its READIED window of a move is open (ready_answer.go, readiedMoveOf). What the
// budget is, what the use spends and closes, what leaves the window open, and who may release.

// readyMoveRequest is the request that has Toren ready a move.
func (a *armed) readyMoveRequest(t *testing.T, e *playv1.Encounter) *playv1.ReadyActionRequest {
	t.Helper()
	return &playv1.ReadyActionRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, "Toren"), IdempotencyKey: newKey(),
		TriggerText: readyTrigger, Kind: playv1.ReadiedKind_READIED_KIND_MOVE,
	}
}

// readyMoveFrom puts Toren on a square of an open lane (a forced move of the master, which provokes nothing), has him ready a move
// and ends his turn: Pensantus acts. walk squares are walked on his turn before he readies, if any.
func (a *armed) readyMoveFrom(t *testing.T, col, row int32, walk ...[2]int32) *playv1.Encounter {
	t.Helper()
	e := a.threeAndAGoblin(t)
	if _, err := a.frozenMove(t, a.master, "Toren", col, row, func(r *playv1.MoveCombatantRequest) { r.Forced = true }); err != nil {
		t.Fatalf("the master puts Toren on (%d,%d): %v", col, row, err)
	}
	for _, sq := range walk {
		if _, err := a.frozenMove(t, a.caio, "Toren", sq[0], sq[1]); err != nil {
			t.Fatalf("Toren's walk on his turn to %v: %v", sq, err)
		}
	}
	a.mustReady(t, a.caio, a.readyMoveRequest(t, e))
	return a.mustEndTurn(t, a.master, e)
}

// untouched says nothing of the readied move was used: Toren's reaction is free, the row waits, its window is open, and the log has no
// readied_fired line.
func (a *armed) untouched(t *testing.T, e *playv1.Encounter, what string) {
	t.Helper()
	if byLabel(t, a.get(t, a.master), "Toren").GetReactionUsed() || a.readiedStatus(t, "Toren") != readiedWaiting || !a.readiedOpen(t, "Toren") || len(a.firedLines(t, a.master, e)) != 0 {
		t.Errorf("after %s: reaction used %v, row %q, window open %v, fired lines %d; want the window as it was",
			what, byLabel(t, a.get(t, a.master), "Toren").GetReactionUsed(), a.readiedStatus(t, "Toren"), a.readiedOpen(t, "Toren"), len(a.firedLines(t, a.master, e)))
	}
}

// The budget is the speed Toren had at Ready, whole: the movement he walked on his own turn before readying is not taken
// from it (move_ft is the fresh speed). The turn's movement stays what it was.
func TestAReadiedMoveBudgetIsTheWholeSpeedAtReadyWhateverTheTurnWalked(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.readyMoveFrom(t, 1, 5, [2]int32{1, 6}) // 5 ft on his turn, then the lane of row 6 to the east
	spent := a.moveRowOf(t, "Toren").UsedDft
	if spent == 0 {
		t.Fatal("Toren's walk spent no movement")
	}
	if got := byLabel(t, a.get(t, a.master), "Toren").GetReadied().GetMoveFt(); got != 30 {
		t.Fatalf("move_ft at Ready after walking %d dft = %d, want his whole speed, 30", spent, got)
	}
	a.mustFire(t, e, "Toren")
	opts, err := a.readiedMoveOptions(t, e)
	if err != nil || opts.GetMovementLeftDft() != 300 {
		t.Fatalf("GetMoveOptions off turn = %v, %v; want 300 dft", opts.GetMovementLeftDft(), err)
	}
	_, err = a.frozenMove(t, a.caio, "Toren", 8, 6) // 35 ft
	wantBlockedBy(t, "a readied move of 35 ft", err, reasonTooFar)
	a.untouched(t, e, "the refusal")
	if _, err := a.frozenMove(t, a.caio, "Toren", 7, 6); err != nil { // 30 ft
		t.Fatalf("the readied move of 30 ft after a walk on the turn: %v", err)
	}
	if got := a.moveRowOf(t, "Toren").UsedDft; got != spent {
		t.Errorf("the turn's movement after the readied move = %d, want %d", got, spent)
	}
}

// The speed is read again when the window is open, and held to move_ft: a speed that fell (an effect, a condition) shrinks the
// budget, one that rose does not widen it. GetMoveOptions and MoveCombatant agree each time, and a refusal leaves the window.
func TestAReadiedMoveBudgetFollowsTheSpeedNowButNeverPastMoveFt(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.readyMoveFrom(t, 1, 5) // the lane of row 5 to the east
	a.mustFire(t, e, "Toren")
	for _, c := range []struct {
		name, set, reset string
		leftDFt          int32
		tooFar           int32 // the first column of the lane the budget does not reach
	}{
		{"a speed that doubled", `speed_ft = 60`, `speed_ft = 30`, 300, 8},
		{"a speed that halved", `effect_speed_pct = 50`, `effect_speed_pct = 0`, 150, 5},
		{"a speed of 15", `speed_ft = 15`, `speed_ft = 30`, 150, 5},
		{"grappled", `conditions = ARRAY['condition:grappled']`, `conditions = '{}'`, 0, 2},
	} {
		a.execSQL(t, `UPDATE combatants SET `+c.set+` WHERE label = 'Toren'`)
		opts, err := a.readiedMoveOptions(t, e)
		if err != nil || opts.GetMovementLeftDft() != c.leftDFt {
			t.Errorf("GetMoveOptions with %s = %v, %v; want %d dft", c.name, opts.GetMovementLeftDft(), err, c.leftDFt)
		}
		_, err = a.frozenMove(t, a.caio, "Toren", c.tooFar, 5)
		wantBlockedBy(t, "a readied move past the budget with "+c.name, err, reasonTooFar)
		a.execSQL(t, `UPDATE combatants SET `+c.reset+` WHERE label = 'Toren'`)
		a.untouched(t, e, c.name)
	}
}

// Difficult terrain costs a readied move what it costs a move on turn: the squares of rubble count double against the budget.
func TestAReadiedMoveWalksDifficultTerrainAtItsCost(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	c.terrain.paintDifficult(corridor(7, 8)...)
	e := c.fight(t) // Toren at (6,7), east along row 7
	c.mustReady(t, c.caio, c.readyMoveRequest(t, e))
	e = c.mustEndTurn(t, c.master, e)
	c.mustFire(t, e, "Toren")
	_, err := c.move(t, c.caio, "Toren", 11, 7) // 10 + 10 + 5 + 5 + 5 = 35 ft
	wantBlockedBy(t, "a readied move of 35 ft through two squares of rubble", err, reasonTooFar)
	c.untouched(t, e, "the refusal")
	if _, err := c.move(t, c.caio, "Toren", 10, 7); err != nil { // 10 + 10 + 5 + 5 = 30 ft
		t.Fatalf("the readied move of 30 ft through two squares of rubble: %v", err)
	}
	if got := c.moveRowOf(t, "Toren").UsedDft; got != 0 {
		t.Errorf("the turn's movement after the readied move = %d, want 0", got)
	}
}
