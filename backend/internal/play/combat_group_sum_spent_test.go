package play

import (
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// What a take-out of a group part puts back (#835): one rule. The costs of the declaration (the smite's slot) are paid by the first
// roll, once: the take-out keeps them spent and the roll of the new sum spends nothing. The extra that comes out gives back its
// once-per-turn use (Sneak Attack, Colossus Slayer). A discard of the damage that waits for the sum gives back nothing either:
// the discard, like the undo of a discard, never touches what the roll spent.

func (h *sumSmiteHit) marked(t *testing.T, feature string) bool {
	t.Helper()
	var raw []byte
	if err := h.a.h.pool.QueryRow(t.Context(), `SELECT feature_state FROM combatants WHERE id = $1`, h.a.id(t, "Toren")).Scan(&raw); err != nil {
		t.Fatalf("read the feature state: %v", err)
	}
	_, ok := readFeatureState(raw)[feature]
	return ok
}

// A damage applied to an NPC: the undo part of the take-out puts the hit points back and keeps the slot spent; the second roll
// lands the damage again with no new slot.
func TestTakingAPartOutOfAnAppliedDamageKeepsTheSmiteSlotSpentAndTheNextRollSpendsNothing(t *testing.T) {
	t.Parallel()
	h := newSumSmiteHit(t, "", true)
	before := h.hp(t)
	if h.slotsUsed(t) != 1 {
		t.Fatalf("slots used = %d after the smite, want 1", h.slotsUsed(t))
	}
	if _, err := h.takeOut(t, "sneak-attack", "x"); err != nil {
		t.Fatalf("RemoveDamagePart() error = %v", err)
	}
	if h.slotsUsed(t) != 1 {
		t.Errorf("slots used = %d after the take-out, want 1", h.slotsUsed(t))
	}
	if h.hp(t) <= before {
		t.Errorf("hit points = %d, want more than %d: the damage was undone", h.hp(t), before)
	}
	res, err := h.a.damage(t, h.a.caio, h.e, h.id, typedDamage(4))
	if err != nil {
		t.Fatalf("RollDamage() error = %v", err)
	}
	if got := res.GetPendingDamage().GetStatus(); got != playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_APPLIED {
		t.Errorf("status = %v, want APPLIED", got)
	}
	if h.slotsUsed(t) != 1 {
		t.Errorf("slots used = %d after the second roll, want 1", h.slotsUsed(t))
	}
}

// The master discards the damage that waits for the sum: the slot stays spent (the discard gives back nothing, and the undo of a
// discard only puts the damage back to waiting).
func TestDiscardingTheDamageThatWaitsForTheSumGivesTheSmiteSlotBackToNobody(t *testing.T) {
	t.Parallel()
	h := newSumSmiteHit(t, "Brisa", true)
	if _, err := h.takeOut(t, "sneak-attack", "x"); err != nil {
		t.Fatalf("RemoveDamagePart() error = %v", err)
	}
	if _, err := h.a.settle(t, h.a.master, h.e, h.id, false); err != nil {
		t.Fatalf("DiscardPendingDamage() error = %v", err)
	}
	if h.slotsUsed(t) != 1 {
		t.Errorf("slots used = %d after the discard, want 1", h.slotsUsed(t))
	}
	if err := h.a.undo(t, h.a.master, h.e, h.a.log(t, h.a.master, h.e).GetUndoableEventId()); err != nil {
		t.Fatalf("UndoLastAction(the discard) error = %v", err)
	}
	if status, _ := h.a.pendingOf(t, h.id); status != pendingAwaitingGroupSum || h.slotsUsed(t) != 1 {
		t.Errorf("status %q, slots used %d after the undo of the discard, want the damage waiting for the sum and the slot spent", status, h.slotsUsed(t))
	}
}

// The extra that came out gives the turn's Sneak Attack back; what stays keeps what it spent.
func TestTakingSneakAttackOutOfAGroupGivesTheTurnsUseBack(t *testing.T) {
	t.Parallel()
	h := newSumSmiteHit(t, "Brisa", true)
	if !h.marked(t, featureSneakAttack) {
		t.Fatal("Sneak Attack is not marked used after the roll that counted it")
	}
	if _, err := h.takeOut(t, "sneak-attack", "x"); err != nil {
		t.Fatalf("RemoveDamagePart() error = %v", err)
	}
	if h.marked(t, featureSneakAttack) {
		t.Error("Sneak Attack is still marked used after the master took it out")
	}
	if _, err := h.a.damage(t, h.a.caio, h.e, h.id, typedDamage(4)); err != nil {
		t.Fatalf("RollDamage() error = %v", err)
	}
	if h.marked(t, featureSneakAttack) || h.slotsUsed(t) != 1 {
		t.Errorf("after the second roll: Sneak Attack marked %v, slots used %d; want it free and the slot spent once", h.marked(t, featureSneakAttack), h.slotsUsed(t))
	}
}

// The undo of the second roll gives back what that roll did, and the second roll spent nothing: the damage that waits again
// must still not spend what the first roll paid, so a roll after the undo leaves the slot spent once.
func TestTheRollAfterTheUndoOfTheSecondRollSpendsNothingAgain(t *testing.T) {
	t.Parallel()
	h := newSumSmiteHit(t, "Brisa", true)
	if _, err := h.takeOut(t, "sneak-attack", "x"); err != nil {
		t.Fatalf("RemoveDamagePart() error = %v", err)
	}
	if _, err := h.a.damage(t, h.a.caio, h.e, h.id, typedDamage(4)); err != nil {
		t.Fatalf("RollDamage(second) error = %v", err)
	}
	if err := h.a.undo(t, h.a.master, h.e, h.a.log(t, h.a.master, h.e).GetUndoableEventId()); err != nil {
		t.Fatalf("UndoLastAction(the second roll) error = %v", err)
	}
	if h.slotsUsed(t) != 1 {
		t.Fatalf("slots used = %d after the undo of the second roll, want 1", h.slotsUsed(t))
	}
	p := h.pending(t)
	if p.GetStatus() != playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_AWAITING_GROUP_SUM {
		t.Errorf("status = %v after the undo of the second roll, want the damage waiting for the sum again", p.GetStatus())
	}
	if p.GetAwaitedDamageTypeKey() != groupPiercing {
		t.Errorf("awaited = %q after the undo, want %q", p.GetAwaitedDamageTypeKey(), groupPiercing)
	}
	third, err := h.a.damage(t, h.a.caio, h.e, h.id, typedDamage(6))
	if err != nil {
		t.Fatalf("RollDamage(third) error = %v", err)
	}
	if smite := partRollOf(third.GetPendingDamage().GetPartRolls(), "divine-smite"); smite == nil || smite.GetSum() != 5 {
		t.Errorf("the smite = %v after the roll that follows the undo, want the 5 the first roll left", smite)
	}
	if h.slotsUsed(t) != 1 {
		t.Errorf("slots used = %d after the roll that follows the undo, want 1: the first roll paid for the smite", h.slotsUsed(t))
	}
}
