package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// TestAnswerReactionRefusesInTheOrderItAlwaysDid: the bad answer, the bad key, the bad ids and
// a window that is not the caller's, each with its code, before anything is written.
func TestAnswerReactionRefusesInTheOrderItAlwaysDid(t *testing.T) {
	t.Parallel()
	r, re := reactorFight(t, rogueFive)
	hit := r.mustAttack(t, r.master, re, "Goblin", sword, "Reator", d20(15))
	r.mustDamage(t, r.master, re, hit.GetPendingDamage().GetId(), typedDamage(6))
	w := r.windowOf(t, r.ana, playv1.ReactionKind_REACTION_KIND_UNCANNY_DODGE)
	if w == nil {
		t.Fatal("the rogue has no Uncanny Dodge window")
	}
	fake := "00000000-0000-4000-8000-000000000001"
	cases := []struct {
		name string
		u    *user
		set  func(*playv1.AnswerReactionRequest)
		want connect.Code
	}{
		{"no answer", r.ana, func(q *playv1.AnswerReactionRequest) { q.Answer = playv1.ReactionChoice_REACTION_CHOICE_UNSPECIFIED }, connect.CodeInvalidArgument},
		{"a bad key before a bad answer", r.ana, func(q *playv1.AnswerReactionRequest) { q.IdempotencyKey = "nope" }, connect.CodeInvalidArgument},
		{"a bad encounter id", r.ana, func(q *playv1.AnswerReactionRequest) { q.EncounterId = "nope"; passAnswer(q) }, connect.CodeNotFound},
		{"a bad window id", r.ana, func(q *playv1.AnswerReactionRequest) { q.WindowId = "nope"; passAnswer(q) }, connect.CodeNotFound},
		{"a bad window id before a bad answer", r.ana, func(q *playv1.AnswerReactionRequest) { q.WindowId = "nope" }, connect.CodeNotFound},
		{"an invented window", r.ana, func(q *playv1.AnswerReactionRequest) { q.WindowId = fake; passAnswer(q) }, connect.CodePermissionDenied},
		{"another player's window", r.caio, passAnswer, connect.CodePermissionDenied},
	}
	for _, c := range cases {
		_, err := r.answerReaction(t, c.u, re, w.GetId(), c.set)
		wantCode(t, c.name, err, c.want)
	}
	if got := r.windowOf(t, r.ana, playv1.ReactionKind_REACTION_KIND_UNCANNY_DODGE); got == nil {
		t.Error("a refused answer closed the window")
	}
}
