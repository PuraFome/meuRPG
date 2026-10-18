package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The Stealth check of a Hide is an ability check (SRD 5.1): a Bardic Inspiration die the combatant holds may be added to
// it. The d20 is kept before the action is spent or the attempt is written, and the answer settles it with the die added.

func TestAHideWithADieIsHeldWithNothingSpentAndTheDieAddsToIt(t *testing.T) {
	t.Parallel()
	a := newSneaks(t)
	c := a.arena(t, arenaPlan{})
	a.giveCombatantDie(t, "Toren", c.e)

	held, err := c.hide(t, a.caio, "Toren", cunningHide, 7)
	if err != nil || held.GetAttempt() != nil || held.GetHeldRollOffer().GetDie() == nil {
		t.Fatalf("Hide() = %v, %v; want the die's question alone", held, err)
	}
	if offer := held.GetHeldRollOffer(); offer.GetKind() != playv1.HeldRollKind_HELD_ROLL_KIND_HIDE || offer.GetD20().GetFaces()[0] != 7 {
		t.Errorf("the offer = %v, want the Hide's 7", offer)
	}
	if opts := a.mustOptions(t, a.caio, c.refresh(t), "Toren"); opts.GetOptions().GetEconomy().GetBonusAction().GetUsed() {
		t.Error("the bonus action is spent while the d20 waits")
	}

	c.h.roller.queue(4) // Stealth +3: 7 + 3 + the die 4 = 14
	out, err := a.answerHeldRoll(t, a.caio, held.GetHeldRollOffer().GetHoldId(), dieAnswer(true))
	if err != nil {
		t.Fatalf("AnswerHeldRoll() error = %v", err)
	}
	attempt := out.GetHide().GetAttempt()
	if attempt.GetStatus() != playv1.HideAttemptStatus_HIDE_ATTEMPT_STATUS_PENDING || attempt.GetRoll().GetTotal() != 14 || attempt.GetRoll().GetFaces()[0] != 7 {
		t.Errorf("the attempt = %v, want pending, d20 7 + 3 + the die 4 = 14", attempt)
	}
	if opts := a.mustOptions(t, a.caio, c.refresh(t), "Toren"); !opts.GetOptions().GetEconomy().GetBonusAction().GetUsed() {
		t.Error("the bonus action is not spent after the answer")
	}
}

func TestTheMastersHideRollForAPlayerAsksAndWaitsForThePlayer(t *testing.T) {
	t.Parallel()
	a := newSneaks(t)
	c := a.arena(t, arenaPlan{})
	a.giveCombatantDie(t, "Toren", c.e)
	held, err := c.hide(t, a.master, "Toren", cunningHide, 7)
	if err != nil || held.GetHeldRollOffer().GetDie() == nil {
		t.Fatalf("Hide(as the master) = %v, %v; want the question", held, err)
	}
	if _, err := a.answerHeldRoll(t, a.master, held.GetHeldRollOffer().GetHoldId(), dieAnswer(false)); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("the master's early answer error = %v, want failed_precondition", err)
	}
	if _, err := a.answerHeldRoll(t, a.caio, held.GetHeldRollOffer().GetHoldId(), dieAnswer(false)); err != nil {
		t.Errorf("the player's answer error = %v", err)
	}
}

func TestACreaturesHideWaitsForNobody(t *testing.T) {
	t.Parallel()
	a := newSneaks(t)
	c := a.arena(t, arenaPlan{})
	c.advance(t, c.hob)
	res, err := c.hide(t, a.master, c.hob, actionHide, 10)
	if err != nil || res.GetHeldRollOffer() != nil {
		t.Fatalf("Hide(creature) = %v, %v; want the attempt kept", res, err)
	}
}
