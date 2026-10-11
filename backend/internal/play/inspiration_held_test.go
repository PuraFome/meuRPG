package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// giveCombatantDie puts a d8 on a combatant, as a bard's Bardic Inspiration would, for a few rounds.
func (a *armed) giveCombatantDie(t *testing.T, label string, e *playv1.Encounter) {
	t.Helper()
	if _, err := a.h.pool.Exec(t.Context(),
		`UPDATE combatants SET inspiration_sides = 8, inspiration_expires_round = $2 WHERE id = $1`, a.id(t, label), e.GetRound()+50); err != nil {
		t.Fatalf("give %s a die: %v", label, err)
	}
}

// The concentration save of a character that holds a die is held: the d20 is kept, the window
// stays open and nothing is written until the player answers; using the die adds its face before
// the result is told, and the die is gone.
func TestBardicInspirationHeldOnAConcentrationSave(t *testing.T) {
	t.Parallel()
	a, e := concentratingHit(t, 6)
	a.giveCombatantDie(t, "Pensantus", e)
	w := a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_CONCENTRATION_SAVE)
	if w == nil {
		t.Fatal("Pensantus has no concentration window")
	}
	res, err := a.resolveConcentration(t, a.ana, e, w.GetId(), concentrationFace(3)) // 3 + 1 = 4 against 10: fails without the die
	if err != nil {
		t.Fatalf("ResolveConcentrationSave() error = %v", err)
	}
	offer := res.GetInspirationOffer()
	if offer == nil || res.GetResult() != nil || offer.GetSides() != 8 {
		t.Fatalf("answer = result %v, offer %v; want the question about a d8 and no result", res.GetResult(), offer)
	}
	if a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_CONCENTRATION_SAVE) == nil {
		t.Error("the window closed before the player answered")
	}
	// Another roll of the same character waits for the answer.
	_, err = a.resolveConcentration(t, a.ana, e, w.GetId(), concentrationFace(20))
	wantResourceBlocked(t, err, playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_INSPIRATION_PENDING)
	// Only the roller's player reads the question; another player cannot answer it.
	if got := a.outsideState(t, a.caio).GetOffers(); len(got) != 0 {
		t.Errorf("another player reads %d questions, want 0", len(got))
	}
	if got := a.outsideState(t, a.ana).GetOffers(); len(got) != 1 || got[0].GetHoldId() != offer.GetHoldId() {
		t.Errorf("the roller reads %v, want the question %s", got, offer.GetHoldId())
	}
	if _, err := a.answerOutside(t, a.caio, offer.GetHoldId(), false, false, 0); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("another player's answer = %v, want not_found", err)
	}
	// 3 + 1 + 8 = 12 reaches the DC 10: the concentration is kept, and the die is spent.
	out, err := a.answerOutside(t, a.ana, offer.GetHoldId(), true, false, 8)
	if err != nil {
		t.Fatalf("AnswerOutsideInspiration() error = %v", err)
	}
	r := out.GetConcentrationSave().GetResult()
	if !r.GetKept() || r.GetSave().GetTotal() != 12 || len(r.GetBonusDice()) != 1 || r.GetBonusDice()[0].GetFace() != 8 {
		t.Errorf("the result = %v, want a kept save of 12 with the d8 of 8", r)
	}
	if byLabel(t, a.get(t, a.ana), "Pensantus").GetInspirationDie() != nil {
		t.Error("Pensantus still holds the die after using it")
	}
}

// Keeping the die leaves it on the combatant, and the save is settled with the d20 alone.
func TestBardicInspirationKeptOnAConcentrationSaveKeepsTheDie(t *testing.T) {
	t.Parallel()
	a, e := concentratingHit(t, 6)
	a.giveCombatantDie(t, "Pensantus", e)
	w := a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_CONCENTRATION_SAVE)
	res, err := a.resolveConcentration(t, a.ana, e, w.GetId(), concentrationFace(3))
	if err != nil || res.GetInspirationOffer() == nil {
		t.Fatalf("ResolveConcentrationSave() = %v, %v; want the question", res, err)
	}
	out, err := a.answerOutside(t, a.ana, res.GetInspirationOffer().GetHoldId(), false, false, 0)
	if err != nil {
		t.Fatalf("AnswerOutsideInspiration() error = %v", err)
	}
	if r := out.GetConcentrationSave().GetResult(); r.GetKept() || r.GetSave().GetTotal() != 4 || len(r.GetBonusDice()) != 0 {
		t.Errorf("the result = %v, want the failed save of 4 with no die", r)
	}
	if byLabel(t, a.get(t, a.ana), "Pensantus").GetInspirationDie() == nil {
		t.Error("the kept die is gone")
	}
}

// A character without a die gets no question: the save is settled at once.
func TestNoQuestionOnAConcentrationSaveWithoutADie(t *testing.T) {
	t.Parallel()
	a, e := concentratingHit(t, 6)
	w := a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_CONCENTRATION_SAVE)
	res, err := a.resolveConcentration(t, a.ana, e, w.GetId(), concentrationFace(15))
	if err != nil || res.GetInspirationOffer() != nil || !res.GetResult().GetKept() {
		t.Fatalf("ResolveConcentrationSave() = %v, %v; want the result and no question", res, err)
	}
}

// The master's own roll for a character never asks, even if the character holds a die.
func TestTheMastersConcentrationRollNeverAsksForTheDie(t *testing.T) {
	t.Parallel()
	a, e := concentratingHit(t, 6)
	a.giveCombatantDie(t, "Pensantus", e)
	w := a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_CONCENTRATION_SAVE)
	res, err := a.resolveConcentration(t, a.master, e, w.GetId(), concentrationFace(15))
	if err != nil || res.GetInspirationOffer() != nil || res.GetResult() == nil {
		t.Fatalf("ResolveConcentrationSave(master) = %v, %v; want the result and no question", res, err)
	}
}

// A hold of a combat round that is over, or whose combatant no longer holds the die, is stale.
func TestStaleHoldOfACombat(t *testing.T) {
	t.Parallel()
	round := int32(3)
	combatant := playdb.Combatant{}
	held := playdb.InspirationHold{Round: &round}
	out := playdb.InspirationHold{}
	cases := []struct {
		name  string
		hold  playdb.InspirationHold
		die   heldDie
		found bool
		want  bool
	}{
		{"same round", held, heldDie{Combatant: &combatant, Round: 3}, true, false},
		{"later round", held, heldDie{Combatant: &combatant, Round: 4}, true, true},
		{"die gone", held, heldDie{}, false, true},
		{"combat over", held, heldDie{Sides: 8}, true, true},
		{"out of a combat", out, heldDie{Sides: 8}, true, false},
	}
	for _, tc := range cases {
		if got := staleHold(tc.hold, tc.die, tc.found); got != tc.want {
			t.Errorf("%s: staleHold = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// A leveled spell attack against one target by a character that holds a die: the d20 is rolled
// and kept before the slot or the action is spent, the cast answers the question, and the answer
// casts it with that d20 and the die added (the hit or miss is decided after).
func TestBardicInspirationHeldOnASpellAttack(t *testing.T) {
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
	res, err := a.cast(t, a.bia, e, "Brisa", guidingBolt, slotOfLevel(1), a.at(t, "Goblin"), typed)
	if err != nil {
		t.Fatalf("CastSpell() error = %v", err)
	}
	offer := res.GetInspirationOffer()
	if offer == nil || res.GetCast() != nil || offer.GetSides() != 8 {
		t.Fatalf("cast = %v, offer %v; want the question about a d8 and no cast", res.GetCast(), offer)
	}
	if byLabel(t, a.get(t, a.bia), "Brisa").GetActionUsed() {
		t.Error("the action was spent before the player answered")
	}
	out, err := a.answerOutside(t, a.bia, offer.GetHoldId(), true, false, 8)
	if err != nil {
		t.Fatalf("AnswerOutsideInspiration() error = %v", err)
	}
	cast := out.GetSpellCast().GetCast()
	if len(cast.GetTargets()) != 1 || len(cast.GetTargets()[0].GetBonusDice()) != 1 || cast.GetTargets()[0].GetBonusDice()[0].GetFace() != 8 {
		t.Fatalf("the cast = %v, want the d8 of 8 on the spell attack", cast)
	}
	if roll := cast.GetTargets()[0].GetAttackRoll(); roll.GetFaces()[0] != 4 || roll.GetTotal() < 12 {
		t.Errorf("attack roll = %v, want the held d20 of 4 with the die in the total", roll)
	}
	c := byLabel(t, a.get(t, a.bia), "Brisa")
	if !c.GetActionUsed() || c.GetInspirationDie() != nil {
		t.Errorf("after the answer: action used %v, die %v; want the action spent and the die gone", c.GetActionUsed(), c.GetInspirationDie())
	}
}
