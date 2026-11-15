package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The ask follows the character: when Toren changes hands while his ask waits, the new owner reads it and cancels it, and the old owner
// reads and cancels nothing of it, so neither sees the other's data through the pending ask.
func TestCancelOfAnAskFollowsTheCharacterToItsNewOwner(t *testing.T) {
	t.Parallel()
	f := newFavorFight(t)
	f.give(t, f.toren)
	pass := f.mustAsk(t, f.toren.GetId(), f.pens.GetId())
	// Bia gives up Brisa (she may own one living character) and takes Toren over from Caio.
	f.execSQL(t, `UPDATE characters SET reserved = true, player_user_id = NULL WHERE id = $1`, f.bri.GetId())
	f.execSQL(t, `UPDATE characters SET player_user_id = $2 WHERE id = $1`, f.toren.GetId(), f.bia.id)

	if got := f.passesOf(t, f.caio); len(got) != 0 {
		t.Errorf("the old owner reads %d asks, want none", len(got))
	}
	if got := f.passesOf(t, f.bia); len(got) != 1 || got[0].GetId() != pass.GetId() {
		t.Errorf("the new owner reads %v, want the ask", got)
	}
	_, err := f.cancelAs(f.caio, pass.GetId(), newKey())
	wantCode(t, "the old owner's cancel", err, connect.CodeNotFound)
	if _, pending := f.passRows(t); pending != 1 {
		t.Fatalf("pending asks = %d after the old owner's cancel, want 1", pending)
	}
	if _, err := f.cancelAs(f.bia, pass.GetId(), newKey()); err != nil {
		t.Fatalf("the new owner's cancel error = %v", err)
	}
	if status := f.passStatus(t, pass.GetId()); status != "canceled" {
		t.Errorf("the ask is %q, want canceled", status)
	}
}

// A closing key used in an earlier session of the campaign is refused as reused, not as an internal error: the retry check reads the open
// session's events, so the table's unique key on closed_key is what catches it.
func TestAnswerKeyUsedInAnotherSessionIsRefusedAsReused(t *testing.T) {
	t.Parallel()
	f := newFavorParty(t)
	if err := f.grant(f.toren.GetId()); err != nil {
		t.Fatalf("GrantFavor() error = %v", err)
	}
	key := newKey()
	first := f.mustAsk(t, f.toren.GetId(), f.pens.GetId())
	if _, err := f.answerAs(f.master, first.GetId(), false, key); err != nil {
		t.Fatalf("AnswerFavorPass(Recusar) error = %v", err)
	}
	f.master.end(t, &playv1.GameSession{CampaignId: f.campaignID, Id: f.sessionID})
	open := f.master.start(t, f.campaignID).GetGameSession()
	f.sessionID = open.GetId()
	second := f.mustAsk(t, f.toren.GetId(), f.pens.GetId())
	_, err := f.answerAs(f.master, second.GetId(), false, key)
	wantCode(t, "a key used by an answer of an earlier session", err, connect.CodeInvalidArgument)
	if status := f.passStatus(t, second.GetId()); status != "pending" {
		t.Errorf("the second ask is %q, want pending", status)
	}
}

// An answer's key used in an earlier session of the campaign is refused as reused, not as an internal error: the retry check reads the open
// session's events, so the table's unique key on answer_key is what catches it, whichever answer it is.
func TestRefundAnswerKeyUsedInAnotherSessionIsRefusedAsReused(t *testing.T) {
	t.Parallel()
	for _, giveBack := range []bool{true, false} {
		name := "keep"
		if giveBack {
			name = "give back"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f, first := refundFight(t)
			key := newKey()
			if _, err := f.answerRefund(f.master, first.GetId(), false, key); err != nil {
				t.Fatalf("AnswerFavorRefund(Manter gasta) error = %v", err)
			}
			f.master.end(t, &playv1.GameSession{CampaignId: f.campaignID, Id: f.sessionID})
			f.sessionID = f.master.start(t, f.campaignID).GetGameSession().GetId()
			var second string
			if err := f.h.pool.QueryRow(t.Context(), `INSERT INTO favor_refund_prompts (campaign_id, game_session_id, character_id, roll_label_pt)
				VALUES ($1, $2, $3, 'Ataque') RETURNING id`, f.campaignID, f.sessionID, f.toren.GetId()).Scan(&second); err != nil {
				t.Fatalf("open the second session's prompt: %v", err)
			}
			seq := f.seq(t)
			_, err := f.answerRefund(f.master, second, giveBack, key)
			wantCode(t, "a key used by an answer of an earlier session", err, connect.CodeInvalidArgument)
			if status := f.promptStatus(t, second); status != "open" {
				t.Errorf("the second prompt is %q, want open", status)
			}
			if f.rows(t) != 0 || f.seq(t) != seq {
				t.Errorf("rows %d, number %d (want %d), want the refusal to change nothing", f.rows(t), f.seq(t), seq)
			}
		})
	}
}
