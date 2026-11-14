package play

import (
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// countRows counts the rows a query finds.
func (a *armed) countRows(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := a.h.pool.QueryRow(t.Context(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("count %q: %v", sql, err)
	}
	return n
}

// The undo of a fire whose readied attack waits for its roller's answer about a die: the held roll names the READIED
// window (its pinned RollAttack carries catch_window_id), so the undo, which would delete the window, is refused while it
// waits (REACTION_PENDING). Nothing is left naming a window that is gone, and the answer then makes the attack.
func TestTheUndoOfAFireWaitsForTheUseItHolds(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.readyThenPass(t, nil)
	a.giveCombatantDie(t, "Toren", e)
	w := a.mustFire(t, e, "Toren")
	res, err := a.attack(t, a.caio, e, "Toren", battleaxe, "Goblin", func(r *playv1.RollAttackRequest) { d20(10)(r); r.CatchWindowId = w.GetId() })
	if err != nil || res.GetHeldRollOffer() == nil {
		t.Fatalf("the readied attack = %v, %v; want it held for the die", res, err)
	}
	err = a.undo(t, a.master, e, a.lastEventID(t, e))
	wantBlockedBy(t, "the undo of the fire while its attack is held", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_REACTION_PENDING)
	if n := a.countRows(t, `SELECT count(*) FROM reaction_windows WHERE id = $1 AND status = 'open'`, w.GetId()); n != 1 {
		t.Fatalf("the READIED window rows after the refused undo = %d, want it open", n)
	}
	if _, err := a.answer(t, a.caio, e, res.GetHeldRollOffer().GetHoldId(), keepTheDie); err != nil {
		t.Fatalf("the die's answer error = %v", err)
	}
	if toren := byLabel(t, a.get(t, a.caio), "Toren"); !toren.GetReactionUsed() || a.readiedStatus(t, "Toren") != readiedFired {
		t.Errorf("after the answer: reaction %v, rows %q", toren.GetReactionUsed(), a.readiedStatus(t, "Toren"))
	}
}
