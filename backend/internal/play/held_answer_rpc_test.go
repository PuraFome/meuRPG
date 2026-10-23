package play

import (
	"sync"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/dbtest"
)

// heldRPCCase is one roll that waits, ready to be answered by AnswerHeldRoll: who rolled it, who is another player and the
// hold (a Bardic Inspiration die's question, or Lucky's).
type heldRPCCase struct {
	a       *armed
	hold    string
	owner   *user
	other   *user
	answer  func(*playv1.AnswerHeldRollRequest)
	settled func(t *testing.T, res *playv1.AnswerHeldRollResponse)
}

// heldRPCCases are the rolls the matrix answers: a weapon attack in a combat, a spell attack in a combat (a roll out
// of the attack's path, run again as its roller) and a scene check out of one.
var heldRPCCases = []struct {
	name  string
	build func(t *testing.T) heldRPCCase
}{
	{"weapon attack", func(t *testing.T) heldRPCCase {
		t.Helper()
		a := newLuckyTable(t)
		e := a.luckyInspired(t)
		held, err := a.attack(t, a.caio, e, "Pipo", battleaxe, "Goblin", d20(14))
		if err != nil || held.GetInspirationOffer() == nil {
			t.Fatalf("Pipo's attack = %v, %v; want the die's question", held, err)
		}
		return heldRPCCase{
			a: a, hold: held.GetInspirationOffer().GetHoldId(), owner: a.caio, other: a.ana, answer: dieAnswer(false),
			settled: func(t *testing.T, res *playv1.AnswerHeldRollResponse) {
				t.Helper()
				if r := res.GetAttack().GetRoll(); r.GetD20().GetFaces()[0] != 14 || res.GetNext() != nil {
					t.Errorf("the settled attack = %v, want it on the d20 of 14", r)
				}
			},
		}
	}},
	{"spell attack", func(t *testing.T) heldRPCCase {
		t.Helper()
		a, e := brisaFight(t, true)
		a.h.roller.queue(9, 13)
		held, err := a.castGuidingBolt(t, e)
		if err != nil || held.GetInspirationOffer() == nil {
			t.Fatalf("Brisa's Guiding Bolt = %v, %v; want the die's question", held, err)
		}
		return heldRPCCase{
			a: a, hold: held.GetInspirationOffer().GetHoldId(), owner: a.bia, other: a.ana, answer: dieAnswer(false),
			settled: func(t *testing.T, res *playv1.AnswerHeldRollResponse) {
				t.Helper()
				if got := res.GetSpellCast().GetCast().GetTargets()[0].GetAttackRoll().GetFaces()[0]; got != 9 || res.GetNext() != nil {
					t.Errorf("the settled cast's d20 = %d, want 9", got)
				}
			},
		}
	}},
	{"scene check", func(t *testing.T) heldRPCCase {
		t.Helper()
		a := newResourceTable(t)
		dalila, dalilaID := joinDalila(t, a.h, a.master, a.campaignID)
		point, actions := a.h.newScene(a.mapID, "O muro do pátio", true, 3, sceneSpec{key: "skill:athletics", dc: new(int32(25))})
		a.openScene(t, point)
		if _, err := a.giveOutside(t, a.bia, dalilaID); err != nil {
			t.Fatalf("GiveBardicInspirationOutside() error = %v", err)
		}
		held, err := a.rollSceneTyped(t, dalila, actions[0], newKey(), 14)
		if err != nil || held.GetInspirationOffer() == nil {
			t.Fatalf("RollSceneCheck() = %v, %v; want the die's question", held, err)
		}
		return heldRPCCase{
			a: a, hold: held.GetInspirationOffer().GetHoldId(), owner: dalila, other: a.bia, answer: dieAnswer(false),
			settled: func(t *testing.T, res *playv1.AnswerHeldRollResponse) {
				t.Helper()
				if r := res.GetSceneCheck().GetRoll().GetRoll(); r.GetFaces()[0] != 14 || res.GetNext() != nil {
					t.Errorf("the settled check = %v, want it on the d20 of 14", r)
				}
			},
		}
	}},
}

// TestAnswerHeldRollIsAnsweredByTheOwnerAndTheMasterAndNoOneElse: for each roll that waits, its player and the master settle
// it with the die kept (and the path runs as the roller), another player is told the hold is not found and someone signed
// out is turned away.
func TestAnswerHeldRollIsAnsweredByTheOwnerAndTheMasterAndNoOneElse(t *testing.T) {
	t.Parallel()
	for _, kind := range heldRPCCases {
		for _, who := range []string{"owner", "master", "another player", "signed out"} {
			t.Run(kind.name+"/"+who, func(t *testing.T) {
				t.Parallel()
				c := kind.build(t)
				switch who {
				case "another player":
					_, err := c.a.answerHeldRoll(t, c.other, c.hold, c.answer)
					wantCode(t, "another player's answer", err, connect.CodeNotFound)
					if list := c.a.listHeldRolls(t, c.other); len(list) != 0 {
						t.Errorf("another player lists %v, want nothing", list)
					}
					return
				case "signed out":
					_, err := c.a.answerHeldRoll(t, c.a.h.anonymous(), c.hold, c.answer)
					wantCode(t, "a signed-out answer", err, connect.CodeUnauthenticated)
					return
				}
				u := c.owner
				if who == "master" {
					u = c.a.master
					c.a.h.afterThePlayersChance()
				}
				res, err := c.a.answerHeldRoll(t, u, c.hold, c.answer)
				if err != nil {
					t.Fatalf("the %s's answer: error = %v", who, err)
				}
				c.settled(t, res)
				// The roll is written once: an answer under another key is late, for the player and for the master.
				for _, late := range []*user{c.owner, c.a.master} {
					if _, err := c.a.answerHeldRoll(t, late, c.hold, c.answer); connect.CodeOf(err) != connect.CodeFailedPrecondition {
						t.Errorf("a late answer = %v, want failed_precondition", err)
					}
				}
				if list := c.a.listHeldRolls(t, c.a.master); len(list) != 0 {
					t.Errorf("the master lists %v after the answer, want nothing", list)
				}
			})
		}
	}
}

// TestTheOwnerAndTheMasterAnsweringAtOnceSettleItOnce: the first answer wins, the other is told the roll was answered
// already, and the die is spent once.
func TestTheOwnerAndTheMasterAnsweringAtOnceSettleItOnce(t *testing.T) {
	t.Parallel()
	dbtest.PoolSize(t, 2)
	a := newLuckyTable(t)
	e := a.luckyInspired(t)
	held, err := a.attack(t, a.caio, e, "Pipo", battleaxe, "Goblin", d20(14))
	if err != nil || held.GetInspirationOffer() == nil {
		t.Fatalf("Pipo's attack = %v, %v; want the die's question", held, err)
	}
	hold := held.GetInspirationOffer().GetHoldId()
	a.h.afterThePlayersChance() // the master's chance has come: both may answer
	users := []*user{a.caio, a.master}
	errs := make([]error, len(users))
	var wg sync.WaitGroup
	start := dbtest.NewBarrier(len(users))
	for i, u := range users {
		wg.Go(func() {
			start.Wait()
			_, errs[i] = a.answerHeldRoll(t, u, hold, dieAnswer(true))
		})
	}
	wg.Wait()
	var won, lost int
	for _, err := range errs {
		switch {
		case err == nil:
			won++
		case connect.CodeOf(err) == connect.CodeFailedPrecondition:
			lost++
		default:
			t.Errorf("an answer error = %v, want none or failed_precondition", err)
		}
	}
	if won != 1 || lost != 1 {
		t.Errorf("answers: %d won and %d lost, want one each (%v)", won, lost, errs)
	}
	var sides *int32
	if err := a.h.pool.QueryRow(t.Context(), `SELECT inspiration_sides FROM combatants WHERE id = $1`, a.id(t, "Pipo")).Scan(&sides); err != nil || sides != nil {
		t.Errorf("Pipo's die = %v (%v), want it spent once and gone", sides, err)
	}
}
