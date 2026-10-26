package play

import (
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The economy of a creature comes back at the start of its own turn, joint or not, also when
// a member of the joint turn died in between (MR-013). These tests need the database.

// toTheGoblinsOfRound1 passes the parts of the turns before the goblins' joint turn.
func toTheGoblinsOfRound1(t *testing.T, a *armed, e *playv1.Encounter) *playv1.Encounter {
	t.Helper()
	a.mustEndPart(t, a.bia, e, "Brisa")
	a.mustEndPart(t, a.caio, e, "Toren")
	a.mustEndPart(t, a.master, e, "Capitão Goblin")
	return a.mustEndPart(t, a.ana, e, "Pensantus")
}

func TestJointTurnLeftoversDoNotFollowTheSurvivorIntoTheNextRound(t *testing.T) {
	t.Parallel()
	a, e := jointFight(t, false)
	e = toTheGoblinsOfRound1(t, a, e)
	if group(e) != "Goblin 1,Goblin 2" {
		t.Fatalf("group = %q, want both goblins", group(e))
	}
	for _, g := range []string{"Goblin 1", "Goblin 2"} {
		a.h.roller.queue(2)
		a.mustAttack(t, a.master, e, g, sword, "Toren", inAppRoll) // each spends its action
	}
	a.mustEndPart(t, a.master, e, "Goblin 2")
	a.mustEndPart(t, a.master, e, "Goblin 1")
	// Round 2: Brisa and Toren, Capitão, Pensantus, then the survivor alone.
	e = a.get(t, a.master)
	for _, step := range []struct {
		u     *user
		label string
	}{{a.bia, "Brisa"}, {a.caio, "Toren"}, {a.master, "Capitão Goblin"}, {a.ana, "Pensantus"}} {
		e = a.mustEndPart(t, step.u, e, step.label)
		if step.label == "Capitão Goblin" { // one of the pair dies in the middle of round 2
			if _, err := a.adjustHP(t, e, "Goblin 1", damageHP(1000)); err != nil {
				t.Fatalf("kill Goblin 1: %v", err)
			}
		}
	}
	if e.GetRound() != 2 || group(e) != "Goblin 2" {
		t.Fatalf("round %d group %q, want round 2 and Goblin 2 alone", e.GetRound(), group(e))
	}
	g := byLabel(t, a.get(t, a.master), "Goblin 2")
	if g.GetActionUsed() || g.GetReactionUsed() || g.GetBonusActionUsed() {
		t.Errorf("Goblin 2 starts its turn with action %v, bonus %v, reaction %v used; want all back", g.GetActionUsed(), g.GetBonusActionUsed(), g.GetReactionUsed())
	}
}
