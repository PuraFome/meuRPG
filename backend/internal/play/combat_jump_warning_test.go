package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// jumpOptions is GetMoveOptions asked for a long jump, as u.
func (c *cave) jumpOptions(t *testing.T, u *user, label string, edit ...func(*playv1.GetMoveOptionsRequest)) (*playv1.GetMoveOptionsResponse, error) {
	t.Helper()
	req := &playv1.GetMoveOptionsRequest{
		CampaignId: c.campaignID, EncounterId: c.get(t, c.master).GetId(), CombatantId: c.id(t, label), Jump: playv1.JumpKind_JUMP_KIND_LONG,
	}
	for _, e := range edit {
		e(req)
	}
	res, err := u.combat.GetMoveOptions(t.Context(), connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

// withRunningStart gives Toren the 10 ft he walked just before, as a move on foot would.
func (c *cave) withRunningStart(t *testing.T, label string) {
	t.Helper()
	c.execSQL(t, `UPDATE combatants SET last_move_dft = 100 WHERE id = $1`, c.id(t, label))
}

func squareAt(o *playv1.GetMoveOptionsResponse, col, row int32) *playv1.ReachableSquare {
	for _, r := range o.GetReachable() {
		if r.GetCol() == col && r.GetRow() == row {
			return r
		}
	}
	return nil
}

// TestGetMoveOptionsOfALongJumpWarnsAboutOpportunityAttacks: the preview of a long jump
// lists the squares the jump can land on (within its distance, nothing in the way, paid
// by the movement left) and says, for each, the visible enemies whose reach the jump
// leaves, as a walk does (SRD 5.1: a jump is movement, and leaving a hostile creature's
// reach provokes). It names nobody a player does not see, nothing after Desengajar, and
// agrees with the real move.
func TestGetMoveOptionsOfALongJumpWarnsAboutOpportunityAttacks(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	e := c.oppFight(t)
	c.withRunningStart(t, "Toren")
	gob := c.id(t, "Goblin 1")

	opts, err := c.jumpOptions(t, c.caio, "Toren")
	if err != nil {
		t.Fatalf("GetMoveOptions(jump) error = %v", err)
	}
	if sq := squareAt(opts, 4, 6); sq == nil || len(sq.GetProvokesReactorIds()) != 1 || sq.GetProvokesReactorIds()[0] != gob {
		t.Errorf("(4,6) of the jump = %v, want it listed and provoking Goblin 1", sq)
	}
	if sq := squareAt(opts, 6, 8); sq == nil || len(sq.GetProvokesReactorIds()) != 0 {
		t.Errorf("(6,8), inside Goblin 1's reach, = %v, want it listed and provoking nothing", sq)
	}
	if sq := squareAt(opts, 1, 7); sq != nil {
		t.Errorf("(1,7), five squares away, is on the list of a 16 ft jump: %v", sq)
	}
	if sq := squareAt(opts, 6, 7); sq != nil {
		t.Errorf("the jumper's own square is on the list: %v", sq)
	}

	// Without the running start the jump is half as long.
	c.execSQL(t, `UPDATE combatants SET last_move_dft = 0 WHERE id = $1`, c.id(t, "Toren"))
	if short, err := c.jumpOptions(t, c.caio, "Toren"); err != nil || squareAt(short, 4, 6) != nil || squareAt(short, 5, 7) == nil {
		t.Errorf("a standing jump = %v, %v; want (5,7) and not (4,6)", short, err)
	}
	c.withRunningStart(t, "Toren")

	// A hidden reactor is never named to a player; the master's warning is exact.
	c.hide(t, "Goblin 1")
	if o, err := c.jumpOptions(t, c.caio, "Toren"); err != nil || len(squareAt(o, 4, 6).GetProvokesReactorIds()) != 0 {
		t.Errorf("with the goblin hidden, a player's jump warns %v (%v): a hidden reactor is never named", squareAt(o, 4, 6), err)
	}
	if o, err := c.jumpOptions(t, c.master, "Toren"); err != nil || len(squareAt(o, 4, 6).GetProvokesReactorIds()) != 1 {
		t.Errorf("the master's jump warns %v (%v), want Goblin 1", squareAt(o, 4, 6), err)
	}
	c.execSQL(t, `UPDATE combatants SET hidden = false WHERE id = $1`, gob)

	// A high jump changes no square and provokes nothing: it has no preview.
	_, err = c.jumpOptions(t, c.caio, "Toren", func(r *playv1.GetMoveOptionsRequest) { r.Jump = playv1.JumpKind_JUMP_KIND_HIGH })
	wantCode(t, "GetMoveOptions of a high jump", err, connect.CodeInvalidArgument)

	// The real jump provokes where the preview said it would, and the offer says it was a jump.
	res, err := c.jumpMove(t, "Toren", 4, 6, jumpTo(playv1.JumpKind_JUMP_KIND_LONG))
	if err != nil || !res.GetProvoked() {
		t.Fatalf("the jump to (4,6) = %v, %v; want it to provoke", res, err)
	}
	offers := c.offersOf(t, c.master)
	if len(offers) != 1 || offers[0].GetJump() != playv1.JumpKind_JUMP_KIND_LONG {
		t.Fatalf("the master's offers = %v, want one, made by a long jump", offers)
	}
	for _, en := range c.log(t, c.master, e).GetRounds()[0].GetEntries() {
		if en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_MOVED && en.GetJump() == playv1.JumpKind_JUMP_KIND_LONG && !en.GetJumpRunningStart() {
			t.Errorf("the log line of the jump = %v, want it to say the jump had a running start", en)
		}
	}
}

// TestGetMoveOptionsOfAJumpDoesNotWarnAfterDisengage: a combatant that took Desengajar
// provokes nothing, jumping included.
func TestGetMoveOptionsOfAJumpDoesNotWarnAfterDisengage(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	e := c.oppFight(t)
	if _, err := c.action(t, c.caio, e, "Toren", "standard:disengage"); err != nil {
		t.Fatalf("TakeAction(disengage) error = %v", err)
	}
	c.withRunningStart(t, "Toren") // an action breaks the run: the jump needs a new one
	o, err := c.jumpOptions(t, c.caio, "Toren")
	if err != nil || squareAt(o, 4, 6) == nil || len(squareAt(o, 4, 6).GetProvokesReactorIds()) != 0 {
		t.Errorf("after Desengajar the jump to (4,6) warns %v (%v), want it listed with no warning", squareAt(o, 4, 6), err)
	}
}

// TestAWalkOfferIsNotAJump: an opportunity offer made by a walk is not marked as a jump.
func TestAWalkOfferIsNotAJump(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	c.oppFight(t)
	c.leaveGoblin(t)
	if offers := c.offersOf(t, c.master); len(offers) != 1 || offers[0].GetJump() != playv1.JumpKind_JUMP_KIND_UNSPECIFIED {
		t.Errorf("the offers after a walk = %v, want one that is no jump", offers)
	}
}
