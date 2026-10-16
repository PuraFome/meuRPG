package play

import (
	"fmt"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// rpcAdvance moves the game time on and writes the rows and the vitals left.
func (a *armed) rpcAdvance(t *testing.T, s *rpcScript, seconds int32) {
	t.Helper()
	a.advanceGameTime(t, seconds)
	s.note(fmt.Sprintf("after %d seconds: rows", seconds), a.rpcCharRows(t))
	s.note(fmt.Sprintf("after %d seconds: Brisa", seconds), a.vitalsLine(t, a.bri))
}

// rpcLongRest has the master take a long rest.
func (a *armed) rpcLongRest(t *testing.T) error {
	t.Helper()
	_, err := a.master.resource.TakeRest(t.Context(), connect.NewRequest(&playv1.TakeRestRequest{CampaignId: a.campaignID, Kind: playv1.RestKind_REST_KIND_LONG, IdempotencyKey: newKey()}))
	return err
}

// TestLastingRPCGameTimeTakesTheEffectsAway freezes advanceGameTime and the long rest: what ran
// out of time ends with the temporary hit points it gave and the cast that held it.
func TestLastingRPCGameTimeTakesTheEffectsAway(t *testing.T) {
	t.Parallel()
	a := newCastingParty(t)
	a.rpcOutsideParty(t)
	a.rpcTempHP(t, a.bri, 6)
	s := a.rpcScript(t)
	s.note("before", a.rpcCharRows(t))
	a.rpcAdvance(t, s, 1)
	a.rpcAdvance(t, s, 60)
	a.rpcAdvance(t, s, 59)
	s.note("casts", castLines(a.casts(t, a.bia).GetActive())+"log: "+castLines(a.casts(t, a.bia).GetLog()))
	// A long rest ends what lasts until it, and the time of the rest (8 hours) ends the others.
	if err := a.rpcLongRest(t); err != nil {
		t.Fatalf("TakeRest(long) error = %v", err)
	}
	s.note("after the long rest: rows", a.rpcCharRows(t))
	s.want("outside_time")
}

// TestLastingRPCALongRestEndsTheEffectsOfACombatInSetup freezes the long rest while a combat is
// being set up: its effects that last until a long rest end with the others.
func TestLastingRPCALongRestEndsTheEffectsOfACombatInSetup(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: a.goblin.GetId()}},
		npcRolls: []int{3},
		players:  map[string]int32{"Toren": 18, "Pensantus": 10, "Brisa": 1},
		setup:    true,
	})
	a.rpcAdd(t, e, longstriderKey, []string{"Toren"}, a.rpcDuration(t, rpcDurLongRest, 0, ""))
	a.rpcAdd(t, e, guidanceKey, []string{"Brisa"}, a.rpcDuration(t, rpcDurDismissed, 0, ""))
	s := a.rpcScript(t)
	s.state("before")
	if err := a.rpcLongRest(t); err != nil {
		s.say("a long rest in a combat", nil, err)
	}
	s.state("after")
	s.want("outside_long_rest_in_combat")
}
