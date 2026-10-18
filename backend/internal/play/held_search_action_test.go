package play

import (
	"testing"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The Search action of a combat is an ability check (SRD 5.1): a Bardic Inspiration die the combatant holds may be added
// to it. The d20 is kept before the action is spent or anything is written, and the answer spends the action and searches
// with the die added.

func TestTheSearchActionOfACombatWithADieIsHeldWithTheActionUnspent(t *testing.T) {
	t.Parallel()
	r := newTrapRig(t)
	pit := r.trap(t, "Fosso Dourado", 7, 7, func(s *mapsv1.TrapSpec) { s.NoticeDc = 20 })
	e := r.fight(t)
	r.giveCombatantDie(t, "Toren", e)

	held, err := r.trapSearchCall(t, r.caio, trapSearchTyped(perception, 12)) // Perception +4: 16, a 20 needs the die
	if err != nil || held.GetRoll() != nil || held.GetHeldRollOffer().GetDie() == nil {
		t.Fatalf("SearchForTraps() = %v, %v; want the die's question alone", held, err)
	}
	if offer := held.GetHeldRollOffer(); offer.GetKind() != playv1.HeldRollKind_HELD_ROLL_KIND_TRAP_SEARCH {
		t.Errorf("the offer kind = %v, want the trap search's", offer.GetKind())
	}
	if byLabel(t, r.get(t, r.master), "Toren").GetActionUsed() {
		t.Error("the action is spent while the d20 waits")
	}
	if n := r.eventCount(t, "trap_searched"); n != 0 {
		t.Errorf("trap_searched events = %d while the d20 waits, want none", n)
	}

	r.h.roller.queue(4) // the die: 12 + 4 + 4 = 20 finds the pit
	out, err := r.answerHeldRoll(t, r.caio, held.GetHeldRollOffer().GetHoldId(), dieAnswer(true))
	if err != nil {
		t.Fatalf("AnswerHeldRoll() error = %v", err)
	}
	res := out.GetTrapSearch()
	if !res.GetSpentAction() || len(res.GetFoundPointIds()) != 1 || res.GetFoundPointIds()[0] != pit.GetId() {
		t.Errorf("the search = %v, want the pit found and the action spent", res)
	}
	if !byLabel(t, r.get(t, r.master), "Toren").GetActionUsed() {
		t.Error("the action is not spent after the answer")
	}
}
