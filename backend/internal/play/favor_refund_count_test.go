package play

import (
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The counting rules of the refund prompts: a roll's prompt closes for its own roller alone, and an Inspiração a roll spent is
// given back once.

// heldRefundFight is Toren's attack made with the Inspiração and a Bardic Inspiration die over a hidden advantage: the roll waits for the
// die's question, and the master reads the prompt of that spend.
func heldRefundFight(t *testing.T) (f *favorFight, holdID string, prompt *playv1.FavorRefundPrompt) {
	t.Helper()
	f = newFavorFight(t)
	f.give(t, f.toren)
	f.giveCombatantDie(t, "Toren", f.e)
	f.hiddenEffect(t, "Toren", false, "["+attackAdv+"]")
	f.h.roller.queue(4, 15)
	held, err := favorPaths[0].send(t, f, f.caio, newKey(), true, nil)
	if err != nil || held.holdID == "" {
		t.Fatalf("the attack with the Inspiração and a die = %+v, %v, want the die's question", held, err)
	}
	prompts := f.prompts(t, f.master)
	if len(prompts) != 1 {
		t.Fatalf("the master reads %d prompts of the held roll, want 1", len(prompts))
	}
	return f, held.holdID, prompts[0]
}

// dropTheHold turns the round and makes the next roll of Toren, which forgets the stale hold.
func (f *favorFight) dropTheHold(t *testing.T, holdID string) {
	t.Helper()
	if _, err := f.endTurn(t, f.master, f.e, true); err != nil {
		t.Fatalf("EndTurn() error = %v", err)
	}
	f.e = f.passTo(t, f.e, "Toren")
	if !f.holdExists(t, holdID) {
		t.Fatalf("the hold %s went before the next roll: the test does not reach the gate", holdID)
	}
	f.h.roller.queue(7)
	if _, err := favorPaths[0].send(t, f, f.caio, newKey(), false, nil); err != nil {
		t.Fatalf("the next attack error = %v", err)
	}
}

// otherPromptOfTheRoll opens, by SQL, the prompt another member of the same group check has for the roll of the prompt.
func (f *favorFight) otherPromptOfTheRoll(t *testing.T, of *playv1.FavorRefundPrompt, characterID string) string {
	t.Helper()
	var id string
	if err := f.h.pool.QueryRow(t.Context(), `INSERT INTO favor_refund_prompts (campaign_id, game_session_id, character_id, roll_label_pt, roll_key)
		SELECT campaign_id, game_session_id, $2, roll_label_pt, roll_key FROM favor_refund_prompts WHERE id = $1 RETURNING id`, of.GetId(), characterID).Scan(&id); err != nil {
		t.Fatalf("open the other member's prompt: %v", err)
	}
	return id
}

// The held roll of a group check is forgotten: Toren's prompt closes, and Pensantus's, for the same roll key, stays open.
func TestForgettingARollKeepsOnlyItsRollersPrompt(t *testing.T) {
	t.Parallel()
	f, holdID, p := heldRefundFight(t)
	other := f.otherPromptOfTheRoll(t, p, f.pens.GetId())
	f.dropTheHold(t, holdID)
	if status := f.promptStatus(t, p.GetId()); status != "kept" {
		t.Errorf("the roller's prompt is %q, want kept", status)
	}
	if status := f.promptStatus(t, other); status != "open" {
		t.Errorf("the other member's prompt is %q, want open: it is another character's spend", status)
	}
}

// The undo of the roll closes the prompt of the character that rolled it, and the other member's stays open.
func TestUndoOfARollKeepsOnlyItsRollersPrompt(t *testing.T) {
	t.Parallel()
	f, p := refundFight(t)
	other := f.otherPromptOfTheRoll(t, p, f.pens.GetId())
	if err := f.undo(t, f.master, f.e, f.undoableLine(t).GetId()); err != nil {
		t.Fatalf("UndoLastAction(the roll) error = %v", err)
	}
	if status := f.promptStatus(t, p.GetId()); status != "kept" {
		t.Errorf("the roller's prompt is %q, want kept", status)
	}
	if status := f.promptStatus(t, other); status != "open" {
		t.Errorf("the other member's prompt is %q, want open: it is another character's spend", status)
	}
}

// "Devolver" gave the Inspiração back, and Toren passed it away: the hold of that roll dies after, and gives nothing back, so the table
// has the one Inspiração it had.
func TestReturnedRefundIsNotGivenBackAgainWhenTheHoldDiesAfterAPass(t *testing.T) {
	t.Parallel()
	f, holdID, p := heldRefundFight(t)
	if _, err := f.answerRefund(f.master, p.GetId(), true, newKey()); err != nil {
		t.Fatalf("AnswerFavorRefund(Devolver) error = %v", err)
	}
	pass := f.mustAsk(t, f.toren.GetId(), f.pens.GetId())
	if _, err := f.answer(pass.GetId(), true); err != nil {
		t.Fatalf("AnswerFavorPass(Aceitar) error = %v", err)
	}
	f.dropTheHold(t, holdID)
	if n := f.rows(t); n != 1 || !f.has(t, f.pens.GetId()) {
		t.Errorf("character_favor has %d rows (Pensantus has it %v), want the one Pensantus got", n, f.has(t, f.pens.GetId()))
	}
}

// "Devolver" gave it back and Toren spent it on something else (the row is gone): the dying hold of the first roll gives nothing back.
func TestReturnedRefundIsNotGivenBackAgainWhenTheHoldDiesAfterASpend(t *testing.T) {
	t.Parallel()
	f, holdID, p := heldRefundFight(t)
	if _, err := f.answerRefund(f.master, p.GetId(), true, newKey()); err != nil {
		t.Fatalf("AnswerFavorRefund(Devolver) error = %v", err)
	}
	f.execSQL(t, `DELETE FROM character_favor WHERE character_id = $1`, f.toren.GetId()) // spent again, on another roll
	f.dropTheHold(t, holdID)
	if n := f.rows(t); n != 0 {
		t.Errorf("character_favor has %d rows, want 0: the forgotten hold gave back an Inspiração that was already returned and spent", n)
	}
}

// The undo of the roll after "Devolver" gives nothing back either, and the prompt keeps its answer.
func TestReturnedRefundIsNotGivenBackAgainByTheUndoOfTheRoll(t *testing.T) {
	t.Parallel()
	f, p := refundFight(t)
	if _, err := f.answerRefund(f.master, p.GetId(), true, newKey()); err != nil {
		t.Fatalf("AnswerFavorRefund(Devolver) error = %v", err)
	}
	f.execSQL(t, `DELETE FROM character_favor WHERE character_id = $1`, f.toren.GetId()) // spent again, on another roll
	if err := f.undo(t, f.master, f.e, f.undoableLine(t).GetId()); err != nil {
		t.Fatalf("UndoLastAction(the roll) error = %v", err)
	}
	if n := f.rows(t); n != 0 {
		t.Errorf("character_favor has %d rows, want 0: the undo gave back an Inspiração that was already returned and spent", n)
	}
	if status := f.promptStatus(t, p.GetId()); status != "returned" {
		t.Errorf("the prompt is %q, want returned", status)
	}
}
