package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

const couatlShield = "innate_3_day_shield"

// innateUse answers a window with one use of an innate spell's resource, and no slot.
func innateUse(resource string) func(*playv1.AnswerReactionRequest) {
	return func(r *playv1.AnswerReactionRequest) {
		r.Answer = playv1.ReactionChoice_REACTION_CHOICE_USE
		r.Slot = &playv1.SpellSlot{Level: 1, Resource: resource}
	}
}

// couatlHit has Toren hit the Couatl (armor class 19) with a 14 and a +6, 20 in all.
func couatlHit(t *testing.T) (*armed, *playv1.Encounter, *playv1.RollAttackResponse) {
	t.Helper()
	a, e := monsterFight(t, "monster:couatl", "Couatl")
	hit := a.mustAttack(t, a.caio, e, "Toren", battleaxe, "Couatl", d20(14))
	return a, e, hit
}

// TestTheCouatlsInnateShieldOpensTheWindowAndStopsTheHit: the Couatl has Shield 3/day as an innate
// spell and no slot. A hit opens the window, the master's prompt offers the use with the uses left,
// the answer counts one use of the resource on the combatant and the hit is stopped; the undo
// gives the use back and the window waits again. Needs the database.
func TestTheCouatlsInnateShieldOpensTheWindowAndStopsTheHit(t *testing.T) {
	t.Parallel()
	a, e, hit := couatlHit(t)
	if hit.GetPendingDamage().GetStatus() != playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_AWAITING_REACTION {
		t.Fatalf("the hit on the Couatl = %v, want it to wait for the reaction", hit.GetPendingDamage())
	}
	w := a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_SHIELD)
	if w == nil || w.GetReactorLabel() != "Couatl" {
		t.Fatalf("the master's window = %v, want the Couatl's Shield", w)
	}
	slots := w.GetShield().GetSlots()
	if len(slots) != 1 || slots[0].GetResource() != couatlShield || slots[0].GetLevel() != 1 || slots[0].GetFree() != 3 || slots[0].GetTotal() != 3 {
		t.Fatalf("the Couatl's Shield choices = %v, want the innate use, 3 of 3 left", slots)
	}
	if old := a.get(t, a.master).GetReactionPrompts(); len(old) != 1 || len(old[0].GetSlots()) != 1 || old[0].GetSlots()[0].GetResource() != couatlShield {
		t.Errorf("the reaction prompts = %v, want the innate use too", old)
	}

	// A slot is not what the Couatl casts it with.
	_, err := a.answerReaction(t, a.master, e, w.GetId(), useAnswer(1))
	wantCode(t, "Shield with a slot the Couatl does not have", err, connect.CodeInvalidArgument)

	res := a.mustAnswer(t, a.master, e, w.GetId(), innateUse(couatlShield))
	if !res.GetResult().GetShield().GetStopped() || !res.GetResult().GetUsed() {
		t.Errorf("the answer's result = %v, want Shield used and the hit stopped", res.GetResult())
	}
	if used := a.usedOf(t, "Couatl"); used != `{"res:innate_3_day_shield": 1}` {
		t.Errorf("the Couatl's count after Shield = %s, want one use of the resource", used)
	}

	a.undoLast(t, a.get(t, a.master))
	if used := a.usedOf(t, "Couatl"); used != `{"res:innate_3_day_shield": 0}` {
		t.Errorf("the Couatl's count after the undo = %s, want the use back", used)
	}
	back := a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_SHIELD)
	if back == nil || len(back.GetShield().GetSlots()) != 1 || back.GetShield().GetSlots()[0].GetFree() != 3 {
		t.Errorf("the window after the undo = %v, want it open with 3 uses", back)
	}
}

// TestTheCouatlsShieldDoesNotOpenWithTheUsesSpent: with the 3 uses of the day counted, the hit
// goes on to its damage with no window. Needs the database.
func TestTheCouatlsShieldDoesNotOpenWithTheUsesSpent(t *testing.T) {
	t.Parallel()
	a, e := monsterFight(t, "monster:couatl", "Couatl")
	if _, err := a.h.pool.Exec(t.Context(), `UPDATE combatants SET slots_used = $2::JSONB WHERE id = $1`, a.id(t, "Couatl"), `{"res:innate_3_day_shield": 3}`); err != nil {
		t.Fatalf("count the uses: %v", err)
	}
	hit := a.mustAttack(t, a.caio, e, "Toren", battleaxe, "Couatl", d20(14))
	if hit.GetPendingDamage().GetStatus() != playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_AWAITING_ROLL {
		t.Errorf("the hit on the Couatl = %v, want damage to roll: no use of Shield is left", hit.GetPendingDamage())
	}
	if w := a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_SHIELD); w != nil {
		t.Errorf("the master's window = %v, want none", w)
	}
}
