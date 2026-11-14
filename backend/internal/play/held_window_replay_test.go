package play

import (
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// A contest start, a contest answer or a Hide that the master's reaction window held (the table asks for the master's check on
// every roll against an enemy) is run again when the window is answered. When that replay holds the d20 for the player's
// Bardic Inspiration die, the player's answer must write what the held request would have written: the contest, its answer or
// the attempt, and its event, once (the event that stands for the held request, which never reaches the log, is not counted).

// windowReplayAsks answers the window: it reads the master's check the request waits in, has the master answer it, and returns the die's
// question the replay put to the player (its hold id).
func (a *armed) windowReplayAsks(t *testing.T, e *playv1.Encounter) string {
	t.Helper()
	w := a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_MASTER_CHECK)
	if w == nil {
		t.Fatalf("the master's windows = %v, want the check the roll waits in", a.windows(t, a.master))
	}
	a.mustAnswer(t, a.master, e, w.GetId(), passAnswer)
	offers := a.listHeldRolls(t, a.caio)
	if len(offers) != 1 || offers[0].GetDie() == nil {
		t.Fatalf("Toren's questions after the window = %v, want the die's", offers)
	}
	return offers[0].GetHoldId()
}

// countOf counts what a query counts.
func (a *armed) countOf(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	if err := a.h.pool.QueryRow(t.Context(), query, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

func TestAContestStartThatAWindowHeldIsWrittenWhenTheDieIsAnswered(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	a.setRules(t, enemyReactionsAlways)
	c := a.arena(t, arenaPlan{})
	a.giveCombatantDie(t, "Toren", c.e)

	c.h.roller.queue(15)
	held, err := c.startContest(t, a.caio, func(r *playv1.StartContestRequest) {
		r.InitiatorId, r.TargetId, r.Purpose = c.id(t, "Toren"), c.id(t, c.hob), playv1.ContestPurpose_CONTEST_PURPOSE_GRAPPLE
	})
	if err != nil || held.GetContest() != nil || held.GetHeldRollOffer() != nil {
		t.Fatalf("StartContest() = %v, %v; want it held in the master's window", held, err)
	}
	holdID := a.windowReplayAsks(t, c.e)

	c.h.roller.queue(4) // Athletics +3: 15 + 3 + the die 4 = 22
	out, err := a.answerHeldRoll(t, a.caio, holdID, dieAnswer(true))
	if err != nil {
		t.Fatalf("AnswerHeldRoll() error = %v", err)
	}
	if roll := out.GetContest().GetContest().GetInitiatorRoll(); roll.GetTotal() != 22 || roll.GetFaces()[0] != 15 {
		t.Errorf("the answer's contest roll = %v, want d20 15 + 3 + the die 4 = 22", roll)
	}
	if n := a.countOf(t, `SELECT count(*) FROM combat_contests`); n != 1 {
		t.Errorf("combat_contests rows = %d, want 1", n)
	}
	if n := a.countOf(t, `SELECT count(*) FROM session_events WHERE kind = $1 AND NOT COALESCE((payload->'reaction'->>'hold')::bool, false)`, eventContestStarted); n != 1 {
		t.Errorf("%s events = %d, want 1", eventContestStarted, n)
	}
	if got := c.state(t, a.master).GetContests(); len(got) != 1 || got[0].GetInitiatorRoll().GetTotal() != 22 {
		t.Errorf("the contests the master reads = %v, want one with the die counted", got)
	}
	if n := a.countOf(t, `SELECT count(*) FROM combatants WHERE id = $1 AND inspiration_sides IS NOT NULL`, c.id(t, "Toren")); n != 0 {
		t.Error("Toren still holds the die he used")
	}
}

func TestAContestAnswerThatAWindowHeldIsWrittenWhenTheDieIsAnswered(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	a.setRules(t, enemyReactionsAlways)
	c := a.arena(t, arenaPlan{})
	c.advance(t, c.hob)
	a.giveCombatantDie(t, "Toren", c.e)

	c.h.roller.queue(10) // the Hobgoblin's Athletics 10 + 1 = 11
	started := c.mustStart(t, a.master, func(r *playv1.StartContestRequest) {
		r.InitiatorId, r.TargetId, r.Purpose = c.id(t, c.hob), c.id(t, "Toren"), playv1.ContestPurpose_CONTEST_PURPOSE_GRAPPLE
	})
	// Toren's Athletics +3: 5 + 3 = 8 loses, 8 + the die 4 = 12 wins.
	held, err := c.respond(t, a.caio, started.GetContest().GetId(), playv1.ContestSkill_CONTEST_SKILL_ATHLETICS, 5)
	if err != nil || held.GetContest() != nil || held.GetHeldRollOffer() != nil {
		t.Fatalf("RespondContest() = %v, %v; want it held in the master's window", held, err)
	}
	holdID := a.windowReplayAsks(t, c.e)

	c.h.roller.queue(4)
	out, err := a.answerHeldRoll(t, a.caio, holdID, dieAnswer(true))
	if err != nil {
		t.Fatalf("AnswerHeldRoll() error = %v", err)
	}
	answered := out.GetContestAnswer().GetContest()
	if answered.GetDefenderRoll().GetTotal() != 12 || answered.GetWinner() != playv1.ContestWinner_CONTEST_WINNER_DEFENDER {
		t.Errorf("answer = %v, want Toren to win with 5 + 3 + the die 4 = 12 against 11", answered)
	}
	if n := a.countOf(t, `SELECT count(*) FROM combat_contests WHERE defender_roll IS NOT NULL`); n != 1 {
		t.Errorf("combat_contests rows with the defender's roll = %d, want 1", n)
	}
	if n := a.countOf(t, `SELECT count(*) FROM session_events WHERE kind = $1 AND NOT COALESCE((payload->'reaction'->>'hold')::bool, false)`, eventContestResolved); n != 1 {
		t.Errorf("%s events = %d, want 1", eventContestResolved, n)
	}
	if got := c.state(t, a.master).GetContests(); len(got) != 1 || got[0].GetDefenderRoll().GetTotal() != 12 {
		t.Errorf("the contests the master reads = %v, want one with Toren's roll", got)
	}
}

func TestAHideThatAWindowHeldIsWrittenWhenTheDieIsAnswered(t *testing.T) {
	t.Parallel()
	a := newSneaks(t)
	a.setRules(t, enemyReactionsAlways)
	c := a.arena(t, arenaPlan{})
	a.giveCombatantDie(t, "Toren", c.e)

	held, err := c.hide(t, a.caio, "Toren", cunningHide, 7)
	if err != nil || held.GetAttempt() != nil || held.GetHeldRollOffer() != nil {
		t.Fatalf("Hide() = %v, %v; want it held in the master's window", held, err)
	}
	holdID := a.windowReplayAsks(t, c.e)

	c.h.roller.queue(4) // Stealth +3: 7 + 3 + the die 4 = 14
	out, err := a.answerHeldRoll(t, a.caio, holdID, dieAnswer(true))
	if err != nil {
		t.Fatalf("AnswerHeldRoll() error = %v", err)
	}
	if attempt := out.GetHide().GetAttempt(); attempt.GetRoll().GetTotal() != 14 || attempt.GetRoll().GetFaces()[0] != 7 {
		t.Errorf("the attempt = %v, want d20 7 + 3 + the die 4 = 14", attempt)
	}
	if n := a.countOf(t, `SELECT count(*) FROM combatants WHERE id = $1 AND inspiration_sides IS NOT NULL`, c.id(t, "Toren")); n != 0 {
		t.Error("Toren still holds the die he used")
	}
	if n := a.countOf(t, `SELECT count(*) FROM combat_hide_attempts`); n != 1 {
		t.Errorf("combat_hide_attempts rows = %d, want 1", n)
	}
	if n := a.countOf(t, `SELECT count(*) FROM session_events WHERE kind = $1 AND NOT COALESCE((payload->'reaction'->>'hold')::bool, false)`, eventHideAttempted); n != 1 {
		t.Errorf("%s events = %d, want 1", eventHideAttempted, n)
	}
	if got := c.state(t, a.master).GetHideAttempts(); len(got) != 1 || got[0].GetRoll().GetTotal() != 14 {
		t.Errorf("the hide attempts the master reads = %v, want one with the die counted", got)
	}
}
