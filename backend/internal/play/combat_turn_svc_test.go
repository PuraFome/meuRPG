package play

import "testing"

// A turn starts only in a transaction that knows the service: its start settles the effects and opens the windows of the features
// that wait for it, and a transaction without the service would start it and skip both. Every caller (BeginCombat, EndTurn, the
// release of a held turn, the one that leaves the turn) runs in Service.write, whose transaction carries the service; the few
// transactions built without it have no encounter. A call without it is a mistake: it is refused, it does not start the turn.
func TestAStartedTurnNeedsTheService(t *testing.T) {
	t.Parallel()
	if err := startTurn(t.Context(), &combatTx{}, []string{"a-combatant"}, 1); err == nil {
		t.Error("startTurn() with no service started the turn; want an error")
	}
}
