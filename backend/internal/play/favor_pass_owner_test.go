package play

import (
	"testing"

	"connectrpc.com/connect"
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
