package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	playdb "github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

// The budget of a readied move (SRD 5.1 Ready): the move_ft the readier chose, held to each surface's fresh speed. What the
// readier walked on its own turn and its Dash do not count, and GetMoveOptions reads the same budget while the window is open.

// readiedMoveOptions is GetMoveOptions of Toren for Toren's player.
func (a *armed) readiedMoveOptions(t *testing.T, e *playv1.Encounter) (*playv1.GetMoveOptionsResponse, error) {
	t.Helper()
	res, err := a.caio.combat.GetMoveOptions(t.Context(), connect.NewRequest(&playv1.GetMoveOptionsRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, "Toren"),
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

// A readier that spent all of its own turn's movement before it readied still walks the move it readied, and the turn's
// movement it spent stays as it was.
func TestReadiedMoveIgnoresTheTurnMovementSpent(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.readyMove(t)
	a.execSQL(t, `UPDATE combatants SET speed_ft = 30, movement_used_dft = 300 WHERE label = 'Toren'`)
	a.execSQL(t, `UPDATE readied_actions SET move_ft = 30`)
	a.mustFire(t, e, "Toren")

	opts, err := a.readiedMoveOptions(t, e)
	if err != nil || opts.GetMovementLeftDft() != 300 {
		t.Fatalf("GetMoveOptions in the readied window = %v, %v; want 300 dft left", opts.GetMovementLeftDft(), err)
	}
	if _, err := a.frozenMove(t, a.caio, "Toren", 3, 5); err != nil {
		t.Fatalf("the readied move of 10 ft after a turn's 30 ft: %v", err)
	}
	if got := a.movementUsed(t, "Toren"); got != 300 {
		t.Errorf("the turn's movement spent = %d, want 300", got)
	}
}

// A readier that Dashed does not get the doubled speed: the readied move is the fresh speed, held to move_ft.
func TestReadiedMoveIgnoresTheDash(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.readyMove(t)
	a.execSQL(t, `UPDATE combatants SET speed_ft = 10, dashed = true, movement_used_dft = 0 WHERE label = 'Toren'`)
	a.execSQL(t, `UPDATE readied_actions SET move_ft = 30`)
	a.mustFire(t, e, "Toren")

	opts, err := a.readiedMoveOptions(t, e)
	if err != nil || opts.GetMovementLeftDft() != 100 {
		t.Fatalf("GetMoveOptions in the readied window = %v, %v; want 100 dft left", opts.GetMovementLeftDft(), err)
	}
	_, err = a.frozenMove(t, a.caio, "Toren", 3, 6)
	wantBlockedBy(t, "a readied move of 15 ft at a speed of 10 that Dashed", err, reasonTooFar)
	if _, err := a.frozenMove(t, a.caio, "Toren", 3, 5); err != nil {
		t.Fatalf("the readied move of 10 ft: %v", err)
	}
}

// GetMoveOptions stays the turn's for a player whose readied window is not open.
func TestReadiedMoveOptionsNeedTheOpenWindow(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.readyMove(t)
	_, err := a.readiedMoveOptions(t, e)
	wantBlockedBy(t, "GetMoveOptions of Toren off his turn before the fire", err, blockedNotYourTurn)
}

// movementUsed is the turn's movement a combatant spent, in tenths of a foot.
func (a *armed) movementUsed(t *testing.T, label string) int32 {
	t.Helper()
	var n int32
	if err := a.h.pool.QueryRow(t.Context(), `SELECT movement_used_dft FROM combatants WHERE label = $1`, label).Scan(&n); err != nil {
		t.Fatalf("movement_used_dft of %s: %v", label, err)
	}
	return n
}

// The speeds of a readied move hold each surface to its own fresh speed and to move_ft; move_ft at Ready is the most of the
// fresh speeds, so a swimmer that readies in water may swim its whole swim speed.
func TestReadiedSpeeds(t *testing.T) {
	t.Parallel()
	swimmer := playdb.Combatant{SpeedFt: 10, SpeedSwimFt: 40, MovementUsedDft: 100, Dashed: true}
	if got := readyMoveFt(swimmer); got != 40 {
		t.Errorf("readyMoveFt(walk 10, swim 40) = %d, want 40", got)
	}
	if got := readiedSpeedsOf(swimmer, 40); got != (grid.Speeds{FloorDFt: 100, WaterDFt: 400, ClimbDFt: 100}) {
		t.Errorf("readiedSpeedsOf(walk 10, swim 40; 40) = %+v", got)
	}
	if got := readiedSpeedsOf(swimmer, 20); got != (grid.Speeds{FloorDFt: 100, WaterDFt: 200, ClimbDFt: 100}) {
		t.Errorf("readiedSpeedsOf(walk 10, swim 40; 20) = %+v", got)
	}
	flier := playdb.Combatant{SpeedFt: 30, SpeedFlyFt: 60, MovementUsedDft: 300}
	if got := readyMoveFt(flier); got != 60 {
		t.Errorf("readyMoveFt(walk 30, fly 60) = %d, want 60", got)
	}
	if got := readiedSpeedsOf(flier, 45); got != (grid.Speeds{FloorDFt: 450, WaterDFt: 450, ClimbDFt: 450}) {
		t.Errorf("readiedSpeedsOf(fly 60; 45) = %+v", got)
	}
}
