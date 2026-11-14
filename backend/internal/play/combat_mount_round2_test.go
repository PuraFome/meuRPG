package play

import (
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// TestMountedCombat_ARidersPlayerMovesNoMountWhosePartEnded: the master ends the Ogro's part of the joint turn while Toren's goes on: the
// rider's player no longer moves the Ogro, and its move options say it is not their turn (the mount's part is its own); the master still
// places it, and the rider's player read its options before the part ended (the positive control).
func TestMountedCombat_ARidersPlayerMovesNoMountWhosePartEnded(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	c.fight(t)
	c.steed(t)
	c.mustMount(t, c.caio, "Toren", "Ogro")
	if _, err := c.options(t, c.caio, "Ogro"); err != nil {
		t.Fatalf("the positive control failed: the options of the mount the rider drives = %v", err)
	}
	e := c.mustEndPart(t, c.master, c.get(t, c.master), "Ogro")
	if !byLabel(t, e, "Ogro").GetTurnPartEnded() || byLabel(t, e, "Toren").GetTurnPartEnded() {
		t.Fatalf("premise: after the master ended the Ogro's part, parts ended = Ogro %v, Toren %v", byLabel(t, e, "Ogro").GetTurnPartEnded(), byLabel(t, e, "Toren").GetTurnPartEnded())
	}
	_, err := c.move(t, c.caio, "Ogro", 8, 7)
	wantBlockedBy(t, "the rider's player moving a mount whose part ended", err, reasonNotYourTurn)
	_, err = c.options(t, c.caio, "Ogro")
	wantBlockedBy(t, "the move options of a mount whose part ended", err, reasonNotYourTurn)
	if got := c.rideRowOf(t, "Ogro"); got.Col != 7 || got.UsedDft != 0 {
		t.Errorf("the Ogro after the refused move = %+v, want where it was and nothing spent", got)
	}
	if _, err := c.move(t, c.master, "Ogro", 8, 7); err != nil {
		t.Errorf("the master placing the mount = %v, want it moved", err)
	}
}

var reasonNotYourTurn = playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NOT_YOUR_TURN

// TestMountedCombat_AMountMarkedControlledMidTurnJoinsTheRidersPart: the Ogro carries Toren as an independent mount and the master marks it
// controlled while Toren's turn runs. It takes the rider's total and joins his part of the turn at once, as a mount the rider just got on
// does: acting, in the joint group, with a fresh economy; or, when its own turn came before this one, with the economy it had spent.
func TestMountedCombat_AMountMarkedControlledMidTurnJoinsTheRidersPart(t *testing.T) {
	t.Parallel()
	t.Run("its own turn still to come", func(t *testing.T) {
		t.Parallel()
		c := newCave(t)
		c.fight(t)
		c.steed(t)
		c.mustMark(t, "Ogro", "independent")
		c.mustMount(t, c.caio, "Toren", "Ogro")
		if got := group(c.get(t, c.master)); got != "Toren" {
			t.Fatalf("premise: the group on turn = %q, want Toren alone with an independent mount", got)
		}
		c.mustMark(t, "Ogro", "controlled")
		if got := group(c.get(t, c.master)); got != "Toren,Ogro" {
			t.Errorf("the group on turn = %q after the mark, want Toren,Ogro", got)
		}
		if ogre := c.joinRowOf(t, "Ogro"); ogre.State != "acting" || ogre.ActionUsed {
			t.Errorf("the Ogro after the mark = %+v, want acting with a fresh economy", ogre)
		}
		if _, err := c.action(t, c.caio, c.get(t, c.caio), "Ogro", "standard:dash"); err != nil {
			t.Errorf("the rider's player making the mount Dash = %v, want it taken", err)
		}
		e := c.mustEndPart(t, c.caio, c.get(t, c.caio), "Toren")
		if current(e) != "Ogro" {
			t.Errorf("after Toren's part the turn is %q's, want the Ogro's part to run", current(e))
		}
	})
	t.Run("its own turn passed", func(t *testing.T) {
		t.Parallel()
		c := newCave(t)
		c.ogreFirstFight(t)
		c.steed(t)
		c.mustMark(t, "Ogro", "independent")
		c.spendTheOgre(t)
		c.passTo(t, c.get(t, c.master), "Toren")
		c.mustMount(t, c.caio, "Toren", "Ogro")
		c.mustMark(t, "Ogro", "controlled")
		if got := group(c.get(t, c.master)); got != "Toren,Ogro" {
			t.Errorf("the group on turn = %q after the mark, want Toren,Ogro", got)
		}
		if ogre := c.joinRowOf(t, "Ogro"); ogre.State != "acting" || !ogre.ActionUsed || !ogre.ReactionUsed {
			t.Errorf("the Ogro after the mark = %+v, want acting with the economy it had spent", ogre)
		}
	})
}
