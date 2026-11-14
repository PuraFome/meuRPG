package play

import (
	"sync"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The refund prompt (favor_refund.go): the spend that added something over what the roller reads but nothing over every source. Toren
// attacks with a hidden advantage on him. Written under the rule that no database test runs in this task: these have not run.

// refundFight is a fight where Toren's Inspiração was spent on an attack that a hidden advantage made no better: the prompt is open.
func refundFight(t *testing.T) (*favorFight, *playv1.FavorRefundPrompt) {
	t.Helper()
	f := newFavorFight(t)
	f.give(t, f.toren)
	f.hiddenEffect(t, "Toren", false, "["+attackAdv+"]")
	f.h.roller.queue(4, 17)
	if _, err := favorPaths[0].send(t, f, f.caio, newKey(), true, nil); err != nil {
		t.Fatalf("the attack with the Inspiração error = %v", err)
	}
	prompts := f.prompts(t, f.master)
	if len(prompts) != 1 {
		t.Fatalf("the master reads %d refund prompts, want 1", len(prompts))
	}
	return f, prompts[0]
}

func (f *favorFight) prompts(t *testing.T, u *user) []*playv1.FavorRefundPrompt {
	t.Helper()
	res, err := u.favor.GetFavor(t.Context(), connect.NewRequest(&playv1.GetFavorRequest{CampaignId: f.campaignID}))
	if err != nil {
		t.Fatalf("GetFavor() error = %v", err)
	}
	return res.Msg.GetRefundPrompts()
}

func (f *favorFight) answerRefund(u *user, id string, giveBack bool, key string) (*playv1.FavorRefundPrompt, error) {
	res, err := u.favor.AnswerFavorRefund(f.h.t.Context(), connect.NewRequest(&playv1.AnswerFavorRefundRequest{CampaignId: f.campaignID, PromptId: id, GiveBack: giveBack, IdempotencyKey: key}))
	if err != nil {
		return nil, err
	}
	return res.Msg.GetPrompt(), nil
}

// The prompt is the master's alone, names the roll and the hidden source, and the roll resolved at once with the Inspiração spent.
func TestRefundPromptOpensForTheMasterAlone(t *testing.T) {
	t.Parallel()
	f, p := refundFight(t)
	if p.GetCharacterId() != f.toren.GetId() || p.GetCharacterName() != "Toren" || p.GetRollLabelPt() != "Ataque" || len(p.GetHiddenSources()) != 1 ||
		p.GetStatus() != playv1.FavorRefundStatus_FAVOR_REFUND_STATUS_OPEN {
		t.Errorf("prompt = %v, want Toren's open attack with one hidden source", p)
	}
	if f.favorHolds(t) || f.eventsOf(t, eventFavorRefundPrompted) != 1 {
		t.Errorf("the Inspiração is there %v, prompted events %d, want it spent and one event", f.favorHolds(t), f.eventsOf(t, eventFavorRefundPrompted))
	}
	for name, u := range map[string]*user{"the roller's player": f.caio, "another player": f.ana, "a third player": f.bia} {
		if got := f.prompts(t, u); len(got) != 0 {
			t.Errorf("%s reads %v, want no refund prompt", name, got)
		}
	}
	if _, err := f.answerRefund(f.caio, p.GetId(), true, newKey()); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("a player's answer = %v, want permission_denied", err)
	}
}

// A spend that adds visibly and over every source opens no prompt.
func TestRefundPromptNeverOpensForASpendThatHelped(t *testing.T) {
	t.Parallel()
	f := newFavorFight(t)
	f.give(t, f.toren)
	f.h.roller.queue(4, 17)
	if _, err := favorPaths[0].send(t, f, f.caio, newKey(), true, nil); err != nil {
		t.Fatal(err)
	}
	if got := f.prompts(t, f.master); len(got) != 0 {
		t.Errorf("the master reads %v, want no prompt", got)
	}
}

// Giving back returns the Inspiração through the one refund path, closes the prompt RETURNED, tells the master and the owner and writes
// the combat's line: the player reads only that the master gave it back.
func TestAnswerFavorRefundGiveBack(t *testing.T) {
	t.Parallel()
	f, p := refundFight(t)
	before, key := f.seq(t), newKey()
	got, err := f.answerRefund(f.master, p.GetId(), true, key)
	if err != nil || got.GetStatus() != playv1.FavorRefundStatus_FAVOR_REFUND_STATUS_RETURNED {
		t.Fatalf("AnswerFavorRefund(give back) = %v, %v, want RETURNED", got, err)
	}
	if !f.favorHolds(t) || f.seq(t) != before+1 || f.eventsOf(t, eventFavorRefunded) != 1 {
		t.Errorf("row %v, number %d (want %d), refunded events %d, want it back in one step", f.favorHolds(t), f.seq(t), before+1, f.eventsOf(t, eventFavorRefunded))
	}
	f.wantTexts(t, "the refund", map[*user][]string{f.master: {"O mestre devolveu a Inspiração de Toren."}, f.caio: {"O mestre devolveu a sua Inspiração."}})
	if again, err := f.answerRefund(f.master, p.GetId(), true, key); err != nil || again.GetStatus() != got.GetStatus() || f.seq(t) != before+1 || f.rows(t) != 1 {
		t.Errorf("the retry = %v, %v (number %d), want the same answer and no new step", again, err, f.seq(t))
	}
	if _, err := f.answerRefund(f.master, p.GetId(), false, newKey()); !isBlockedBy(err, blockedPassClosed) {
		t.Errorf("a new answer to a closed prompt = %v, want PASS_CLOSED", err)
	}
	if got := f.prompts(t, f.master); len(got) != 0 {
		t.Errorf("the master still reads the answered prompt: %v", got)
	}
}

// A character that holds another Inspiração by then keeps the one row.
func TestAnswerFavorRefundNeverStacks(t *testing.T) {
	t.Parallel()
	f, p := refundFight(t)
	f.give(t, f.toren)
	if _, err := f.answerRefund(f.master, p.GetId(), true, newKey()); err != nil {
		t.Fatal(err)
	}
	if f.rows(t) != 1 {
		t.Errorf("character_favor has %d rows, want 1", f.rows(t))
	}
}

// Keeping it leaves it spent, tells the master alone and writes nothing the player reads.
func TestAnswerFavorRefundKeep(t *testing.T) {
	t.Parallel()
	f, p := refundFight(t)
	got, err := f.answerRefund(f.master, p.GetId(), false, newKey())
	if err != nil || got.GetStatus() != playv1.FavorRefundStatus_FAVOR_REFUND_STATUS_KEPT {
		t.Fatalf("AnswerFavorRefund(keep) = %v, %v, want KEPT", got, err)
	}
	if f.favorHolds(t) || f.eventsOf(t, eventFavorRefundKept) != 1 {
		t.Errorf("row %v, kept events %d, want it spent and one event", f.favorHolds(t), f.eventsOf(t, eventFavorRefundKept))
	}
	f.wantTexts(t, "the kept prompt", nil)
}

// An unanswered prompt keeps it spent and closes KEPT with the session.
func TestTheEndOfASessionKeepsOpenRefundPrompts(t *testing.T) {
	t.Parallel()
	f, p := refundFight(t)
	if _, err := f.master.play.EndGameSession(t.Context(), connect.NewRequest(&playv1.EndGameSessionRequest{CampaignId: f.campaignID, GameSessionId: f.sessionID})); err != nil {
		t.Fatalf("EndGameSession() error = %v", err)
	}
	var status string
	if err := f.h.pool.QueryRow(t.Context(), `SELECT status FROM favor_refund_prompts WHERE id = $1`, p.GetId()).Scan(&status); err != nil || status != "kept" {
		t.Errorf("status = %q (%v), want kept", status, err)
	}
	if f.eventsOf(t, eventFavorRefundKept) != 1 || f.favorHolds(t) {
		t.Errorf("kept events %d, row %v, want one event and the Inspiração still spent", f.eventsOf(t, eventFavorRefundKept), f.favorHolds(t))
	}
}

// Two answers at once: one wins, the other reads PASS_CLOSED, and the Inspiração is back at most once.
func TestAnswerFavorRefundRace(t *testing.T) {
	t.Parallel()
	f, p := refundFight(t)
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = f.answerRefund(f.master, p.GetId(), i == 0, newKey())
		}()
	}
	wg.Wait()
	wins := 0
	for _, err := range errs {
		if err == nil {
			wins++
		} else if !isBlockedBy(err, blockedPassClosed) {
			t.Errorf("the losing answer = %v, want PASS_CLOSED", err)
		}
	}
	if wins != 1 || f.rows(t) > 1 {
		t.Errorf("%d answers won, %d rows, want one winner and at most one row", wins, f.rows(t))
	}
}

// A refund answered with the roller hidden is the master's line alone, even after the master shows the roller (RN-10).
func TestRefundLineOfAHiddenRollerIsTheMastersAlone(t *testing.T) {
	t.Parallel()
	f, p := refundFight(t)
	hideAgain(t, f.armed, f.e, "Toren")
	if _, err := f.answerRefund(f.master, p.GetId(), true, newKey()); err != nil {
		t.Fatal(err)
	}
	want := map[*user][]string{f.master: {"O mestre devolveu a Inspiração de Toren."}}
	f.wantTexts(t, "the refund with Toren hidden", want)
	showNPC(t, f.armed, f.e, "Toren")
	f.wantTexts(t, "the refund after Toren is shown", want)
}
