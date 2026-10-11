package play

import (
	"testing"
	"time"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The master's "Chamar a atenção" (NudgePlayer): a stream-only event to one player, nothing stored.
// These tests need the database (MEURPG_TEST_DATABASE_URL), except the cooldown book's.

func (a *armed) nudge(u *user, characterID string) error {
	_, err := u.play.NudgePlayer(a.h.t.Context(), connect.NewRequest(&playv1.NudgePlayerRequest{CampaignId: a.campaignID, CharacterId: characterID}))
	return err
}

func TestNudgePlayerReachesOnlyThatPlayersStream(t *testing.T) {
	a := newArmed(t)
	ana, caio, master := a.ana.watch(t, a.campaignID), a.caio.watch(t, a.campaignID), a.master.watch(t, a.campaignID)
	for _, w := range []*watcher{ana, caio, master} {
		w.ready(t)
	}
	if err := a.nudge(a.master, a.pens.GetId()); err != nil {
		t.Fatalf("NudgePlayer() error = %v", err)
	}
	// The marker ends the wait for nothing: whatever was sent before it is read.
	a.master.markCurrentMap(t, a.campaignID, a.mapID, ana, caio, master)
	if got := ana.beforeMarker(t); len(got) == 0 || got[0].GetCallingYou() == nil {
		t.Errorf("Ana's stream got %v, want calling_you", got)
	}
	for name, w := range map[string]*watcher{"Caio": caio, "the master": master} {
		for _, ev := range w.beforeMarker(t) {
			if ev.GetCallingYou() != nil {
				t.Errorf("%s's stream got calling_you, want it for Ana alone", name)
			}
		}
	}
}

func TestNudgePlayerIsTheMastersOnly(t *testing.T) {
	a := newArmed(t)
	wantCode(t, "NudgePlayer as a player", a.nudge(a.caio, a.pens.GetId()), connect.CodePermissionDenied)
}

func TestNudgePlayerRefusesWhatIsNotAPlayersCharacter(t *testing.T) {
	a := newArmed(t)
	wantCode(t, "NudgePlayer for an NPC", a.nudge(a.master, a.goblin.GetId()), connect.CodeNotFound)
	wantCode(t, "NudgePlayer for an unknown character", a.nudge(a.master, newKey()), connect.CodeNotFound)
	wantCode(t, "NudgePlayer for a malformed ID", a.nudge(a.master, "x"), connect.CodeNotFound)
}

func TestNudgePlayerWaitsForTheCooldown(t *testing.T) {
	a := newArmed(t)
	if err := a.nudge(a.master, a.pens.GetId()); err != nil {
		t.Fatalf("NudgePlayer() error = %v", err)
	}
	wantCode(t, "the second NudgePlayer", a.nudge(a.master, a.pens.GetId()), connect.CodeResourceExhausted)
	// Another character is not held back by Pensantus's wait.
	if err := a.nudge(a.master, a.toren.GetId()); err != nil {
		t.Errorf("NudgePlayer(Toren) error = %v, want none", err)
	}
}

func TestNudgePlayerNeedsAnOpenSession(t *testing.T) {
	a := newArmed(t)
	started := a.master.liveSession(t, a.campaignID).GetGameSession()
	a.master.end(t, started)
	err := a.nudge(a.master, a.pens.GetId())
	wantBlocked(t, "NudgePlayer", err, playv1.GameSessionBlockedReason_GAME_SESSION_BLOCKED_REASON_NO_OPEN_SESSION)
}

func TestNudgeBookCooldown(t *testing.T) {
	t.Parallel()
	var b nudgeBook
	at := time.Date(2026, 10, 11, 20, 0, 0, 0, time.UTC)
	if !b.take("a", at) {
		t.Fatal("the first nudge was refused")
	}
	if b.take("a", at.Add(nudgeCooldown-time.Millisecond)) {
		t.Error("a nudge inside the cooldown was allowed")
	}
	if !b.take("b", at.Add(time.Second)) {
		t.Error("another character was held back")
	}
	if !b.take("a", at.Add(nudgeCooldown)) {
		t.Error("a nudge after the cooldown was refused")
	}
}
