package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The master's screen offers "Tirar" on a damage already applied to an NPC (#835), so GetTurnOptions of the attacker lists, for the
// master alone, the damage a take-out could still undo: the attacker's applied damage on an NPC or a creature whose roll is still
// the last action of the session (the condition of takeOutOfGroup). The status APPLIED tells it from one that waits.

func (g *groupHit) listedFor(t *testing.T, u *user) *playv1.PendingDamage {
	t.Helper()
	opts, err := g.a.options(t, u, g.e, "Toren")
	if err != nil {
		t.Fatalf("GetTurnOptions(Toren) error = %v", err)
	}
	for _, p := range opts.GetPendingDamages() {
		if p.GetId() == g.id {
			return p
		}
	}
	return nil
}

func TestTheMasterListsTheAppliedDamageATakeOutCouldStillUndo(t *testing.T) {
	t.Parallel()
	g := newGroupHit(t, 12)
	p := g.listedFor(t, g.a.master)
	if p == nil || p.GetStatus() != playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_APPLIED {
		t.Fatalf("the master reads %v, want the applied damage listed with the status APPLIED", p)
	}
	if sneak := damagePartOf(p, "sneak-attack"); sneak == nil || !sneak.GetSelected() {
		t.Errorf("Sneak Attack = %v in the listed damage, want it listed and selected", sneak)
	}
	if got := g.listedFor(t, g.a.caio); got != nil {
		t.Errorf("the attacker's player reads %v, want only the damage that is open (RN-10)", got)
	}
	_, err := g.a.options(t, g.a.ana, g.e, "Toren")
	wantCode(t, "GetTurnOptions(Toren) as another player", err, connect.CodePermissionDenied)
}

func TestTheAppliedDamageIsNoLongerListedAfterAnotherAction(t *testing.T) {
	t.Parallel()
	g := newGroupHit(t, 12)
	if g.listedFor(t, g.a.master) == nil {
		t.Fatal("the applied damage is not listed to start with")
	}
	if _, err := g.a.adjustHP(t, g.e, g.shrub, damageHP(1)); err != nil {
		t.Fatalf("AdjustCombatantHitPoints() error = %v", err)
	}
	if got := g.listedFor(t, g.a.master); got != nil {
		t.Errorf("the master reads %v after another action, want the damage gone from the list", got)
	}
}

// The listing and the refusal agree: a damage the take-out would refuse is not listed (a player's character was given it already).
func TestTheAppliedDamageOnACharacterIsNotListed(t *testing.T) {
	t.Parallel()
	h := newSumSmiteHit(t, "Brisa", false)
	if _, err := h.a.settle(t, h.a.master, h.e, h.id, true); err != nil {
		t.Fatalf("ApplyPendingDamage() error = %v", err)
	}
	if got := h.listedFor(t, h.a.master); got != nil {
		t.Errorf("the master reads %v, want a damage applied to a character not listed", got)
	}
}
