package play

import (
	"slices"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The defensive path of SubmitInitiative: no call makes a pair in SETUP (MountCombatant refuses it), so a pair that stands there is a row
// written outside the calls (here, SQL). The rider's SubmitInitiative still gives the controlled mount the rider's initiative, as getting
// on after the roll does, and the pair begins the combat as one turn.
func TestMountedCombat_ThePairIsOrderedByTheRidersFirstRollDefensively(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	e := c.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: c.capitao.GetId()}, {CharacterId: c.ogre.GetId()}},
		npcRolls: []int{2, 15},
		players:  map[string]int32{"Pensantus": 10, "Brisa": 1},
		reveal:   []string{"Capitão Goblin", "Ogro"},
		at:       map[string][2]int32{"Pensantus": {5, 8}, "Brisa": {4, 7}, "Capitão Goblin": {12, 13}, "Ogro": {7, 7}, "Toren": {7, 7}},
		setup:    true,
	})
	c.mustMark(t, "Ogro", "controlled")
	c.sitOn(t, "Toren", "Ogro")
	c.submitFaceKeepingTheD20(t, e, "Toren", 18)

	var toren, ogro int32
	for _, who := range []struct {
		label string
		into  *int32
	}{{"Toren", &toren}, {"Ogro", &ogro}} {
		if err := c.h.pool.QueryRow(t.Context(), `SELECT initiative FROM combatants WHERE id = $1`, c.id(t, who.label)).Scan(who.into); err != nil {
			t.Fatalf("read %s: %v", who.label, err)
		}
	}
	if ogro != toren {
		t.Errorf("the mount's initiative = %d, want the rider's %d", ogro, toren)
	}
	order := c.orderNow(t)
	if at := slices.Index(order, "Toren"); at < 0 || at+1 >= len(order) || order[at+1] != "Ogro" {
		t.Errorf("order = %v, want the Ogro right after Toren", order)
	}
	if _, err := c.master.combat.BeginCombat(t.Context(), connect.NewRequest(&playv1.BeginCombatRequest{CampaignId: c.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey()})); err != nil {
		t.Fatalf("BeginCombat() error = %v", err)
	}
	if got := group(c.get(t, c.master)); got != "Toren,Ogro" {
		t.Errorf("the first turn's group = %q, want Toren,Ogro", got)
	}
}
