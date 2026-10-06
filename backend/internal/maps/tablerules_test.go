package maps

import (
	"testing"

	"connectrpc.com/connect"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
)

// setNewMapFog saves the table's rules with the fog of new maps on or off, as the
// master does on "Regras da mesa" (RN-24).
func (u *user) setNewMapFog(campaignID string, on bool) {
	u.h.t.Helper()
	got, err := u.campaigns.GetTableRules(u.h.t.Context(), connect.NewRequest(&campaignsv1.GetTableRulesRequest{CampaignId: campaignID}))
	if err != nil {
		u.h.t.Fatalf("GetTableRules() error = %v", err)
	}
	r := got.Msg.GetRules()
	r.FogOnNewMaps = on
	if _, err := u.campaigns.SetTableRules(u.h.t.Context(), connect.NewRequest(&campaignsv1.SetTableRulesRequest{CampaignId: campaignID, Rules: r})); err != nil {
		u.h.t.Fatalf("SetTableRules() error = %v", err)
	}
}

// TestRN24_NewMapsGetTheFogTheTableChoseWithTheirFirstGrid: a map has no fog until
// the table's rules say so. The fog is made of the grid's squares, so a map made
// while the rule is on has none until its first grid (it never says "fog on" with
// no grid, which the fog code reads as off and would send the players the raw
// image); then it comes on, an image that another map uses is copied (RN-10), the
// players of a revealed map no longer get the image, and a map that already
// existed keeps its own.
func TestRN24_NewMapsGetTheFogTheTableChoseWithTheirFirstGrid(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, ana := h.newUser("Mestre"), h.newUser("Ana")
	campaign := h.newCampaign(master, ana)
	image := master.newImage(campaign)

	before := master.createMap(campaign, "Antes", image)
	master.mustSetGrid(campaign, before.GetId(), 20)
	if got := master.mustGetMap(campaign, before.GetId()).GetMap(); got.GetFogEnabled() {
		t.Fatal("a map made with the default rules has the fog on")
	}

	master.setNewMapFog(campaign, true)
	// Made while the rule is on: no fog without a grid, the players' view included.
	m := master.createMap(campaign, "Depois", image) // the same image as "Antes"
	master.setMapRevealed(campaign, m.GetId(), true)
	if m.GetFogEnabled() {
		t.Error("a map with no grid says the fog is on")
	}
	if got := ana.mustGetMap(campaign, m.GetId()).GetMap(); got.GetFogEnabled() {
		t.Errorf("the player reads fog_enabled on a map with no grid")
	}
	// The first grid turns the fog on, with a copy of the image the other map shares.
	withGrid := master.mustSetGrid(campaign, m.GetId(), 20)
	if !withGrid.GetFogEnabled() {
		t.Fatal("the first grid did not turn the table's fog on")
	}
	if withGrid.GetImage().GetId() == image {
		t.Errorf("the fog map still has the shared image %q, want a copy", image)
	}
	player := ana.mustGetMap(campaign, m.GetId()).GetMap()
	if !player.GetFogEnabled() || player.GetImage().GetId() != "" || player.GetImage().GetUrl() != "" {
		t.Errorf("the player's view of the fog map = fog %v, image %v; want fog on and no image", player.GetFogEnabled(), player.GetImage())
	}
	// The rule is spent: removing the grid and setting it again does not turn the fog on again,
	// and the map that already existed was never touched.
	master.mustSetGrid(campaign, m.GetId(), 0)
	master.mustSetGrid(campaign, m.GetId(), 20)
	if got := master.mustGetMap(campaign, m.GetId()).GetMap(); got.GetFogEnabled() {
		t.Error("the fog came back on with a second grid")
	}
	if got := master.mustGetMap(campaign, before.GetId()).GetMap(); got.GetFogEnabled() || got.GetImage().GetId() != image {
		t.Errorf("the map that already existed changed: fog %v, image %q", got.GetFogEnabled(), got.GetImage().GetId())
	}

	// Switching the rule off: a map made after it has no fog with its first grid.
	master.setNewMapFog(campaign, false)
	if after := master.createMap(campaign, "Sem névoa", master.newImage(campaign)); master.mustSetGrid(campaign, after.GetId(), 20).GetFogEnabled() {
		t.Error("a map made after switching the rule off has the fog on")
	}
}
