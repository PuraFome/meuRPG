package play

import (
	"regexp"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The master reads the latest NPC check of the open session again (GetNpcCheck): the card of a settled check does not vanish with the
// screen, and it holds what the master read when it settled.

// latestCheck is what GetNpcCheck answers to the caller.
func (n *npcTable) latestCheck(t *testing.T, u *user) (*playv1.NpcCheckRead, error) {
	t.Helper()
	res, err := u.play.GetNpcCheck(t.Context(), connect.NewRequest(&playv1.GetNpcCheckRequest{CampaignId: n.campaignID}))
	if err != nil {
		return nil, err
	}
	return res.Msg.GetCheck(), nil
}

func (n *npcTable) mustLatest(t *testing.T) *playv1.NpcCheckRead {
	t.Helper()
	got, err := n.latestCheck(t, n.master)
	if err != nil {
		t.Fatalf("GetNpcCheck() error = %v", err)
	}
	return got
}

// TestGetNpcCheckReadsTheLatestCheck: none before the first check; the check that waits, as it stands; the settled one with the
// total and what each bard did; and then the next check, as soon as it is rolled.
func TestGetNpcCheckReadsTheLatestCheck(t *testing.T) {
	t.Parallel()
	n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
	if got := n.mustLatest(t); got != nil {
		t.Fatalf("GetNpcCheck() before any check = %v, want none", got)
	}
	res, err := n.roll(t, 15, true)
	if err != nil {
		t.Fatalf("RollNpcCheck() error = %v", err)
	}
	held := n.mustLatest(t)
	if !proto.Equal(held, res.GetCheck()) || held.GetSettled() || len(held.GetAsks()) != 2 {
		t.Fatalf("GetNpcCheck() while it waits = %v, want the roll's own read, not settled: %v", held, res.GetCheck())
	}
	hold, first := res.GetCheck().GetHoldId(), n.firstAsked(res)
	second := n.ana
	if first == n.ana {
		second = n.bia
	}
	if _, err := n.answer(t, first, hold, true, 4); err != nil {
		t.Fatalf("the first answer error = %v", err)
	}
	if mid := n.mustLatest(t); mid.GetSettled() || mid.GetAsks()[0].GetState() != playv1.NpcCheckAskState_NPC_CHECK_ASK_STATE_CUT || mid.GetAsks()[0].GetFace() != 4 {
		t.Errorf("GetNpcCheck() after the first answer = %v, want the first ask cut with the 4, not settled", mid)
	}
	if _, err := n.answer(t, second, hold, false, 0); err != nil {
		t.Fatalf("the last answer error = %v", err)
	}
	settled := n.mustLatest(t)
	if !settled.GetSettled() || settled.GetTotal() != 11 || settled.GetCut() != 4 || settled.GetCreatureLabel() != "Barão" || settled.GetCheckKey() != deception ||
		settled.GetD20().GetFaces()[0] != 15 || settled.GetHoldId() != hold {
		t.Fatalf("GetNpcCheck() once settled = %v, want the Barão's deception, 15 less 4", settled)
	}
	states := []playv1.NpcCheckAskState{settled.GetAsks()[0].GetState(), settled.GetAsks()[1].GetState()}
	if states[0] != playv1.NpcCheckAskState_NPC_CHECK_ASK_STATE_CUT || states[1] != playv1.NpcCheckAskState_NPC_CHECK_ASK_STATE_PASSED {
		t.Errorf("the asks = %v, want the bard asked first cut and the other let it pass", states)
	}
	if again := n.mustLatest(t); !proto.Equal(again, settled) {
		t.Errorf("GetNpcCheck() again = %v, want the same: reading changes nothing", again)
	}
	next, err := n.roll(t, 9, true)
	if err != nil {
		t.Fatalf("the next RollNpcCheck() error = %v", err)
	}
	if got := n.mustLatest(t); got.GetHoldId() != next.GetCheck().GetHoldId() || got.GetSettled() || got.GetD20().GetFaces()[0] != 9 {
		t.Errorf("GetNpcCheck() after the next roll = %v, want the next check, waiting", got)
	}
}

// TestGetNpcCheckReadsWhoWasSkippedAndTheCheckSettledAtOnce: a bard the master did not wait for reads as skipped, and a check nobody was
// asked about reads settled, with no hold.
func TestGetNpcCheckReadsWhoWasSkippedAndTheCheckSettledAtOnce(t *testing.T) {
	t.Parallel()
	n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
	alone, err := n.roll(t, 12, false)
	if err != nil || alone.GetHeld() {
		t.Fatalf("RollNpcCheck() = %v, %v; want a check settled at once", alone, err)
	}
	if got := n.mustLatest(t); !got.GetSettled() || got.GetTotal() != 12 || got.GetHoldId() != "" || len(got.GetAsks()) != 0 {
		t.Errorf("GetNpcCheck() of a check settled at once = %v, want a settled 12 with no hold and no asks", got)
	}
	res, err := n.roll(t, 15, true)
	if err != nil {
		t.Fatalf("RollNpcCheck() error = %v", err)
	}
	skipped, err := n.skip(n.master, res.GetCheck().GetHoldId(), newKey())
	if err != nil {
		t.Fatalf("SkipNpcCheckAsks() error = %v", err)
	}
	got := n.mustLatest(t)
	if !proto.Equal(got, skipped) || !got.GetSettled() || got.GetTotal() != 15 {
		t.Fatalf("GetNpcCheck() after the skip = %v, want the skip's own read, a settled 15", got)
	}
	for _, a := range got.GetAsks() {
		if a.GetState() != playv1.NpcCheckAskState_NPC_CHECK_ASK_STATE_CLOSED {
			t.Errorf("the ask of %s = %v, want closed (not waited for)", a.GetCharacterName(), a.GetState())
		}
	}
}

// TestGetNpcCheckEndsWithTheSession: the check of a session is not read in the next one, and with no session open the read is refused
// with the reason; a player is refused whatever was rolled, and the refusal says nothing of the check.
func TestGetNpcCheckEndsWithTheSession(t *testing.T) {
	t.Parallel()
	n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
	if _, err := n.roll(t, 17, true); err != nil {
		t.Fatalf("RollNpcCheck() error = %v", err)
	}
	for _, u := range []*user{n.bia, n.ana, n.caio} {
		_, err := n.latestCheck(t, u)
		wantCode(t, "a player reads the check", err, connect.CodePermissionDenied)
		if msg := err.Error(); strings.Contains(msg, "Barão") || strings.Contains(msg, "17") || strings.Contains(msg, deception) {
			t.Errorf("the refusal of a player = %q, want no word of the check", msg)
		}
	}
	n.master.end(t, n.session(t))
	_, err := n.latestCheck(t, n.master)
	wantCode(t, "read with no session open", err, connect.CodeFailedPrecondition)
	n.master.start(t, n.campaignID)
	if got := n.mustLatest(t); got != nil {
		t.Errorf("GetNpcCheck() in the next session = %v, want none: the check was the earlier session's", got)
	}
}

// TestNpcCheckRefusalsGiveGenderToNobody: what the server says when it refuses a roll, a skip, a read or an answer of an NPC's check
// names no player and no character as "he" or "she" ("ele", "ela"), and none of the words holds a number or a name of the check.
func TestNpcCheckRefusalsGiveGenderToNobody(t *testing.T) {
	t.Parallel()
	n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
	res, err := n.roll(t, 17, true)
	if err != nil {
		t.Fatalf("RollNpcCheck() error = %v", err)
	}
	hold, first := res.GetCheck().GetHoldId(), n.firstAsked(res)
	second := n.ana
	if first == n.ana {
		second = n.bia
	}
	refusals := map[string]error{}
	_, refusals["a player rolls"] = n.bia.play.RollNpcCheck(t.Context(), connect.NewRequest(&playv1.RollNpcCheckRequest{CampaignId: n.campaignID, IdempotencyKey: newKey()}))
	_, refusals["a player skips"] = n.skip(n.bia, hold, newKey())
	_, refusals["a player reads the check"] = n.latestCheck(t, n.caio)
	_, refusals["a player lists the picker"] = n.bia.play.ListNpcCheckPicker(t.Context(), connect.NewRequest(&playv1.ListNpcCheckPickerRequest{CampaignId: n.campaignID}))
	_, refusals["a player who is no bard answers"] = n.answer(t, n.caio, hold, false, 0)
	_, refusals["a bard answers out of turn"] = n.answer(t, second, hold, false, 0)
	_, refusals["a bard cuts with no face"] = n.answerAs(first, hold, newKey(), &playv1.CuttingWordsAnswer{Cut: true})
	key := newKey()
	if _, err := n.answerAs(first, hold, key, typedCut(4)); err != nil {
		t.Fatalf("the cut error = %v", err)
	}
	_, refusals["the key of a cut is used for a pass"] = n.answerAs(first, hold, key, &playv1.CuttingWordsAnswer{Cut: false})
	_, refusals["a bard answers twice"] = n.answer(t, first, hold, false, 0)
	if _, err := n.skip(n.master, hold, newKey()); err != nil {
		t.Fatalf("the skip error = %v", err)
	}
	_, refusals["a bard answers after the skip"] = n.answer(t, second, hold, false, 0)
	n.master.end(t, n.session(t))
	_, refusals["the master reads with no session"] = n.latestCheck(t, n.master)
	words := regexp.MustCompile(`(?i)\b(he|she|his|her|hers|him|ele|ela|dele|dela)\b`)
	for what, err := range refusals {
		if err == nil {
			t.Errorf("%s: no refusal", what)
		} else if msg := err.Error(); words.MatchString(msg) || strings.Contains(msg, "Barão") || strings.Contains(msg, hold) {
			t.Errorf("%s: %q, want neutral words and nothing of the check", what, msg)
		}
	}
}
