package play

import (
	"slices"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// A mount that stops sharing its rider's total in the middle of the rider's turn (the rider removed or dismissed, the mount marked
// independent) has its own place back and leaves the turn that runs: the turn then passes to the group after the rider's.

// squireOnTheOgre is the cave with the Escudeiro (NPC rider) on the controlled Ogro, in the Escudeiro's own turn.
func (c *cave) squireOnTheOgre(t *testing.T) {
	t.Helper()
	c.rolledFight(t, []int{2, 2, 2, 2, 11, 15}, map[string]int32{"Toren": 18, "Pensantus": 10, "Brisa": 1})
	c.steed(t)
	c.mustMove(t, c.master, "Escudeiro", 6, 8)
	c.passTo(t, c.get(t, c.master), "Escudeiro")
	c.mustMount(t, c.master, "Escudeiro", "Ogro")
}

// leavesTheTurn checks the Ogro is out of the turn and the group on turn is the first one after the Escudeiro's place that is not the Ogro's own
// place in the order: the turn passed on as if the Ogro had never been in it.
func (c *cave) leavesTheTurn(t *testing.T, before []string, what string) {
	t.Helper()
	e := c.get(t, c.master)
	if st := c.joinRowOf(t, "Ogro").State; st != "idle" {
		t.Errorf("%s: the Ogro's turn_state = %q, want idle: its own place is not in the Escudeiro's group", what, st)
	}
	order := labels(e)
	at := slices.Index(before, "Escudeiro")
	want := order[at%len(order)]
	if at > 0 && order[at-1] == "Ogro" {
		want = order[at]
	}
	if cur := current(e); cur != want || group(e) != want {
		t.Errorf("%s: the turn is %q's (group %q), want %q's: the group after the Escudeiro's in %v", what, cur, group(e), want, order)
	}
}

func TestMountedCombat_ARiderThatLeavesMidTurnTakesItsMountOutOfTheTurn(t *testing.T) {
	t.Parallel()
	t.Run("removed", func(t *testing.T) {
		t.Parallel()
		c := newCave(t)
		c.squireOnTheOgre(t)
		before := c.orderNow(t)
		if _, err := c.master.combat.RemoveCombatant(t.Context(), connect.NewRequest(&playv1.RemoveCombatantRequest{
			CampaignId: c.campaignID, EncounterId: c.get(t, c.master).GetId(), CombatantId: c.id(t, "Escudeiro"), IdempotencyKey: newKey(),
		})); err != nil {
			t.Fatalf("RemoveCombatant(Escudeiro) error = %v", err)
		}
		c.leavesTheTurn(t, before, "the rider removed")
	})
	t.Run("marked independent", func(t *testing.T) {
		t.Parallel()
		c := newCave(t)
		c.squireOnTheOgre(t)
		c.mustMark(t, "Ogro", "independent")
		e := c.get(t, c.master)
		if st := c.joinRowOf(t, "Ogro").State; st != "idle" {
			t.Errorf("the Ogro marked independent: turn_state = %q, want idle", st)
		}
		if group(e) != "Escudeiro" {
			t.Errorf("group = %q, want the Escudeiro alone", group(e))
		}
		if e = c.mustEndPart(t, c.master, e, "Escudeiro"); group(e) == "Ogro" || current(e) == "Ogro" {
			t.Errorf("after the Escudeiro's part the turn is %q's, want the group after the Escudeiro's, not the Ogro's", current(e))
		}
	})
}
