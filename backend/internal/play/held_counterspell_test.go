package play

import (
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The check of a Counterspell above the slot is an ability check with the spellcasting ability (SRD 5.1): a Bardic
// Inspiration die the reactor holds may be added to it. The d20 is kept with the window open and nothing spent, and the
// answer settles the check with the die added.

func TestACounterspellCheckWithADieIsHeldAndTheDieCountersTheSpell(t *testing.T) {
	t.Parallel()
	a, e, w := upcastWindow(t)
	a.giveCombatantDie(t, "Pensantus", e)

	held, err := a.answerReaction(t, a.ana, e, w.GetId(), counterspellTypedAnswer(9)) // 9 + 4 = 13 against 14
	if err != nil || held.GetResult() != nil || held.GetHeldRollOffer().GetDie() == nil {
		t.Fatalf("AnswerReaction() = %v, %v; want the die's question alone", held, err)
	}
	if offer := held.GetHeldRollOffer(); offer.GetKind() != playv1.HeldRollKind_HELD_ROLL_KIND_COUNTERSPELL_CHECK || offer.GetD20().GetFaces()[0] != 9 {
		t.Errorf("the offer = %v, want the check's 9", offer)
	}
	if pens := byLabel(t, held.GetEncounter(), "Pensantus"); pens.GetReactionUsed() {
		t.Error("the reaction is spent while the d20 waits")
	}
	if a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_COUNTERSPELL) == nil {
		t.Fatal("the window closed while the d20 waits")
	}

	a.h.roller.queue(3) // 9 + 4 + the die 3 = 16: the spell is countered
	out, err := a.answerHeldRoll(t, a.ana, held.GetHeldRollOffer().GetHoldId(), dieAnswer(true))
	if err != nil {
		t.Fatalf("AnswerHeldRoll() error = %v", err)
	}
	c := out.GetCounterspellCheck().GetResult().GetCounterspell()
	if !c.GetCountered() || c.GetCheck().GetTotal() != 16 || c.GetCheck().GetFaces()[0] != 9 || c.GetCheckDc() != 14 {
		t.Errorf("result = %v, want the spell countered on 9 + 4 + the die 3 = 16", c)
	}
	if pens := byLabel(t, out.GetCounterspellCheck().GetEncounter(), "Pensantus"); !pens.GetReactionUsed() {
		t.Error("the reaction is not spent after the answer")
	}
}

func TestACounterspellCheckWithTheDieKeptCountsTheBareD20(t *testing.T) {
	t.Parallel()
	a, e, w := upcastWindow(t)
	a.giveCombatantDie(t, "Pensantus", e)
	held, err := a.answerReaction(t, a.ana, e, w.GetId(), counterspellTypedAnswer(9))
	if err != nil || held.GetHeldRollOffer() == nil {
		t.Fatalf("AnswerReaction() = %v, %v; want the question", held, err)
	}
	out, err := a.answerHeldRoll(t, a.ana, held.GetHeldRollOffer().GetHoldId(), dieAnswer(false))
	if err != nil {
		t.Fatalf("AnswerHeldRoll() error = %v", err)
	}
	if c := out.GetCounterspellCheck().GetResult().GetCounterspell(); c.GetCountered() || c.GetCheck().GetTotal() != 13 {
		t.Errorf("result = %v, want the spell through on a bare 13", c)
	}
}

func TestACounterspellCheckWithoutADieSettlesAtOnce(t *testing.T) {
	t.Parallel()
	a, e, w := upcastWindow(t)
	res, err := a.answerReaction(t, a.ana, e, w.GetId(), counterspellTypedAnswer(9))
	if err != nil || res.GetHeldRollOffer() != nil || res.GetResult().GetCounterspell() == nil {
		t.Fatalf("AnswerReaction() = %v, %v; want the check settled", res, err)
	}
}
