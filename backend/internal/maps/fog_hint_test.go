package maps

import (
	"slices"
	"testing"
	"time"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/maps/mapsdb"
)

// TestRN10_AViewReadNeverSwallowsTheMoveHint is the race an e2e run caught
// (fog.spec.ts, "depois de andar"). A player's page reads its view as soon as
// `token_moved` arrives, and that read can come before the move's
// refreshVision. If the read counted as "told", the refresh found nothing new
// and sent no `vision_changed`, the page never read the tokens again, and a
// goblin that had gone out of sight stayed on the player's map (RN-10). Here
// the read is forced between the move's write and its refresh.
func TestRN10_AViewReadNeverSwallowsTheMoveHint(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	m := c.master
	ana := c.ana.watch(c.campaign)
	drain := func() []*playv1.WatchGameSessionResponse {
		m.setMapRevealed(c.campaign, c.probeMap, !m.mustGetMap(c.campaign, c.probeMap).GetMap().GetRevealed())
		return ana.drain(c.probeMap)
	}
	goblin := c.goblin2.GetName()
	if !slices.Contains(tokenNames(c.ana.mustGetMap(c.campaign, c.mapID)), goblin) {
		t.Fatalf("Pensantus should see %s at the start", goblin)
	}
	c.ana.mustVision(c.campaign, c.mapID) // her page has read her view
	drain()

	// Pensantus walks to the chest chamber: the move is written, its refresh has not run yet.
	x, y := at(10, 13)
	if _, err := c.h.svc.queries.MoveMapToken(t.Context(), mapsdb.MoveMapTokenParams{
		MapID: c.mapID, CharacterID: c.pens.GetId(), XBp: x, YBp: y, UpdatedAt: time.Now().UTC().Truncate(time.Microsecond),
	}); err != nil {
		t.Fatal(err)
	}
	// Her page heard `token_moved` and reads the view first.
	c.ana.mustVision(c.campaign, c.mapID)
	// Then the move's refresh runs.
	c.h.svc.refreshVision(t.Context(), c.campaign, c.mapID)

	if evs := drain(); !slices.ContainsFunc(evs, func(e *playv1.WatchGameSessionResponse) bool {
		return e.GetVisionChanged().GetMapId() == c.mapID
	}) {
		t.Fatalf("Ana got %v after her view changed; want vision_changed, so her page reads the tokens again", evs)
	}
	if slices.Contains(tokenNames(c.ana.mustGetMap(c.campaign, c.mapID)), goblin) {
		t.Errorf("Ana still receives %s, out of her sight", goblin)
	}
}

// The master's light warning reads the same scene as the players' views, so a change of
// what is seen that moves no token (a zone that blocks sight, a form, a familiar's eyes)
// reaches the master as vision_changed, also when no player's view changed.
func TestRN37_VisionChangedAlsoReachesTheMaster(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	m := c.master
	master, ana := m.watch(c.campaign), c.ana.watch(c.campaign)
	sm := NewSessionMaps(c.h.pool)
	sm.SetService(c.h.svc)
	drain := func() (masterGot, anaGot []*playv1.WatchGameSessionResponse) {
		m.setMapRevealed(c.campaign, c.probeMap, !m.mustGetMap(c.campaign, c.probeMap).GetMap().GetRevealed())
		return master.drain(c.probeMap), ana.drain(c.probeMap)
	}
	drain() // what the setup left in the streams

	sm.VisionChanged(t.Context(), c.campaign, c.mapID)

	masterGot, anaGot := drain()
	if len(masterGot) != 1 || masterGot[0].GetVisionChanged().GetMapId() != c.mapID {
		t.Errorf("the master got %v, want one vision_changed for the map", masterGot)
	}
	if len(anaGot) != 0 {
		t.Errorf("a player whose view did not change got %v, want nothing", anaGot)
	}
}
