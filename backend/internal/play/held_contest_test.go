package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The two d20 of a contest are ability checks (SRD 5.1): a Bardic Inspiration die the combatant holds may be added to
// either. The d20 is kept before anything is spent or written, and the answer settles the contest with the die added.

func TestAnInitiatorsContestRollWithADieIsHeldWithNothingSpent(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	c := a.arena(t, arenaPlan{})
	a.giveCombatantDie(t, "Toren", c.e)

	c.h.roller.queue(15)
	held, err := c.startContest(t, a.caio, func(r *playv1.StartContestRequest) {
		r.InitiatorId, r.TargetId, r.Purpose = c.id(t, "Toren"), c.id(t, c.hob), playv1.ContestPurpose_CONTEST_PURPOSE_GRAPPLE
	})
	if err != nil || held.GetContest() != nil || held.GetHeldRollOffer().GetDie() == nil {
		t.Fatalf("StartContest() = %v, %v; want the die's question alone", held, err)
	}
	if offer := held.GetHeldRollOffer(); offer.GetKind() != playv1.HeldRollKind_HELD_ROLL_KIND_CONTEST || offer.GetD20().GetFaces()[0] != 15 {
		t.Errorf("the offer = %v, want the contest's 15", offer)
	}
	if n := len(c.state(t, a.master).GetContests()); n != 0 {
		t.Errorf("%d contests while the d20 waits, want none", n)
	}
	if o := a.mustOptions(t, a.caio, c.e, "Toren"); !grappleOption(o, playv1.ContestAttackOptionKind_CONTEST_ATTACK_OPTION_KIND_GRAPPLE).GetEnabled() {
		t.Error("the grapple is not offered: the attack was spent while the d20 waits")
	}

	c.h.roller.queue(4) // Athletics +3: 15 + 3 + 4 = 22
	out, err := a.answerHeldRoll(t, a.caio, held.GetHeldRollOffer().GetHoldId(), dieAnswer(true))
	if err != nil {
		t.Fatalf("AnswerHeldRoll() error = %v", err)
	}
	contest := out.GetContest().GetContest()
	if roll := contest.GetInitiatorRoll(); roll.GetTotal() != 22 || roll.GetFaces()[0] != 15 {
		t.Errorf("Toren's roll = %v, want d20 15 + 3 + the die 4 = 22", roll)
	}
	if contest.GetWaitingFor() != playv1.ContestWaitFor_CONTEST_WAIT_FOR_MASTER {
		t.Errorf("contest = %v, want awaiting the master", contest)
	}
	if o := a.mustOptions(t, a.caio, c.e, "Toren"); grappleOption(o, playv1.ContestAttackOptionKind_CONTEST_ATTACK_OPTION_KIND_GRAPPLE).GetEnabled() {
		t.Error("a second grapple is offered with the attack spent")
	}
}

func TestADefendersRollWithADieIsHeldAndTheDieAddsToIt(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	c := a.arena(t, arenaPlan{})
	c.advance(t, c.hob)
	a.giveCombatantDie(t, "Toren", c.e)

	c.h.roller.queue(10) // the Hobgoblin's Athletics 10 + 1 = 11
	started := c.mustStart(t, a.master, func(r *playv1.StartContestRequest) {
		r.InitiatorId, r.TargetId, r.Purpose = c.id(t, c.hob), c.id(t, "Toren"), playv1.ContestPurpose_CONTEST_PURPOSE_GRAPPLE
	})
	// Toren's Athletics +3: 5 + 3 = 8 loses, 8 + the die 4 = 12 wins.
	held, err := c.respond(t, a.caio, started.GetContest().GetId(), playv1.ContestSkill_CONTEST_SKILL_ATHLETICS, 5)
	if err != nil || held.GetContest() != nil || held.GetHeldRollOffer().GetDie() == nil {
		t.Fatalf("RespondContest() = %v, %v; want the die's question alone", held, err)
	}
	if offer := held.GetHeldRollOffer(); offer.GetKind() != playv1.HeldRollKind_HELD_ROLL_KIND_CONTEST {
		t.Errorf("the offer kind = %v, want the contest answer's", offer.GetKind())
	}
	if c.hasCondition(t, "Toren", condGrappled) {
		t.Fatal("Toren is Grappled while his d20 waits")
	}
	c.h.roller.queue(4)
	out, err := a.answerHeldRoll(t, a.caio, held.GetHeldRollOffer().GetHoldId(), dieAnswer(true))
	if err != nil {
		t.Fatalf("AnswerHeldRoll() error = %v", err)
	}
	answered := out.GetContestAnswer().GetContest()
	if answered.GetDefenderRoll().GetTotal() != 12 || answered.GetWinner() != playv1.ContestWinner_CONTEST_WINNER_DEFENDER {
		t.Errorf("answer = %v, want Toren to win with 5 + 3 + the die 4 = 12 against 11", answered)
	}
	if c.hasCondition(t, "Toren", condGrappled) {
		t.Error("Toren is Grappled after winning")
	}
}

func TestTheMastersContestRollForAPlayerAsksAndWaitsForThePlayer(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	c := a.arena(t, arenaPlan{})
	a.giveCombatantDie(t, "Toren", c.e)
	c.h.roller.queue(15)
	held, err := c.startContest(t, a.master, func(r *playv1.StartContestRequest) {
		r.InitiatorId, r.TargetId, r.Purpose = c.id(t, "Toren"), c.id(t, c.hob), playv1.ContestPurpose_CONTEST_PURPOSE_GRAPPLE
	})
	if err != nil || held.GetHeldRollOffer().GetDie() == nil {
		t.Fatalf("StartContest(as the master) = %v, %v; want the question", held, err)
	}
	if _, err := a.answerHeldRoll(t, a.master, held.GetHeldRollOffer().GetHoldId(), dieAnswer(false)); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("the master's early answer error = %v, want failed_precondition", err)
	}
	if _, err := a.answerHeldRoll(t, a.caio, held.GetHeldRollOffer().GetHoldId(), dieAnswer(false)); err != nil {
		t.Errorf("the player's answer error = %v", err)
	}
}

func TestACreaturesContestRollWaitsForNobody(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	c := a.arena(t, arenaPlan{})
	c.advance(t, c.hob)
	res, err := c.startContest(t, a.master, func(r *playv1.StartContestRequest) {
		c.h.roller.queue(10)
		r.InitiatorId, r.TargetId, r.Purpose = c.id(t, c.hob), c.id(t, "Toren"), playv1.ContestPurpose_CONTEST_PURPOSE_GRAPPLE
	})
	if err != nil || res.GetHeldRollOffer() != nil || res.GetContest() == nil {
		t.Fatalf("StartContest() = %v, %v; want the contest opened", res, err)
	}
}
