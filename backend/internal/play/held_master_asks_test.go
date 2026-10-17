package play

import (
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The master's own roll for a player's character asks like the player's own (MR-014, decided by Vinicius on
// 16/10/2026, E4-10): the master is asked in his own sheet, the player reads the same question, the master's answer
// waits for the player's first 10 s (PLAYER_FIRST) and the first answer wins. The concentration save and the group
// check are in inspiration_held_test.go and inspiration_lucky_test.go; these are the spell attack and the effect's
// save. A combatant no player plays keeps settling at once (saveSpec, spellMayAsk, mayAskMember).

// TestTheMastersSpellAttackForAPlayerAsksForTheDie: the master casts Guiding Bolt for Brisa, who holds a die; the
// question comes back to the master and to Brisa's player, the cast is not written, and Brisa's answer casts it.
func TestTheMastersSpellAttackForAPlayerAsksForTheDie(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: a.goblin.GetId()}},
		npcRolls: []int{3},
		players:  map[string]int32{"Toren": 18, "Pensantus": 10, "Brisa": 1},
		reveal:   []string{"Goblin"},
		at:       map[string][2]int32{"Toren": {3, 3}, "Goblin": {4, 3}, "Pensantus": {10, 3}, "Brisa": {4, 9}},
	})
	a.passTo(t, e, "Brisa")
	e = a.get(t, a.master)
	a.giveCombatantDie(t, "Brisa", e)
	typed := func(r *playv1.CastSpellRequest) { r.Roll = &playv1.CastSpellRequest_D20Face{D20Face: 4} }
	res, err := a.cast(t, a.master, e, "Brisa", guidingBolt, slotOfLevel(1), a.at(t, "Goblin"), typed)
	if err != nil || res.GetCast() != nil || res.GetInspirationOffer() == nil || res.GetHeldRollOffer() == nil {
		t.Fatalf("CastSpell(master) = %v, %v; want the die's question and no cast", res, err)
	}
	hold := res.GetInspirationOffer().GetHoldId()
	if got := byLabel(t, a.get(t, a.bia), "Brisa").GetHeldRollOffer(); got.GetHoldId() != hold {
		t.Errorf("the player reads %v, want the master's question about her cast", got)
	}
	if byLabel(t, a.get(t, a.master), "Brisa").GetActionUsed() {
		t.Error("the action was spent before anybody answered")
	}
	_, err = a.answerHeldRoll(t, a.master, hold, dieAnswer(false))
	wantResourceBlocked(t, err, playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_PLAYER_FIRST)
	out, err := a.answerOutside(t, a.bia, hold, true, false, 8)
	if err != nil {
		t.Fatalf("the player's answer: error = %v", err)
	}
	cast := out.GetSpellCast().GetCast()
	if len(cast.GetTargets()) != 1 || len(cast.GetTargets()[0].GetBonusDice()) != 1 || cast.GetTargets()[0].GetAttackRoll().GetFaces()[0] != 4 {
		t.Errorf("the cast = %v, want the master's d20 of 4 with the player's d8", cast)
	}
	if !byLabel(t, a.get(t, a.master), "Brisa").GetActionUsed() {
		t.Error("the action is not spent after the answer")
	}
}

// TestTheMastersEffectSaveForAHalflingAsksLucky: the master rolls the end-of-turn save of Hold Person for Toren, a
// halfling; the 1 is held for Lucky, Toren's player reads the question and answers it first.
func TestTheMastersEffectSaveForAHalflingAsksLucky(t *testing.T) {
	t.Parallel()
	a := newCastersOf(t, 3, "race:halfling")
	e := a.closeFight(t)
	a.mustAddEffect(t, e, holdPerson, []string{"Toren"}, a.rounds(t, 10, "Toren"))
	a.passTo(t, e, "Toren")
	if _, err := a.endTurnRaw(t, a.master, e, true); err != nil {
		t.Fatalf("EndTurn() error = %v", err)
	}
	w := a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_EFFECT_SAVE)
	if w == nil {
		t.Fatalf("the master's windows = %v, want the save of Hold Person", a.windows(t, a.master))
	}
	res, err := a.rollEffectSave(t, a.master, e, w.GetId(), effectSaveFace(1))
	if err != nil || res.GetResult() != nil || res.GetLuckyOffer() == nil {
		t.Fatalf("RollEffectSave(master) = %v, %v; want Lucky's question alone", res, err)
	}
	hold := res.GetLuckyOffer().GetHoldId()
	if got := byLabel(t, a.get(t, a.caio), "Toren").GetHeldRollOffer(); got.GetHoldId() != hold {
		t.Errorf("the player reads %v, want the master's question about his save", got)
	}
	_, err = a.answerHeldRoll(t, a.master, hold, luckyAnswer(17))
	wantResourceBlocked(t, err, playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_PLAYER_FIRST)
	if _, err := a.answerHeldRoll(t, a.caio, hold, luckyAnswer(17)); err != nil {
		t.Fatalf("the player's answer: error = %v", err)
	}
	a.h.afterThePlayersChance()
	if _, err := a.answerHeldRoll(t, a.master, hold, luckyAnswer(3)); err == nil {
		t.Error("the master's answer after the player's = nil, want the roll answered already")
	}
}
