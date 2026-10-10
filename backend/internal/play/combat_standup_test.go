package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// Standing up from Prone (SRD 5.1, "Being Prone"): half the speed, and not without it.
// Crawling: every foot costs 1 extra foot. The fixture is the arena of combat_contests_test.go:
// Toren (30 ft) acts first at (3,3), the Hobgoblin at (4,3), Pensantus at (3,6).

var (
	blockedNotProne     = playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NOT_PRONE
	blockedCantStandUp  = playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_CANNOT_STAND_UP
	contestHideRevealed = playv1.ContestLogLine_CONTEST_LOG_LINE_HIDE_REVEALED
)

func (a *armed) standUp(t *testing.T, u *user, e *playv1.Encounter, label string) (*playv1.StandUpResponse, error) {
	t.Helper()
	res, err := u.combat.StandUp(t.Context(), connect.NewRequest(&playv1.StandUpRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, label), IdempotencyKey: newKey(),
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

// contestLine is the CONTEST log line of the given kind in the log as u reads it, or nil.
func (a *armed) contestLine(t *testing.T, u *user, e *playv1.Encounter, line playv1.ContestLogLine) *playv1.CombatLogEntry {
	t.Helper()
	for _, r := range a.log(t, u, e).GetRounds() {
		for _, en := range r.GetEntries() {
			if en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_CONTEST && en.GetContest().GetLine() == line {
				return en
			}
		}
	}
	return nil
}

func TestStandUpCostsHalfTheSpeedAndTheMasterCanUndoIt(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	c := a.arena(t, arenaPlan{})
	_, err := a.standUp(t, a.caio, c.refresh(t), "Toren")
	wantBlockedBy(t, "StandUp while not prone", err, blockedNotProne)

	a.setConditions(t, c.refresh(t), "Toren", condProne)
	if got := c.combatant(t, a.caio, "Toren").GetStandUpCostDft(); got != 150 {
		t.Fatalf("stand_up_cost_dft = %d, want 150 (half of 30 ft)", got)
	}
	if got := c.combatant(t, a.ana, "Toren").GetStandUpCostDft(); got != 0 {
		t.Errorf("another player reads a stand up cost of %d, want 0", got)
	}
	res, err := a.standUp(t, a.caio, c.e, "Toren")
	if err != nil {
		t.Fatalf("StandUp() error = %v", err)
	}
	if res.GetMovementLeftDft() != 150 || c.hasCondition(t, "Toren", condProne) {
		t.Errorf("after standing up: left %d, prone %v; want 150 and not prone", res.GetMovementLeftDft(), c.hasCondition(t, "Toren", condProne))
	}
	if got := c.combatant(t, a.caio, "Toren").GetStandUpCostDft(); got != 0 {
		t.Errorf("stand_up_cost_dft = %d after standing, want 0", got)
	}
	// The log: every player who sees Toren reads it, and the master can undo it.
	for who, u := range map[string]*user{"Toren's player": a.caio, "Pensantus's player": a.ana} {
		if en := a.contestLine(t, u, c.e, playv1.ContestLogLine_CONTEST_LOG_LINE_STOOD_UP); en == nil || en.GetActorLabel() != "Toren" {
			t.Errorf("%s reads %v, want Toren's line of standing up", who, en)
		}
	}
	if en := a.contestLine(t, a.master, c.e, playv1.ContestLogLine_CONTEST_LOG_LINE_STOOD_UP); en == nil || !en.GetUndoable() {
		t.Fatalf("the master's line = %v, want it undoable", en)
	}
	a.undoLast(t, c.e)
	if !c.hasCondition(t, "Toren", condProne) || c.combatant(t, a.caio, "Toren").GetMovementLeftDft() != 300 {
		t.Errorf("after the undo: prone %v, left %d; want prone and 300", c.hasCondition(t, "Toren", condProne), c.combatant(t, a.caio, "Toren").GetMovementLeftDft())
	}
}

func TestStandUpIsRefusedWithoutTheMovement(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	c := a.arena(t, arenaPlan{at: map[string][2]int32{"Hobgoblin": {8, 3}}}) // nobody's reach: the crawl offers no opportunity attack
	a.setConditions(t, c.refresh(t), "Toren", condProne)

	// Crawling: two squares (10 ft) cost 20 ft while prone, so 10 ft are left, 5 ft short.
	if _, err := c.moveTo(t, a.caio, "Toren", 3, 5); err != nil {
		t.Fatalf("a crawl of two squares error = %v", err)
	}
	if got := c.combatant(t, a.caio, "Toren"); got.GetMovementUsedDft() != 200 || got.GetMovementLeftDft() != 100 {
		t.Fatalf("after crawling 10 ft: used %d, left %d; want 200 and 100 (every foot costs 2)", got.GetMovementUsedDft(), got.GetMovementLeftDft())
	}
	_, err := a.standUp(t, a.caio, c.e, "Toren")
	b := wantBlockedBy(t, "StandUp with 10 ft left", err, blockedTooFar)
	if b.GetMissingFt() != 5 || b.GetMissingDft() != 50 {
		t.Errorf("TOO_FAR missing = %d ft, %d tenths; want 5 and 50", b.GetMissingFt(), b.GetMissingDft())
	}
	if !c.hasCondition(t, "Toren", condProne) {
		t.Error("a refused stand up took the condition off")
	}
	// The preview of the reach is the crawl's: 10 ft left pay for 5 ft of path.
	for _, r := range c.moveOptions(t, a.caio, "Toren").GetReachable() {
		if r.GetCostDft() > 50 {
			t.Errorf("square (%d,%d) is offered at %d tenths, want none beyond 50 (5 ft of path)", r.GetCol(), r.GetRow(), r.GetCostDft())
		}
	}
	// A move past what crawling allows is refused.
	_, err = c.moveTo(t, a.caio, "Toren", 3, 8)
	wantBlockedBy(t, "a crawl past the movement", err, blockedTooFar)
}

func TestStandUpIsRefusedWithSpeedZeroOrOutOfTurn(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	c := a.arena(t, arenaPlan{})
	a.setConditions(t, c.refresh(t), "Toren", condProne, "condition:grappled")
	_, err := a.standUp(t, a.caio, c.e, "Toren")
	wantBlockedBy(t, "StandUp while grappled", err, blockedCantStandUp)

	a.setConditions(t, c.refresh(t), "Pensantus", condProne)
	_, err = a.standUp(t, a.ana, c.e, "Pensantus")
	wantBlockedBy(t, "StandUp out of turn", err, blockedNotYourTurn)
	_, err = a.standUp(t, a.master, c.e, "Pensantus")
	wantBlockedBy(t, "the master's StandUp out of turn", err, blockedNotYourTurn)
	_, err = a.standUp(t, a.bia, c.e, "Toren")
	wantCode(t, "StandUp for another player's character", err, connect.CodePermissionDenied)

	// The master stands an NPC up on its turn.
	a.setConditions(t, c.refresh(t), "Goblin", condProne)
	c.advance(t, "Goblin")
	res, err := a.standUp(t, a.master, c.refresh(t), "Goblin")
	if err != nil {
		t.Fatalf("the master's StandUp(Goblin) error = %v", err)
	}
	if res.GetMovementLeftDft() != 150 { // the Goblin walks 30 ft
		t.Errorf("the Goblin has %d tenths left, want 150", res.GetMovementLeftDft())
	}
}

func TestStandUpWithoutAMap(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.theatreThree(t) // Toren first
	a.setConditions(t, e, "Toren", condProne)
	if _, err := a.standUp(t, a.caio, e, "Toren"); err != nil {
		t.Fatalf("StandUp in theatre mode error = %v", err)
	}
	got := a.theatreCombatant(t, a.caio, "Toren")
	if got.GetMovementLeftDft() != 150 || a.theatreCombatant(t, a.master, "Toren").GetStandUpCostDft() != 0 {
		t.Errorf("after standing up: left %d; want 150", got.GetMovementLeftDft())
	}
	wantNoSquares(t, "after StandUp", a.get(t, a.master))
}
