package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The request of a held weapon attack made again once the answer wrote the attack gives the written attack back: the answer
// wrote it from the request the hold kept (the gate pins the mode in it), and the retry is the same request. The same key
// for another request is still refused.
func TestAHeldAttackMadeAgainAfterItsAnswerGivesTheWrittenAttackAndAnotherRequestIsRefused(t *testing.T) {
	t.Parallel()
	a := newLuckyTable(t)
	e := a.luckyInspired(t)
	key := newKey()
	send := func(attackKey string) (*playv1.RollAttackResponse, error) {
		return a.attack(t, a.caio, e, "Pipo", attackKey, "Goblin", func(r *playv1.RollAttackRequest) { inAppRoll(r); r.IdempotencyKey = key })
	}
	a.h.roller.queue(14)
	held, err := send(battleaxe)
	if err != nil || held.GetHeldRollOffer() == nil {
		t.Fatalf("RollAttack() = %v, %v; want the die's question", held, err)
	}
	answer, err := a.answer(t, a.caio, e, held.GetHeldRollOffer().GetHoldId(), func(r *playv1.AnswerBardicInspirationRequest) { keepTheDie(r) })
	if err != nil || answer.GetAttack().GetRoll() == nil {
		t.Fatalf("AnswerBardicInspiration() = %v, %v; want the attack", answer, err)
	}
	again, err := send(battleaxe)
	if err != nil || again.GetRoll().GetD20().GetTotal() != answer.GetAttack().GetRoll().GetD20().GetTotal() {
		t.Errorf("the request again = %v, %v; want the attack the answer wrote", again.GetRoll().GetD20(), err)
	}
	if _, err := send(sword); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("another attack under the same key: error = %v, want invalid_argument", err)
	}
}
