package play

import (
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The death saving throw of a character at 0 hit points asks the halfling's Lucky about a 1 and the Bardic
// Inspiration die like any other saving throw (SRD 5.1): the d20 is kept, nothing is written, the counts do not move,
// and the answer settles it. A natural 1 stays two failures and a natural 20 the return with 1 hit point whatever the
// die adds.

// dyingToren is the table with Toren (a halfling when lucky) at 0 hit points and a death save due on his turn.
func dyingToren(t *testing.T, race string) (*armed, *playv1.Encounter) {
	t.Helper()
	a := newCastersOf(t, 3, race)
	e := a.castersFight(t, 1)
	a.correct(t, a.toren, hpIs(0))
	a.mustEndTurn(t, a.ana, e) // Toren's turn: the save is due
	return a, e
}

func torenCounts(t *testing.T, a *armed) (successes, failures int32) {
	t.Helper()
	c := byLabel(t, a.get(t, a.master), "Toren")
	return c.GetDeathSuccesses(), c.GetDeathFailures()
}

func TestADeathSaveWithABardicDieIsHeldAndTheDieAddsToIt(t *testing.T) {
	t.Parallel()
	a, e := dyingToren(t, "")
	a.giveCombatantDie(t, "Toren", e)
	res, err := a.deathSave(t, a.caio, e, "Toren", deathFace(7))
	if err != nil || res.GetDeathSave() != nil || res.GetHeldRollOffer().GetDie() == nil {
		t.Fatalf("RollDeathSave() = %v, %v; want the die's question alone", res, err)
	}
	if offer := res.GetHeldRollOffer(); offer.GetKind() != playv1.HeldRollKind_HELD_ROLL_KIND_DEATH_SAVE || offer.GetD20().GetFaces()[0] != 7 {
		t.Errorf("the offer = %v, want the death save's 7", offer)
	}
	if s, f := torenCounts(t, a); s != 0 || f != 0 || !byLabel(t, a.get(t, a.master), "Toren").GetDeathSaveDue() {
		t.Errorf("counts %d/%d, due %v while the question waits; want 0/0 and the save still due", s, f, byLabel(t, a.get(t, a.master), "Toren").GetDeathSaveDue())
	}
	a.h.roller.queue(4) // 7 + 4 = 11: a success the bare 7 would not have been
	out, err := a.answerHeldRoll(t, a.caio, res.GetHeldRollOffer().GetHoldId(), dieAnswer(true))
	if err != nil {
		t.Fatalf("AnswerHeldRoll() error = %v", err)
	}
	ds := out.GetDeathSave().GetDeathSave()
	if ds.GetOutcome() != playv1.DeathSaveOutcome_DEATH_SAVE_OUTCOME_SUCCESS || ds.GetSuccesses() != 1 || ds.GetFailures() != 0 ||
		ds.GetRoll().GetModifier() != 4 || ds.GetRoll().GetTotal() != 11 || len(ds.GetBonusDice()) != 1 {
		t.Errorf("the death save = %v, want a success of 7 + the die 4 = 11", ds)
	}
	if byLabel(t, a.get(t, a.master), "Toren").GetInspirationDie() != nil {
		t.Error("Toren still holds the die after using it")
	}
}

func TestADeathSaveWithTheDieKeptCountsTheBareD20(t *testing.T) {
	t.Parallel()
	a, e := dyingToren(t, "")
	a.giveCombatantDie(t, "Toren", e)
	res, err := a.deathSave(t, a.caio, e, "Toren", deathFace(7))
	if err != nil {
		t.Fatalf("RollDeathSave() error = %v", err)
	}
	out, err := a.answerHeldRoll(t, a.caio, res.GetHeldRollOffer().GetHoldId(), dieAnswer(false))
	if err != nil {
		t.Fatalf("AnswerHeldRoll() error = %v", err)
	}
	if ds := out.GetDeathSave().GetDeathSave(); ds.GetOutcome() != playv1.DeathSaveOutcome_DEATH_SAVE_OUTCOME_FAILURE || ds.GetFailures() != 1 {
		t.Errorf("the death save = %v, want a failure of the bare 7", ds)
	}
	if byLabel(t, a.get(t, a.master), "Toren").GetInspirationDie() == nil {
		t.Error("the kept die is gone")
	}
}

func TestTheDieNeverChangesANaturalOne(t *testing.T) {
	t.Parallel()
	a, e := dyingToren(t, "")
	a.giveCombatantDie(t, "Toren", e)
	res, err := a.deathSave(t, a.caio, e, "Toren", deathFace(1))
	if err != nil {
		t.Fatalf("RollDeathSave() error = %v", err)
	}
	a.h.roller.queue(8)
	out, err := a.answerHeldRoll(t, a.caio, res.GetHeldRollOffer().GetHoldId(), dieAnswer(true))
	if err != nil {
		t.Fatalf("AnswerHeldRoll() error = %v", err)
	}
	if ds := out.GetDeathSave().GetDeathSave(); ds.GetOutcome() != playv1.DeathSaveOutcome_DEATH_SAVE_OUTCOME_CRITICAL_FAILURE || ds.GetFailures() != 2 {
		t.Errorf("the death save = %v, want the natural 1: two failures", ds)
	}
}

func TestAHalflingsDeathSaveAsksLuckyAboutA1(t *testing.T) {
	t.Parallel()
	a, e := dyingToren(t, "race:halfling")
	res, err := a.deathSave(t, a.caio, e, "Toren", deathFace(1))
	if err != nil || res.GetDeathSave() != nil || res.GetHeldRollOffer().GetLucky() == nil {
		t.Fatalf("RollDeathSave() = %v, %v; want Lucky's question alone", res, err)
	}
	if s, f := torenCounts(t, a); s != 0 || f != 0 {
		t.Errorf("counts %d/%d while the question waits; want 0/0", s, f)
	}
	out, err := a.answerHeldRoll(t, a.caio, res.GetHeldRollOffer().GetHoldId(), luckyAnswer(14))
	if err != nil {
		t.Fatalf("AnswerHeldRoll() error = %v", err)
	}
	ds := out.GetDeathSave().GetDeathSave()
	if ds.GetOutcome() != playv1.DeathSaveOutcome_DEATH_SAVE_OUTCOME_SUCCESS || ds.GetSuccesses() != 1 || ds.GetFailures() != 0 ||
		ds.GetRoll().GetFaces()[0] != 14 || len(ds.GetRoll().GetRerolled()) != 1 || ds.GetRoll().GetRerolled()[0].GetFrom() != 1 {
		t.Errorf("the death save = %v, want the 1 rolled again to 14: a success", ds)
	}
	log := logEntries(a.log(t, a.master, a.get(t, a.master)))
	var line *playv1.CombatLogDeathSave
	for _, l := range log {
		if l.GetDeathSave() != nil {
			line = l.GetDeathSave()
		}
	}
	if line == nil || len(line.GetRoll().GetRerolled()) != 1 {
		t.Errorf("the log line = %v, want the reroll", line)
	}
}

func TestADeathSaveWithoutADieOrLuckSettlesAtOnce(t *testing.T) {
	t.Parallel()
	a, e := dyingToren(t, "")
	res, err := a.deathSave(t, a.caio, e, "Toren", deathFace(12))
	if err != nil || res.GetHeldRollOffer() != nil || res.GetDeathSave().GetSuccesses() != 1 {
		t.Fatalf("RollDeathSave() = %v, %v; want the save settled", res, err)
	}
}

func TestTheUndoOfADeathSaveGivesTheDieAndTheCountsBack(t *testing.T) {
	t.Parallel()
	a, e := dyingToren(t, "")
	a.giveCombatantDie(t, "Toren", e)
	a.undoes(t, "a death save with the die", func() {
		res, err := a.deathSave(t, a.caio, e, "Toren", deathFace(7))
		if err != nil {
			t.Fatalf("RollDeathSave() error = %v", err)
		}
		a.h.roller.queue(4)
		if _, err := a.answerHeldRoll(t, a.caio, res.GetHeldRollOffer().GetHoldId(), dieAnswer(true)); err != nil {
			t.Fatalf("AnswerHeldRoll() error = %v", err)
		}
	})
}

func TestTheMastersDeathSaveForAPlayerAsksAndWaitsForThePlayer(t *testing.T) {
	t.Parallel()
	a, e := dyingToren(t, "")
	a.giveCombatantDie(t, "Toren", e)
	res, err := a.deathSave(t, a.master, e, "Toren", deathFace(7))
	if err != nil || res.GetHeldRollOffer() == nil {
		t.Fatalf("RollDeathSave(master) = %v, %v; want the die's question", res, err)
	}
	hold := res.GetHeldRollOffer().GetHoldId()
	if got := byLabel(t, a.get(t, a.caio), "Toren").GetHeldRollOffer().GetHoldId(); got != hold {
		t.Errorf("Toren's player reads %q, want the same question", got)
	}
	_, err = a.answerHeldRoll(t, a.master, hold, dieAnswer(false))
	wantResourceBlocked(t, err, playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_PLAYER_FIRST)
	a.h.afterThePlayersChance()
	out, err := a.answerHeldRoll(t, a.master, hold, dieAnswer(false))
	if err != nil || out.GetDeathSave().GetDeathSave().GetFailures() != 1 {
		t.Fatalf("the master's answer after the time = %v, %v; want the failure of the bare 7", out, err)
	}
}
