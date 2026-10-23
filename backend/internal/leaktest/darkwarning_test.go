package leaktest

import (
	"strings"
	"testing"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1/mapsv1connect"
)

// The master's light warning (ListCharactersInTheDark) is worked out from every light
// on the map, the lights of NPCs the players cannot see included, so it is the
// master's alone (RN-10). The matrix (reads_test.go) classifies the read as
// master-only and proves every other person is refused. This test adds the positive
// control with a needle that has to be found: the fixture's fog map in "Escuro" has
// Toren (human, no darkvision) in the south room with no light, and the master's answer
// names him; Pensantus (gnome, darkvision) is not named. A hidden NPC with a torch
// next to Toren makes the warning go away, and no player's answer ever names the NPC.
func TestTheDarkWarningIsTheMastersAlone(t *testing.T) {
	w := newWorld(t)
	ctx := t.Context()
	must(w.master.maps.SetMapFog(ctx, rq(&mapsv1.SetMapFogRequest{CampaignId: w.campaign, MapId: w.fogMap, BaseLight: mapsv1.LightLevel_LIGHT_LEVEL_DARK.Enum()})))

	ask := func(p *person) reply {
		return p.call(mapsv1connect.MapServiceListCharactersInTheDarkProcedure, &mapsv1.ListCharactersInTheDarkRequest{CampaignId: w.campaign, MapId: w.fogMap})
	}
	needles := []string{w.toren.GetId(), w.pens.GetId(), w.hiddenNPC.GetId()}

	// No player, pending member or stranger gets an answer that holds any of them.
	checkPlayers := func(state string) {
		t.Helper()
		for _, p := range w.players() {
			r := ask(p)
			if r.ok() {
				t.Errorf("%s (%s): the call worked: %s", p.name, state, r.body)
			}
			for _, n := range needles {
				if strings.Contains(string(r.body), n) {
					t.Errorf("%s (%s): the answer holds %s: %s", p.name, state, n, r.body)
				}
			}
		}
	}

	// The positive control: the master is told of Toren, and only of him.
	master := ask(w.master)
	if !master.ok() {
		t.Fatalf("the master's call failed: HTTP %d %s", master.status, master.body)
	}
	body := string(master.body)
	if !strings.Contains(body, w.toren.GetId()) || strings.Contains(body, w.pens.GetId()) || strings.Contains(body, w.hiddenNPC.GetId()) {
		t.Fatalf("the master's answer = %s, want Toren (no light, no darkvision) and nobody else", body)
	}

	checkPlayers("Toren in the dark")

	// A hidden NPC's torch lights Toren: the warning is gone, and still names no NPC.
	must(w.master.maps.SetCarriedLight(ctx, rq(&mapsv1.SetCarriedLightRequest{CampaignId: w.campaign, MapId: w.fogMap, CharacterId: w.hiddenNPC.GetId(), LightKey: "light:torch"})))
	w.place(w.fogMap, w.hiddenNPC.GetId(), 10, 12)
	lit := ask(w.master)
	if !lit.ok() || strings.Contains(string(lit.body), w.toren.GetId()) {
		t.Errorf("with the hidden NPC's torch beside him: HTTP %d %s, want Toren no longer in the dark", lit.status, lit.body)
	}
	checkPlayers("lit by the hidden NPC's torch")
}
