package play

import (
	"testing"

	"google.golang.org/protobuf/proto"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

const acidArrow = "spell:acid-arrow"

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

// retryTheAnswer answers the die's question again with the same key, which a flaky network does: the answer gives the stored
// outcome (not an error) and rolls nothing (the roller's queue holds a sentinel that must stay there).
func (a *armed) retryTheAnswer(t *testing.T, roller *scriptedRoller, holdID, key string, first *playv1.AnswerHeldRollResponse) {
	t.Helper()
	a.retryTheAnswerOf(t, roller, a.caio, holdID, key, dieAnswer(true), first)
}

// retryTheAnswerOf is retryTheAnswer for the answer u gave, as edit says.
func (a *armed) retryTheAnswerOf(t *testing.T, roller *scriptedRoller, u *user, holdID, key string, edit func(*playv1.AnswerHeldRollRequest), first *playv1.AnswerHeldRollResponse) {
	t.Helper()
	roller.queue(heldSentinel)
	again, err := a.answerHeldRollKey(t, u, holdID, key, edit)
	if err != nil {
		t.Fatalf("the answer again: error = %v, want the stored outcome", err)
	}
	if !proto.Equal(again, first) {
		t.Errorf("the answer again = %v, want what it gave before: %v", again, first)
	}
	if n := roller.pending(); n != 1 {
		t.Errorf("the answer again took %d faces from the roller, want none", 1-n)
	}
	roller.drain()
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
	key := newKey()
	out, err := a.answerHeldRollKey(t, a.caio, holdID, key, dieAnswer(true))
	if err != nil {
		t.Fatalf("AnswerHeldRoll() error = %v", err)
	}
	a.retryTheAnswer(t, c.h.roller, holdID, key, out)
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
	key := newKey()
	out, err := a.answerHeldRollKey(t, a.caio, holdID, key, dieAnswer(true))
	if err != nil {
		t.Fatalf("AnswerHeldRoll() error = %v", err)
	}
	a.retryTheAnswer(t, c.h.roller, holdID, key, out)
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
	key := newKey()
	out, err := a.answerHeldRollKey(t, a.caio, holdID, key, dieAnswer(true))
	if err != nil {
		t.Fatalf("AnswerHeldRoll() error = %v", err)
	}
	a.retryTheAnswer(t, c.h.roller, holdID, key, out)
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

// A cast with a spell attack that the master's window held (a player's spell on an enemy, with "Sempre") is replayed when the
// window is answered, and a caster that is to be asked about the d20 is asked then, as a weapon attack is: the replay keeps the d20
// with the request, the question under the request's key and marked as the window's (window_hold), and nothing is spent or written
// until the player answers. The answer casts the spell with that d20, once, under the answer's own key.

// spellAfterAWindow is Pensantus's turn in the arena, with the master's check asked of every roll against an enemy; race is hers.
func spellAfterAWindow(t *testing.T, race string) (*armed, *cx) {
	t.Helper()
	a := newArmedWith(t, func(a *armed) {
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 2, scores16(), []string{battleaxe}, nil)
		a.pens = a.ana.caster(t, a.campaignID, "Pensantus", "class:wizard", race, 3,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 14, Constitution: 12, Intelligence: 16, Wisdom: 10, Charisma: 8}, nil, []string{fireBolt},
			[]string{acidArrow}, []string{acidArrow})
		a.bri = a.bia.hero(t, a.campaignID, "Brisa", "class:fighter", "race:human", 2, scores16(), []string{rapier}, nil)
	})
	a.setRules(t, enemyReactionsAlways)
	c := a.arena(t, arenaPlan{})
	c.advance(t, "Pensantus")
	return a, c
}

// castInTheWindow casts Acid Arrow at the goblin with the d20 typed, and answers the master's check the cast waits in. It returns the
// key of the cast.
func (a *armed) castInTheWindow(t *testing.T, c *cx, face int32) string {
	t.Helper()
	key := newKey()
	held, err := a.castKey(t, a.ana, c.e, "Pensantus", acidArrow, slotOfLevel(2), a.at(t, "Goblin"), d20Face(face), key)
	if err != nil || held.GetCast() != nil || held.GetHeldRollOffer() != nil || held.GetInspirationOffer() != nil || held.GetLuckyOffer() != nil {
		t.Fatalf("CastSpell() = %v, %v; want it held in the master's window", held, err)
	}
	w := a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_MASTER_CHECK)
	if w == nil {
		t.Fatalf("the master's windows = %v, want the check the cast waits in", a.windows(t, a.master))
	}
	a.mustAnswer(t, a.master, c.e, w.GetId(), passAnswer)
	return key
}

// wantTheQuestionOfTheReplay checks the replay of the cast put one question to Pensantus's player, kept under the cast's key as the
// window's own, and that the cast spent and wrote nothing yet. It returns the hold's id.
func (a *armed) wantTheQuestionOfTheReplay(t *testing.T, c *cx, key string) *playv1.HeldRollOffer {
	t.Helper()
	offers := a.listHeldRolls(t, a.ana)
	if len(offers) != 1 || offers[0].GetKind() != playv1.HeldRollKind_HELD_ROLL_KIND_SPELL_ATTACK {
		t.Fatalf("Pensantus's questions after the window = %v, want one, about the spell attack", offers)
	}
	var kind string
	var window, unanswered bool
	if err := a.h.pool.QueryRow(t.Context(), `SELECT kind, window_hold, answer_key IS NULL FROM inspiration_holds WHERE idempotency_key = $1`, key).Scan(&kind, &window, &unanswered); err != nil {
		t.Fatalf("read the question kept under the cast's key: %v", err)
	}
	if kind != string(heldSpellAttack) || !window || !unanswered {
		t.Errorf("the hold = kind %q, window_hold %v, open %v; want a spell attack, the window's, open", kind, window, unanswered)
	}
	if n := a.countOf(t, `SELECT count(*) FROM session_events WHERE kind = $1 AND NOT COALESCE((payload->'reaction'->>'hold')::bool, false)`, eventSpellCast); n != 0 {
		t.Errorf("%s events before the answer = %d, want 0", eventSpellCast, n)
	}
	if byLabel(t, a.get(t, a.ana), "Pensantus").GetActionUsed() {
		t.Error("the action was spent before the player answered")
	}
	return offers[0]
}

// wantTheCastOnce checks the answer settled the cast once: one event that is no hold, and the action spent.
func (a *armed) wantTheCastOnce(t *testing.T, c *cx) {
	t.Helper()
	if n := a.countOf(t, `SELECT count(*) FROM session_events WHERE kind = $1 AND NOT COALESCE((payload->'reaction'->>'hold')::bool, false)`, eventSpellCast); n != 1 {
		t.Errorf("%s events = %d, want 1", eventSpellCast, n)
	}
	if !byLabel(t, a.get(t, a.ana), "Pensantus").GetActionUsed() {
		t.Error("the action is not spent after the answer")
	}
}

// TestASpellAttackCastThatAWindowHeldAsksForTheDieAndUsesIt: Pensantus holds a Bardic Inspiration die; the d20 is 15 and her bonus +5.
// The die's 4 is added: 15 + 5 + 4 = 24. The same answer again gives the same cast and rolls nothing; the cast is taken back with the
// die returned, as any cast's.
func TestASpellAttackCastThatAWindowHeldAsksForTheDieAndUsesIt(t *testing.T) {
	t.Parallel()
	a, c := spellAfterAWindow(t, "race:gnome")
	a.giveCombatantDie(t, "Pensantus", c.e)
	key := a.castInTheWindow(t, c, 15)
	offer := a.wantTheQuestionOfTheReplay(t, c, key)
	if offer.GetDie() == nil || offer.GetD20().GetFaces()[0] != 15 {
		t.Errorf("the question = %v, want the die's, about the d20 15", offer)
	}
	// The cast again, with the request's key: the answer it had, nothing written.
	events := a.countOf(t, `SELECT count(*) FROM session_events`)
	if again, err := a.castKey(t, a.ana, c.e, "Pensantus", acidArrow, slotOfLevel(2), a.at(t, "Goblin"), d20Face(15), key); err != nil || again.GetCast() != nil {
		t.Errorf("the cast again = %v, %v; want it still held", again, err)
	}
	if n := a.countOf(t, `SELECT count(*) FROM session_events`); n != events {
		t.Errorf("the cast again wrote %d events", n-events)
	}

	c.h.roller.queue(4)
	answerKey := newKey()
	out, err := a.answerHeldRollKey(t, a.ana, offer.GetHoldId(), answerKey, dieAnswer(true))
	if err != nil {
		t.Fatalf("AnswerHeldRoll() error = %v", err)
	}
	a.retryTheAnswerOf(t, c.h.roller, a.ana, offer.GetHoldId(), answerKey, dieAnswer(true), out)
	targets := out.GetSpellCast().GetCast().GetTargets()
	if len(targets) != 1 || targets[0].GetAttackRoll().GetFaces()[0] != 15 || targets[0].GetAttackRoll().GetTotal() != 24 ||
		len(targets[0].GetBonusDice()) != 1 || targets[0].GetBonusDice()[0].GetFace() != 4 {
		t.Fatalf("the cast = %v, want the d20 15 + 5 + the die 4 = 24", out.GetSpellCast().GetCast())
	}
	a.wantTheCastOnce(t, c)
	if n := a.countOf(t, `SELECT count(*) FROM combatants WHERE id = $1 AND inspiration_sides IS NOT NULL`, c.id(t, "Pensantus")); n != 0 {
		t.Error("Pensantus still holds the die she used")
	}
	if n := a.countOf(t, `SELECT count(*) FROM inspiration_holds WHERE idempotency_key = $1`, key); n != 1 {
		t.Errorf("the holds under the cast's key = %d, want 1", n)
	}
	// The undo of the cast gives the die and the action back.
	a.undoLast(t, a.get(t, a.master))
	if n := a.countOf(t, `SELECT count(*) FROM combatants WHERE id = $1 AND inspiration_sides IS NOT NULL`, c.id(t, "Pensantus")); n != 1 {
		t.Error("the undo did not give the die back")
	}
	if byLabel(t, a.get(t, a.ana), "Pensantus").GetActionUsed() {
		t.Error("the undo did not give the action back")
	}
	if n := a.countOf(t, `SELECT count(*) FROM session_events WHERE kind = $1 AND NOT COALESCE((payload->'reaction'->>'hold')::bool, false)`, eventSpellCast); n != 0 {
		t.Errorf("%s events after the undo = %d, want 0", eventSpellCast, n)
	}
}

// TestASpellAttackCastThatAWindowHeldAsksForTheDieAndKeepsIt: the player keeps the die: 15 + 5 = 20, and the die stays.
func TestASpellAttackCastThatAWindowHeldAsksForTheDieAndKeepsIt(t *testing.T) {
	t.Parallel()
	a, c := spellAfterAWindow(t, "race:gnome")
	a.giveCombatantDie(t, "Pensantus", c.e)
	key := a.castInTheWindow(t, c, 15)
	offer := a.wantTheQuestionOfTheReplay(t, c, key)

	answerKey := newKey()
	out, err := a.answerHeldRollKey(t, a.ana, offer.GetHoldId(), answerKey, dieAnswer(false))
	if err != nil {
		t.Fatalf("AnswerHeldRoll() error = %v", err)
	}
	a.retryTheAnswerOf(t, c.h.roller, a.ana, offer.GetHoldId(), answerKey, dieAnswer(false), out)
	targets := out.GetSpellCast().GetCast().GetTargets()
	if len(targets) != 1 || targets[0].GetAttackRoll().GetTotal() != 20 || len(targets[0].GetBonusDice()) != 0 {
		t.Fatalf("the cast = %v, want the d20 15 + 5 = 20 and no die", out.GetSpellCast().GetCast())
	}
	a.wantTheCastOnce(t, c)
	if n := a.countOf(t, `SELECT count(*) FROM combatants WHERE id = $1 AND inspiration_sides IS NOT NULL`, c.id(t, "Pensantus")); n != 1 {
		t.Error("Pensantus lost the die she kept")
	}
}

// TestASpellAttackCastThatAWindowHeldAsksLuckyAboutAOne: a halfling's 1 is asked about when the replay rolls it, and rolled again to 15
// the cast lands on the 15.
func TestASpellAttackCastThatAWindowHeldAsksLuckyAboutAOne(t *testing.T) {
	t.Parallel()
	a, c := spellAfterAWindow(t, "race:halfling")
	key := a.castInTheWindow(t, c, 1)
	offer := a.wantTheQuestionOfTheReplay(t, c, key)
	if offer.GetLucky() == nil || offer.GetD20().GetFaces()[0] != 1 {
		t.Fatalf("the question = %v, want Lucky's, about the 1", offer)
	}
	answerKey := newKey()
	out, err := a.answerHeldRollKey(t, a.ana, offer.GetHoldId(), answerKey, luckyAnswer(15))
	if err != nil {
		t.Fatalf("AnswerHeldRoll() error = %v", err)
	}
	a.retryTheAnswerOf(t, c.h.roller, a.ana, offer.GetHoldId(), answerKey, luckyAnswer(15), out)
	targets := out.GetSpellCast().GetCast().GetTargets()
	if len(targets) != 1 || targets[0].GetAttackRoll().GetFaces()[0] != 15 || targets[0].GetAttackRoll().GetTotal() != 20 {
		t.Fatalf("the cast = %v, want the 1 rolled again to 15, 15 + 5 = 20", out.GetSpellCast().GetCast())
	}
	a.wantTheCastOnce(t, c)
}

// TestASpellAttackCastThatAWindowHeldAsksNothingOfACasterWithoutADie: no die and no Lucky: the replay writes the cast at once, once.
func TestASpellAttackCastThatAWindowHeldAsksNothingOfACasterWithoutADie(t *testing.T) {
	t.Parallel()
	a, c := spellAfterAWindow(t, "race:gnome")
	a.castInTheWindow(t, c, 15)
	if offers := a.listHeldRolls(t, a.ana); len(offers) != 0 {
		t.Errorf("Pensantus's questions after the window = %v, want none", offers)
	}
	if n := a.countOf(t, `SELECT count(*) FROM inspiration_holds`); n != 0 {
		t.Errorf("inspiration_holds rows = %d, want 0", n)
	}
	a.wantTheCastOnce(t, c)
}
