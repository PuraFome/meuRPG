package play

import (
	"testing"
	"time"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The master's answer waits for the roller's player: for heldPlayerFirst after the question is asked (the hold's
// created_at) only the player may answer; the master is refused with PLAYER_FIRST and the seconds left, and after it
// the first answer wins. The tests move the clock, never wait.

// TestTheMastersAnswerWaitsForThePlayersFirstChance: the three rolls of the matrix, asked of the master at once, a few
// seconds later and when the time has passed; the player's own answer goes through at once.
func TestTheMastersAnswerWaitsForThePlayersFirstChance(t *testing.T) {
	t.Parallel()
	for _, kind := range heldRPCCases {
		t.Run(kind.name, func(t *testing.T) {
			t.Parallel()
			c := kind.build(t)
			assertWait := func(what string, want int32) {
				t.Helper()
				_, err := c.a.answerHeldRoll(t, c.a.master, c.hold, c.answer)
				b := wantResourceBlocked(t, err, playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_PLAYER_FIRST)
				if b.GetWaitSeconds() != want {
					t.Errorf("%s: the master waits %d s, want %d", what, b.GetWaitSeconds(), want)
				}
			}
			assertWait("at once", 10)
			c.a.h.clock.advance(4 * time.Second)
			assertWait("4 s later", 6)
			c.a.h.clock.advance(5*time.Second + 500*time.Millisecond)
			assertWait("9.5 s later", 1) // whole seconds, rounded up: never "0 s" while it still waits
			// The question still waits for the player, who answers whenever; the master after the time.
			if list := c.a.listHeldRolls(t, c.a.master); len(list) != 1 {
				t.Fatalf("the master lists %v while the player's chance lasts, want the question", list)
			}
			c.a.h.clock.advance(time.Second)
			res, err := c.a.answerHeldRoll(t, c.a.master, c.hold, c.answer)
			if err != nil {
				t.Fatalf("the master's answer after the time: error = %v", err)
			}
			c.settled(t, res)
		})
	}
}

// TestThePlayersAnswerNeverWaits: the roller's player answers at once, and so do the older RPCs for the player.
func TestThePlayersAnswerNeverWaits(t *testing.T) {
	t.Parallel()
	for _, kind := range heldRPCCases {
		t.Run(kind.name, func(t *testing.T) {
			t.Parallel()
			c := kind.build(t)
			res, err := c.a.answerHeldRoll(t, c.owner, c.hold, c.answer)
			if err != nil {
				t.Fatalf("the player's answer at once: error = %v", err)
			}
			c.settled(t, res)
		})
	}
}

// TestTheOlderRPCsWaitForThePlayerToo: the master cannot go round the wait by the RPCs AnswerHeldRoll replaced.
func TestTheOlderRPCsWaitForThePlayerToo(t *testing.T) {
	t.Parallel()
	t.Run("the die of a weapon attack", func(t *testing.T) {
		t.Parallel()
		a := newLuckyTable(t)
		e := a.luckyInspired(t)
		held, err := a.attack(t, a.caio, e, "Pipo", battleaxe, "Goblin", d20(14))
		if err != nil || held.GetInspirationOffer() == nil {
			t.Fatalf("Pipo's attack = %v, %v; want the die's question", held, err)
		}
		_, err = a.answer(t, a.master, e, held.GetInspirationOffer().GetHoldId(), keepTheDie)
		wantResourceBlocked(t, err, playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_PLAYER_FIRST)
		a.h.afterThePlayersChance()
		if _, err := a.answer(t, a.master, e, held.GetInspirationOffer().GetHoldId(), keepTheDie); err != nil {
			t.Errorf("the master's answer after the time: error = %v", err)
		}
	})
	t.Run("Lucky of a weapon attack", func(t *testing.T) {
		t.Parallel()
		a := newLuckyTable(t)
		e := a.luckyFight(t)
		held, err := a.attack(t, a.caio, e, "Pipo", battleaxe, "Goblin", d20(1))
		if err != nil || held.GetLuckyOffer() == nil {
			t.Fatalf("Pipo's attack = %v, %v; want Lucky's question", held, err)
		}
		_, err = a.answerLucky(t, a.master, e, held.GetLuckyOffer().GetHoldId(), keepTheOne)
		wantResourceBlocked(t, err, playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_PLAYER_FIRST)
	})
}

// TestTheMasterDoesNotWaitForARollNoPlayerRolls: a player who left the table (no user plays the combatant) leaves the
// question to the master at once: that is what his answer is for.
func TestTheMasterDoesNotWaitForARollNoPlayerRolls(t *testing.T) {
	t.Parallel()
	a := newLuckyTable(t)
	e := a.luckyInspired(t)
	held, err := a.attack(t, a.caio, e, "Pipo", battleaxe, "Goblin", d20(14))
	if err != nil || held.GetInspirationOffer() == nil {
		t.Fatalf("Pipo's attack = %v, %v; want the die's question", held, err)
	}
	if _, err := a.h.pool.Exec(t.Context(), `UPDATE combatants SET user_id = NULL WHERE id = $1`, a.id(t, "Pipo")); err != nil {
		t.Fatalf("the player leaves: %v", err)
	}
	if _, err := a.answer(t, a.master, e, held.GetInspirationOffer().GetHoldId(), keepTheDie); err != nil {
		t.Errorf("the master's answer at once to a roll no player plays: error = %v", err)
	}
}
