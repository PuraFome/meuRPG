package play

import (
	"context"
	"slices"
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

// holdsOfTheCampaign counts the NPC check holds of the table and the asks of the holds given.
func (n *npcTable) holdsOfTheCampaign(t *testing.T, holds ...string) (rows, asks int) {
	t.Helper()
	if err := n.h.pool.QueryRow(t.Context(), `SELECT count(*) FROM inspiration_holds WHERE campaign_id = $1 AND kind = 'npc_check'`, n.campaignID).Scan(&rows); err != nil {
		t.Fatalf("count the holds: %v", err)
	}
	if err := n.h.pool.QueryRow(t.Context(), `SELECT count(*) FROM npc_check_asks WHERE hold_id = ANY($1::UUID[])`, holds).Scan(&asks); err != nil {
		t.Fatalf("count the asks: %v", err)
	}
	return rows, asks
}

// TestNpcCheckSettledChecksStayUntilTheSessionEndsAndGoWithIt: a settled check keeps its hold and its asks while the session is open (the
// master reads it again with GetNpcCheck), whichever way it settled (the bards' answers, the skip, the next roll); the session that ends
// deletes every NPC check hold of the campaign, the settled ones too, and their asks go with them.
func TestNpcCheckSettledChecksStayUntilTheSessionEndsAndGoWithIt(t *testing.T) {
	t.Parallel()
	n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
	answered, err := n.roll(t, 15, true) // settled by the answers of both bards
	if err != nil || !answered.GetHeld() {
		t.Fatalf("the first RollNpcCheck() = %v, %v; want a held check", answered, err)
	}
	first, second := n.firstAsked(answered), n.bia
	if first == n.bia {
		second = n.ana
	}
	for _, u := range []*user{first, second} {
		if _, err := n.pass(u, answered.GetCheck().GetHoldId(), newKey()); err != nil {
			t.Fatalf("a bard's pass error = %v", err)
		}
	}
	skipped, err := n.roll(t, 14, true) // settled by the master's skip
	if err != nil || !skipped.GetHeld() {
		t.Fatalf("the second RollNpcCheck() = %v, %v; want a held check", skipped, err)
	}
	if _, err := n.skip(n.master, skipped.GetCheck().GetHoldId(), newKey()); err != nil {
		t.Fatalf("SkipNpcCheckAsks() error = %v", err)
	}
	next, err := n.roll(t, 13, true) // settled by the roll that follows it
	if err != nil || !next.GetHeld() {
		t.Fatalf("the third RollNpcCheck() = %v, %v; want a held check", next, err)
	}
	last, err := n.roll(t, 12, true)
	if err != nil || !last.GetHeld() {
		t.Fatalf("the fourth RollNpcCheck() = %v, %v; want a held check", last, err)
	}
	holds := []string{answered.GetCheck().GetHoldId(), skipped.GetCheck().GetHoldId(), next.GetCheck().GetHoldId()}
	for _, id := range holds {
		if row, ok := n.hold(t, id); !ok || !row.answered {
			t.Fatalf("the hold %s = %+v, %v before the session ends; want it kept, settled", id, row, ok)
		}
	}
	if rows, asks := n.holdsOfTheCampaign(t, holds...); rows != 4 || asks != 6 {
		t.Fatalf("before the end: %d holds and %d asks of the three settled checks, want 4 and 6", rows, asks)
	}
	n.master.end(t, n.session(t))
	if rows, asks := n.holdsOfTheCampaign(t, append(holds, last.GetCheck().GetHoldId())...); rows != 0 || asks != 0 {
		t.Errorf("after the end: %d holds and %d asks, want none", rows, asks)
	}
}

// TestNpcCheckAsksOnlyTheBardsOfThePlayersOfTheScene: with no token pair to measure, the master's box says the bards see the creature, and
// the bard's player must also be one the open scene is open to. A bard whose player is outside the scene has no ask, hears nothing of
// the check and reads no offer, whether the creature is on the stage or not; with the scene open to everyone both bards are asked.
// Brisa (Bia's) is asked before Lia (Ana's): the first asked hears the check and reads the offer.
func TestNpcCheckAsksOnlyTheBardsOfThePlayersOfTheScene(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		to    func(n *npcTable) []*user // nil: the scene is open to the whole table
		stage bool
		want  []string
	}{
		"open to everyone":              {nil, false, []string{"Brisa", "Lia"}},
		"open to Ana, off the stage":    {func(n *npcTable) []*user { return []*user{n.ana} }, false, []string{"Lia"}},
		"open to Ana, on the stage":     {func(n *npcTable) []*user { return []*user{n.ana} }, true, []string{"Lia"}},
		"open to Bia, off the stage":    {func(n *npcTable) []*user { return []*user{n.bia} }, false, []string{"Brisa"}},
		"open to Caio, off the stage":   {func(n *npcTable) []*user { return []*user{n.caio} }, false, nil},
		"open to Ana and Bia, on stage": {func(n *npcTable) []*user { return []*user{n.ana, n.bia} }, true, []string{"Brisa", "Lia"}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
			point, _ := n.h.newScene(n.h.newMap(n.campaignID, gridColumns), "A clareira", true, 1)
			if tc.to == nil {
				n.openScene(t, point)
			} else {
				n.openFor(t, point, tc.to(n)...)
			}
			if tc.stage {
				if _, err := n.master.play.PutOnStage(t.Context(), connect.NewRequest(&playv1.PutOnStageRequest{CampaignId: n.campaignID, CharacterId: n.npcID})); err != nil {
					t.Fatalf("PutOnStage() error = %v", err)
				}
			}
			bards := map[string]*user{"Brisa": n.bia, "Lia": n.ana}
			listeners := map[string]heardBy{}
			for name, u := range bards {
				listeners[name] = listen(t, n.h, n.campaignID, u.id, false)
			}
			res, err := n.roll(t, 15, true)
			if err != nil {
				t.Fatalf("RollNpcCheck() error = %v", err)
			}
			var asked []string
			for _, a := range res.GetCheck().GetAsks() {
				asked = append(asked, a.GetCharacterName())
			}
			if res.GetHeld() != (len(tc.want) > 0) || !slices.Equal(asked, tc.want) {
				t.Fatalf("the bards asked = %v (held %v), want %v", asked, res.GetHeld(), tc.want)
			}
			if res.GetHeld() {
				if rows := n.asksOf(t, res.GetCheck().GetHoldId()); rows != len(tc.want) {
					t.Errorf("the check has %d ask rows, want %d", rows, len(tc.want))
				}
			}
			for name, u := range bards {
				first := len(tc.want) > 0 && tc.want[0] == name // only the bard asked now hears the check and reads its offer
				heard := false
				for _, ev := range listeners[name].heard(t, n.h) {
					heard = heard || ev.GetHeldRollsChanged() != nil
				}
				if heard != first {
					t.Errorf("%s's player heard the held rolls change = %v, want %v", name, heard, first)
				}
				if got := n.listHeldRolls(t, u); len(got) != map[bool]int{true: 1}[first] {
					t.Errorf("%s's player reads %d offers, want %v", name, len(got), map[bool]int{true: 1}[first])
				}
			}
		})
	}
}

// asksOf counts the ask rows of a hold.
func (n *npcTable) asksOf(t *testing.T, hold string) int {
	t.Helper()
	var rows int
	if err := n.h.pool.QueryRow(t.Context(), `SELECT count(*) FROM npc_check_asks WHERE hold_id = $1`, hold).Scan(&rows); err != nil {
		t.Fatalf("count the asks: %v", err)
	}
	return rows
}
