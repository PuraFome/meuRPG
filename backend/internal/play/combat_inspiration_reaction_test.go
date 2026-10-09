package play

import (
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// A held roll for Bardic Inspiration and a Cutting Words window on the same attack do not
// race: the window comes first (nothing is rolled until it is answered), the die is asked
// about after, with the d20 already minus the bard's die, and a second roll waits for the
// answer instead of opening another window.
func TestABardicInspirationQuestionComesAfterTheCuttingWordsWindowOfTheSameAttack(t *testing.T) {
	t.Parallel()
	a, e := reactorFight(t, loreBard)
	a.execSQL(t, `UPDATE combatants SET inspiration_sides = 8, inspiration_from = $2, inspiration_expires_round = 50 WHERE id = $1`, a.id(t, "Goblin"), a.id(t, "Brisa"))
	held := a.mustAttack(t, a.master, e, "Goblin", sword, "Toren", d20(8)) // 8 + 4 = 12 against Toren's 11
	if !held.GetRoll().GetHeldForReaction() || held.GetInspirationOffer() != nil {
		t.Fatalf("the attack = %v, want it held for the bard's window, with no question yet", held)
	}
	w := a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_CUTTING_WORDS)
	if w == nil {
		t.Fatal("the bard has no window")
	}
	// A second roll while the first waits for the window is not rolled either.
	res := a.mustAnswer(t, a.ana, e, w.GetId(), func(r *playv1.AnswerReactionRequest) {
		r.Answer, r.Roll = playv1.ReactionChoice_REACTION_CHOICE_USE, &playv1.AnswerReactionRequest_Typed{Typed: 3}
	})
	if n := len(res.GetEncounter().GetReactionWindows()); n != 0 {
		t.Fatalf("windows left: %d", n)
	}
	goblin := byLabel(t, a.get(t, a.master), "Goblin")
	offer := goblin.GetInspirationOffer()
	if offer == nil || offer.GetD20().GetTotal() != 9 { // 8 + 4 - the bard's die 3
		t.Fatalf("the goblin's question = %v, want the d20 8 with 4 - 3 on top (9)", offer)
	}
	// Another attack waits for the answer: no new window, no new roll.
	_, err := a.attack(t, a.master, e, "Goblin", sword, "Toren", d20(19))
	wantResourceBlocked(t, err, playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_INSPIRATION_PENDING)
	if w := a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_CUTTING_WORDS); w != nil {
		t.Errorf("a second window opened while the question waited: %v", w)
	}
	// Using the die: the d20 stays 8, the bonus is 4 - 3 + the d8, and no window asks again.
	a.h.roller.queue(8)
	out, err := a.answer(t, a.master, e, offer.GetHoldId(), useDieInApp)
	if err != nil {
		t.Fatalf("AnswerBardicInspiration() error = %v", err)
	}
	r := out.GetAttack().GetRoll()
	if r.GetD20().GetTotal() != 17 || r.GetOutcome() != playv1.AttackOutcome_ATTACK_OUTCOME_HIT {
		t.Errorf("the attack = %v, want the d20 8 with 4 - 3 + 8 = 17 against 11: a hit", r)
	}
	if w := a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_CUTTING_WORDS); w != nil {
		t.Errorf("the answer opened the window again: %v", w)
	}
}
