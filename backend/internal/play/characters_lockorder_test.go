package play

import (
	"testing"
	"time"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/dbtest"
)

// The lock order of the writes of the characters module that go through the session's funnel and name a character or a link:
// the open session's row first, then the character's and the link's. A claim or a return to the reserve that locked the
// character before the session would hold it while it waits for the session, and a combat write that holds the session and
// reaches for the same character would each wait for what the other holds. (ReviveCharacter, LevelUpCharacter and the sheet's
// saves race a combat write in equip_lockorder_test.go.)
//
// The test holds the session's row from another connection, starts the write and waits until it is stopped at that row.
// Whatever the write took before that, it holds now: the character and the link must be free to lock.

// stoppedAtTheSession runs write while the session's row is held by another connection and, once the write waits for it, calls
// free; then it releases the row and the write must end without an error.
func (a *armed) stoppedAtTheSession(t *testing.T, write func() error, free func(t *testing.T)) {
	t.Helper()
	ctx := t.Context()
	var sessionID string
	if err := a.h.pool.QueryRow(ctx, `SELECT id FROM game_sessions WHERE campaign_id = $1 AND ended_at IS NULL`, a.campaignID).Scan(&sessionID); err != nil {
		t.Fatalf("read the open session: %v", err)
	}
	holder, err := dbtest.SideConnection(t, a.h.pool).Begin(ctx)
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	defer func() { _ = holder.Rollback(t.Context()) }()
	if _, err := holder.Exec(ctx, `SELECT id FROM game_sessions WHERE id = $1 FOR UPDATE`, sessionID); err != nil {
		t.Fatalf("lock the session: %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- write() }()
	observer := dbtest.SideConnection(t, a.h.pool)
	for deadline := time.Now().Add(30 * time.Second); ; time.Sleep(5 * time.Millisecond) {
		n, err := dbtest.WaitingForTable(ctx, observer, "game_sessions")
		if err != nil {
			t.Fatalf("look for the waiting write: %v", err)
		}
		if n > 0 {
			break
		}
		select {
		case err := <-done:
			t.Fatalf("the write ended (%v) while the session's row was held", err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("the write never waited for the session's row")
		}
	}
	free(t)
	if err := holder.Rollback(ctx); err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}
	if err := <-done; err != nil {
		t.Errorf("the write after the session's row was released: %v", err)
	}
}

// notLocked fails the test if the row the write should not hold yet is locked: it asks for the row without waiting.
func (a *armed) notLocked(t *testing.T, what, query, id string) {
	t.Helper()
	tx, err := dbtest.SideConnection(t, a.h.pool).Begin(t.Context())
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	defer func() { _ = tx.Rollback(t.Context()) }()
	if _, err := tx.Exec(t.Context(), query, id); err != nil {
		t.Errorf("%s is locked while the write waits for the session's row (the session's row comes first): %v", what, err)
	}
}

// A claim takes the session's row before the character's and the link's.
func TestClaimCharacterTakesTheSessionBeforeTheCharacter(t *testing.T) {
	t.Parallel()
	a, reserved := partyTable(t)
	token := a.claimToken(t, reserved)
	a.stoppedAtTheSession(t, func() error {
		_, err := a.bia.characters.ClaimCharacter(t.Context(), connect.NewRequest(&charactersv1.ClaimCharacterRequest{Token: token}))
		return err
	}, func(t *testing.T) {
		t.Helper()
		a.notLocked(t, "the character", `SELECT id FROM characters WHERE id = $1 FOR UPDATE NOWAIT`, reserved.GetId())
		a.notLocked(t, "the claim link", `SELECT id FROM claim_links WHERE character_id = $1 FOR UPDATE NOWAIT`, reserved.GetId())
	})
}

// A return to the reserve takes the session's row before the character's.
func TestReturnCharacterToReserveTakesTheSessionBeforeTheCharacter(t *testing.T) {
	t.Parallel()
	a, reserved := partyTable(t)
	token := a.claimToken(t, reserved)
	if _, err := a.bia.characters.ClaimCharacter(t.Context(), connect.NewRequest(&charactersv1.ClaimCharacterRequest{Token: token})); err != nil {
		t.Fatalf("ClaimCharacter() error = %v", err)
	}
	a.stoppedAtTheSession(t, func() error {
		_, err := a.master.characters.ReturnCharacterToReserve(t.Context(), connect.NewRequest(&charactersv1.ReturnCharacterToReserveRequest{CampaignId: a.campaignID, CharacterId: reserved.GetId()}))
		return err
	}, func(t *testing.T) {
		t.Helper()
		a.notLocked(t, "the character", `SELECT id FROM characters WHERE id = $1 FOR UPDATE NOWAIT`, reserved.GetId())
	})
}
