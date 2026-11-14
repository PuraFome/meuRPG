package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// untouched says nothing was rolled (the scripted dice are all still there) and the session's number is where it was.
func (e *checkEnv) untouched(t *testing.T, what string, queued int, seq int64) {
	t.Helper()
	if got := e.h.roller.pending(); got != queued {
		t.Errorf("%s: the scripted dice left = %d, want %d: a d20 was rolled", what, got, queued)
	}
	if got := e.seq(t); got != seq {
		t.Errorf("%s: the session's number moved %d -> %d, want it still", what, seq, got)
	}
}

// The refusals spend nothing, roll nothing and move no number: no Inspiration, and an advantage the Inspiration cannot add to.
func TestFavorRefusalsOnACheck(t *testing.T) {
	t.Parallel()
	forEachCheck(t, func(t *testing.T, e *checkEnv) {
		t.Helper()
		e.h.roller.queue(4, 15)
		seq := e.seq(t)
		_, err := e.send(t, newKey(), true)
		wantFavorBlocked(t, e.name+" with none", err, playv1.FavorBlockedReason_FAVOR_BLOCKED_REASON_HAS_NONE)
		e.untouched(t, "no Inspiration", 2, seq)

		e.give(t, e.who)
		e.advantage(t)
		seq = e.seq(t)
		_, err = e.send(t, newKey(), true)
		wantFavorBlocked(t, e.name+" that has an advantage", err, playv1.FavorBlockedReason_FAVOR_BLOCKED_REASON_ADDS_NOTHING)
		e.untouched(t, "an advantage already", 2, seq)
		if !e.favorHolds(t) {
			t.Error("a refused spend took the Inspiration")
		}
		// Without the spend the same roll is made, with the advantage it has and the Inspiration kept.
		got, err := e.send(t, newKey(), false)
		if err != nil || len(got.faces) != 2 || got.mode != "advantage" || got.hasFavor() || !e.favorHolds(t) {
			t.Errorf("the roll that does not spend = %+v, %v (Inspiration there %v), want its own advantage without the source", got, err, e.favorHolds(t))
		}
	})
}

const hintTryName = "a hint try"

// A retry of a roll that was written answers the same, spends nothing again and rolls nothing; a retry that flips the spend is another
// request under the same key (the hint try, which keeps no request hash, answers the stored try).
func TestFavorSettledRetryOfACheck(t *testing.T) {
	t.Parallel()
	forEachCheck(t, func(t *testing.T, e *checkEnv) {
		t.Helper()
		e.give(t, e.who)
		key := newKey()
		e.h.roller.queue(4, 15)
		first, err := e.send(t, key, true)
		if err != nil {
			t.Fatalf("%s error = %v", e.name, err)
		}
		e.give(t, e.who) // another one: the retry must leave it
		e.h.roller.queue(9, 9)
		seq := e.seq(t)
		again, err := e.send(t, key, true)
		if err != nil || len(again.faces) != 2 || again.mode != "advantage" || len(again.faces) != len(first.faces) || !e.favorHolds(t) {
			t.Errorf("the retry = %+v, %v (Inspiration there %v), want the written roll and the second Inspiration left", again, err, e.favorHolds(t))
		}
		e.untouched(t, "the retry", 2, seq)
		flipped, err := e.send(t, key, false)
		if e.name == hintTryName {
			// The hint try has no request hash: a retry of a written try answers the stored result and ignores the flag.
			if err != nil || len(flipped.faces) != len(first.faces) || flipped.mode != first.mode {
				t.Errorf("the hint retry that drops the spend = %+v, %v, want the stored answer", flipped, err)
			}
		} else if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("the retry that drops the spend error = %v, want invalid_argument (the key is another request's)", err)
		}
		e.untouched(t, "the retry that drops the spend", 2, seq)
	})
}

// A retry that adds the spend to a roll that was made without it is not the same request either: the d20 is rolled already.
func TestFavorRetryThatAddsTheSpendToAWrittenCheck(t *testing.T) {
	t.Parallel()
	forEachCheck(t, func(t *testing.T, e *checkEnv) {
		t.Helper()
		key := newKey()
		e.h.roller.queue(4)
		if _, err := e.send(t, key, false); err != nil {
			t.Fatalf("%s error = %v", e.name, err)
		}
		e.give(t, e.who)
		e.h.roller.queue(9, 9)
		seq := e.seq(t)
		got, err := e.send(t, key, true)
		if e.name == hintTryName {
			// The stored try answers, rolled as it was (one d20, no advantage), and the retry spends nothing.
			if err != nil || len(got.faces) != 1 || got.mode != "normal" {
				t.Errorf("the hint retry that adds the spend = %+v, %v, want the stored answer", got, err)
			}
		} else if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("the retry that adds the spend error = %v, want invalid_argument (the key is another request's)", err)
		}
		e.untouched(t, "the retry that adds the spend", 2, seq)
		if !e.favorHolds(t) {
			t.Error("a refused retry took the Inspiration")
		}
	})
}

// holdSpent is the favor_spent of the held roll.
func (e *checkEnv) holdSpent(t *testing.T, holdID string) bool {
	t.Helper()
	var spent bool
	if err := e.h.pool.QueryRow(t.Context(), `SELECT favor_spent FROM inspiration_holds WHERE id = $1`, holdID).Scan(&spent); err != nil {
		t.Fatalf("read the hold %s: %v", holdID, err)
	}
	return spent
}

// A roll that waits for the Bardic Inspiration question keeps the Inspiration it spent in its hold: a retry under the same key finds the row
// gone and spends nothing more (and is never told it has none), and the answer writes the roll with the advantage and the source.
func TestFavorOnAHeldCheck(t *testing.T) {
	t.Parallel()
	forEachCheck(t, func(t *testing.T, e *checkEnv) {
		t.Helper()
		e.die(t)
		e.give(t, e.who)
		key := newKey()
		e.h.roller.queue(4, 15)
		held, err := e.send(t, key, true)
		if err != nil || held.holdID == "" {
			t.Fatalf("%s with a die and the spend = %+v, %v, want the die's question", e.name, held, err)
		}
		if e.favorHolds(t) || !e.holdSpent(t, held.holdID) {
			t.Errorf("the Inspiration is there = %v, the hold remembers it = %v, want it spent and kept in the hold", e.favorHolds(t), e.holdSpent(t, held.holdID))
		}

		e.give(t, e.who) // another one: the retry and the answer must leave it
		seq := e.seq(t)
		e.h.roller.queue(9, 9)
		again, err := e.send(t, key, true)
		if err != nil || again.holdID != held.holdID || !e.favorHolds(t) {
			t.Errorf("the retry = %+v, %v (Inspiration there %v), want the same question and nothing spent", again, err, e.favorHolds(t))
		}
		e.untouched(t, "the retry of a held roll", 2, seq)
		e.h.roller.drain()

		out, err := e.answerHeldRoll(t, e.roller, held.holdID, dieAnswer(false))
		if err != nil {
			t.Fatalf("AnswerHeldRoll() error = %v", err)
		}
		got := e.answered(out)
		if len(got.faces) != 2 || got.mode != "advantage" || !got.hasFavor() {
			t.Errorf("the answered roll = %+v, want the two dice kept with the advantage and the source Inspiração", got)
		}
		if !e.favorHolds(t) || !e.stored(t) {
			t.Errorf("after the answer the Inspiration is there = %v and the roll says it was spent = %v, want the new one untouched and the roll to say so", e.favorHolds(t), e.stored(t))
		}
		retry, err := e.send(t, key, true)
		if err != nil || len(retry.faces) != 2 || !e.favorHolds(t) {
			t.Errorf("the retry after the answer = %+v, %v (Inspiration there %v), want the written roll and nothing spent", retry, err, e.favorHolds(t))
		}
	})
}

// A retry that adds the spend to a held roll made without it comes after the d20: refused with AFTER_THE_ROLL, and nothing is spent.
func TestFavorAfterTheRollOnAHeldCheck(t *testing.T) {
	t.Parallel()
	forEachCheck(t, func(t *testing.T, e *checkEnv) {
		t.Helper()
		e.die(t)
		key := newKey()
		e.h.roller.queue(4)
		held, err := e.send(t, key, false)
		if err != nil || held.holdID == "" {
			t.Fatalf("%s with a die = %+v, %v, want the die's question", e.name, held, err)
		}
		e.give(t, e.who)
		seq := e.seq(t)
		_, err = e.send(t, key, true)
		wantFavorBlocked(t, e.name+" that adds the spend to a held roll", err, playv1.FavorBlockedReason_FAVOR_BLOCKED_REASON_AFTER_THE_ROLL)
		if !e.favorHolds(t) || e.seq(t) != seq {
			t.Errorf("Inspiration there = %v, number %d -> %d, want it kept and the number still", e.favorHolds(t), seq, e.seq(t))
		}
	})
}

// With typed dice the faces are as many as the mode the Inspiration makes: one face under a spend that makes advantage is refused (the
// existing dice-count error) and takes nothing, the pair is written.
func TestFavorTypedFacesOfACheck(t *testing.T) {
	t.Parallel()
	forEachCheck(t, func(t *testing.T, e *checkEnv) {
		t.Helper()
		e.give(t, e.who)
		_, err := e.typed(t, newKey(), true, 10)
		if c := connect.CodeOf(err); c != connect.CodeInvalidArgument && c != connect.CodeAborted {
			t.Fatalf("%s with one face under a spend error = %v, want the dice-count error", e.name, err)
		}
		if !e.favorHolds(t) {
			t.Fatal("a refused roll took the Inspiration")
		}
		got, err := e.typed(t, newKey(), true, 10, 15)
		if err != nil || len(got.faces) != 2 || got.mode != "advantage" || !got.hasFavor() || e.favorHolds(t) {
			t.Errorf("the pair under a spend = %+v, %v (Inspiration there %v), want the advantage written and the Inspiration spent", got, err, e.favorHolds(t))
		}
	})
}
