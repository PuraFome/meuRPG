package play

import (
	"context"
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
)

// The held NPC check after the first review: the keys of its answers, the errors of its answer, the rows a session leaves.

// TestNpcCheckLastAnswerUnderTheKeyOfAnotherEventIsAReusedKey: the last bard's answer settles the check under its own key, so a key that
// another event of the session already holds (the roll of an earlier check) is refused as every reused key is, and nothing is spent or
// settled; a cut under a key of its own then settles the check.
func TestNpcCheckLastAnswerUnderTheKeyOfAnotherEventIsAReusedKey(t *testing.T) {
	t.Parallel()
	n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
	earlier := newKey()
	a, err := n.rollWith(t, 15, true, earlier)
	if err != nil || !a.GetHeld() {
		t.Fatalf("the first RollNpcCheck() = %v, %v; want a held check", a, err)
	}
	if _, err := n.skip(n.master, a.GetCheck().GetHoldId(), newKey()); err != nil {
		t.Fatalf("SkipNpcCheckAsks() error = %v", err)
	}
	b, err := n.roll(t, 12, true)
	if err != nil || !b.GetHeld() {
		t.Fatalf("the second RollNpcCheck() = %v, %v; want a held check", b, err)
	}
	hold, first := b.GetCheck().GetHoldId(), n.firstAsked(b)
	last := n.ana
	if first == n.ana {
		last = n.bia
	}
	if _, err := n.pass(first, hold, newKey()); err != nil {
		t.Fatalf("the first bard's pass error = %v", err)
	}
	before := n.uses(t, n.charOf(last))
	for name, a := range map[string]*playv1.CuttingWordsAnswer{"a pass": {Cut: false}, "a cut": typedCut(3)} {
		_, err := n.answerAs(last, hold, earlier, a)
		wantCode(t, "the last bard answers with the key of an earlier roll, "+name, err, connect.CodeInvalidArgument)
	}
	if got := n.uses(t, n.charOf(last)); got != before {
		t.Errorf("uses %d -> %d after refused answers, want none spent", before, got)
	}
	if row, ok := n.hold(t, hold); !ok || row.answered {
		t.Errorf("the hold = %+v, %v after refused answers; want it still waiting", row, ok)
	}
	if _, err := n.answerAs(last, hold, newKey(), typedCut(3)); err != nil {
		t.Fatalf("the last bard's cut under a key of its own error = %v", err)
	}
	if row, ok := n.hold(t, hold); !ok || !row.answered {
		t.Errorf("the hold = %+v, %v after the last answer; want it settled", row, ok)
	}
}

// TestNpcCheckAnswerWrapsTheErrorOfItsTransaction: an error of the store that is not a refusal reads as the store's error of every
// path (the canceled request is told as canceled), not as the raw error of the transaction.
func TestNpcCheckAnswerWrapsTheErrorOfItsTransaction(t *testing.T) {
	t.Parallel()
	n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
	res, err := n.roll(t, 15, true)
	if err != nil {
		t.Fatalf("RollNpcCheck() error = %v", err)
	}
	first := n.firstAsked(res)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	m := authz.Membership{CampaignID: n.campaignID, UserID: first.id, Role: authz.RolePlayer}
	req := &playv1.AnswerHeldRollRequest{
		CampaignId: n.campaignID, HoldId: res.GetCheck().GetHoldId(), IdempotencyKey: newKey(),
		Answer: &playv1.AnswerHeldRollRequest_CuttingWords{CuttingWords: &playv1.CuttingWordsAnswer{Cut: false}},
	}
	_, err = n.h.svc.answerNpcCheck(ctx, m, req, req.GetIdempotencyKey(), req.GetHoldId())
	wantCode(t, "the answer of a canceled request", err, connect.CodeCanceled)
}

// TestNpcCheckRerunSettlesNothing: the rerun the registry holds for the kind refuses with an internal error and writes nothing: the check
// is still waiting, with no resolved event, after a path that reached it by mistake.
func TestNpcCheckRerunSettlesNothing(t *testing.T) {
	t.Parallel()
	n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
	res, err := n.roll(t, 15, true)
	if err != nil || !res.GetHeld() {
		t.Fatalf("RollNpcCheck() = %v, %v; want a held check", res, err)
	}
	hold := res.GetCheck().GetHoldId()
	def, ok := heldRegistry[heldNpcCheck]
	if !ok || def.rerun == nil {
		t.Fatal("the registry has no rerun for the NPC check")
	}
	out, err := def.rerun(t.Context(), n.h.svc, heldRoll{ID: hold, CampaignID: n.campaignID, Kind: heldNpcCheck}, newKey())
	wantCode(t, "the rerun of an NPC check", err, connect.CodeInternal)
	if out != nil {
		t.Errorf("the rerun answered %v, want nothing", out)
	}
	if row, ok := n.hold(t, hold); !ok || row.answered {
		t.Errorf("the hold after the rerun = %+v, %v; want it still waiting", row, ok)
	}
	if got := n.resolvedCount(t); got != 0 {
		t.Errorf("%d resolved events after the rerun, want none", got)
	}
}
