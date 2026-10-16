package play

import (
	"sync/atomic"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// These tests freeze what the shared parts of a combat change refuse: a log asked by
// who is no member or for a combat that is not there, a change after the session ended,
// and a change that gives up when the combat keeps changing under its sight. They need the
// database (MEURPG_TEST_DATABASE_URL).

// endSession ends the campaign's open game session, as the master.
func (a *armed) endSession(t *testing.T) {
	t.Helper()
	var id string
	if err := a.h.pool.QueryRow(t.Context(), `SELECT id FROM game_sessions WHERE campaign_id = $1 AND ended_at IS NULL`, a.campaignID).Scan(&id); err != nil {
		t.Fatalf("find the open session: %v", err)
	}
	a.master.end(t, &playv1.GameSession{CampaignId: a.campaignID, Id: id})
}

// TestListCombatLogRefusesWhoIsNoMemberAndACombatThatIsNotThere freezes the words.
func TestListCombatLogRefusesWhoIsNoMemberAndACombatThatIsNotThere(t *testing.T) {
	t.Parallel()
	a, e := rollsTable(t)
	list := func(u *user, campaign, enc string) error {
		_, err := u.combat.ListCombatLog(t.Context(), connect.NewRequest(&playv1.ListCombatLogRequest{CampaignId: campaign, EncounterId: enc}))
		return err
	}
	outsider := a.h.newUser("Intruso")
	const (
		notFoundCampaign = "not_found|not_found: campaign not found"
		notFoundCombat   = "not_found|not_found: encounter not found"
		noSession        = "failed_precondition|failed_precondition: the campaign has no open game session|GAME_SESSION_BLOCKED_REASON_NO_OPEN_SESSION"
	)
	tests := []struct {
		name string
		got  error
		want string
	}{
		{"an outsider", list(outsider, a.campaignID, e.GetId()), notFoundCampaign},
		{"a campaign that does not exist", list(a.master, newKey(), e.GetId()), notFoundCampaign},
		{"a combat that is not a UUID", list(a.master, a.campaignID, "x"), notFoundCombat},
		{"no combat", list(a.master, a.campaignID, ""), notFoundCombat},
		{"a combat that does not exist", list(a.caio, a.campaignID, newKey()), notFoundCombat},
	}
	for _, tt := range tests {
		if got := shapeOf(tt.got); got != tt.want {
			t.Errorf("ListCombatLog(%s) = %s, want %s", tt.name, got, tt.want)
		}
	}
	// Once the session is over there is no combat to read, and no change to make.
	a.endSession(t)
	if got := shapeOf(list(a.master, a.campaignID, e.GetId())); got != noSession {
		t.Errorf("ListCombatLog(after the session) = %s, want %s", got, noSession)
	}
	if got := shapeOf(a.undo(t, a.master, e, newKey())); got != noSession {
		t.Errorf("UndoLastAction(after the session) = %s, want %s", got, noSession)
	}
}

// TestCombatChangeGivesUpWhenTheCombatKeepsChanging: the sight is read before the
// transaction and read again when the combat changed meanwhile; a combat that changes under
// every try aborts the change after maxSightTries.
func TestCombatChangeGivesUpWhenTheCombatKeepsChanging(t *testing.T) {
	t.Parallel()
	f := newFogCave(t)
	e := f.fight(t)
	var inHook atomic.Bool
	var reads atomic.Int32
	// Toren goes behind the walls and back: the combat changes under every try.
	places := [][2]int32{{7, 12}, {6, 7}}
	f.h.svc.afterSightRead = func(string) {
		if !inHook.CompareAndSwap(false, true) {
			return
		}
		defer inHook.Store(false)
		at := places[reads.Add(1)%2]
		f.mustMove(t, f.master, "Toren", at[0], at[1])
	}
	_, err := f.conditions(t, f.master, e, "Goblin 1", []string{"condition:prone"}, true, false)
	f.h.svc.afterSightRead = nil
	if got, want := shapeOf(err), "aborted|aborted: the combat keeps changing: try again"; got != want {
		t.Errorf("SetCombatantConditions(under a combat that keeps changing) = %s, want %s", got, want)
	}
	if got := reads.Load(); got != maxSightTries {
		t.Errorf("the sight was read %d times, want %d", got, maxSightTries)
	}
}
