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
