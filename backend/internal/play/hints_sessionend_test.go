package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// Ending the session (#735 part 2, 4d) goes through Service.tx: the session's row is locked first, its number moves once with the end,
// and every stream hears the end with it. A repeat of the call, on a session ended already, changes and tells nothing.

const kSessionEnded = "*playv1.WatchGameSessionResponse_SessionEnded_"

func (a *armed) endCall(t *testing.T, sessionID string) func() error {
	return func() error {
		_, err := a.master.play.EndGameSession(t.Context(), connect.NewRequest(&playv1.EndGameSessionRequest{CampaignId: a.campaignID, GameSessionId: sessionID}))
		return err
	}
}

func TestEndGameSessionHints(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	g := a.stageHints(t)
	heard := g.first(a.endCall(t, g.session), true)
	heardKinds(t, heard, map[string][]string{"master": {kSessionEnded}, "Ana": {kSessionEnded}, "Caio": {kSessionEnded}, "Bia": {kSessionEnded}})
}

func TestEndGameSessionAgainMovesNothing(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	g := a.stageHints(t)
	do(t, a.endCall(t, g.session))
	g.drain()
	before := g.seq()
	do(t, a.endCall(t, g.session))
	if got := g.seq(); got != before {
		t.Errorf("hint_seq = %d, want %d: the session was ended already", got, before)
	}
	for name, evs := range g.drain() {
		if len(evs) != 0 {
			t.Errorf("%s heard %v of a session ended before", name, eventKinds(evs))
		}
	}
}

func TestEndGameSessionRefusedTellsNothing(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	g := a.stageHints(t)
	g.refused(a.endCall(t, newKey())) // no such session
}
