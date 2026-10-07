package maps

import (
	"net/http"
	"slices"
	"strconv"
	"testing"
	"time"

	"connectrpc.com/connect"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/maps/mapsdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

// Round two of the fog's review: what is remembered and when, the clear that must
// beat a late refresh, who is told after a restart, the parents of a map, the
// images a fog map owns, and a combat running on the map.

func codeAt(t *testing.T, res *mapsv1.GetMapVisionResponse, sq grid.Square) byte {
	t.Helper()
	return codes(t, res)[sq.Row*caveGrid.Columns+sq.Col]
}

func (c *cave) rememberedBy(u *user) int {
	n := 0
	for _, code := range codes(c.h.t, u.mustVision(c.campaign, c.mapID)) {
		if code == 5 {
			n++
		}
	}
	return n
}

// A map the master prepares in private records nothing: Toren dragged through it and
// back remembers nothing once the players see it, whether the master reveals it or
// makes it the session's current map.
func TestMR036_NothingIsRememberedOnAMapPlayersCannotSee(t *testing.T) {
	t.Parallel()
	for _, how := range []string{"revealed", "current"} {
		t.Run(how, func(t *testing.T) {
			t.Parallel()
			c := newCaveWith(t, false)
			c.master.placeAt(c.campaign, c.mapID, c.toren.GetId(), grid.Square{Col: 14, Row: 7})
			c.master.placeAt(c.campaign, c.mapID, c.toren.GetId(), grid.Square{Col: 12, Row: 12})
			c.master.placeAt(c.campaign, c.mapID, c.toren.GetId(), sqToren)
			if n := c.memoryRows(); n != 0 {
				t.Fatalf("%d memories are stored for a map the players cannot see", n)
			}
			if how == "revealed" {
				c.master.setMapRevealed(c.campaign, c.mapID, true)
			} else if _, err := c.master.setCurrentMap(c.campaign, c.mapID); err != nil {
				t.Fatal(err)
			}
			if got := c.rememberedBy(c.caio); got != 0 {
				t.Errorf("Caio remembers %d squares he never saw", got)
			}
			diffRows(t, "Toren once the map is shown", picture(t, c.caio.mustVision(c.campaign, c.mapID), sqToren), caveOracle["toren"])
			if c.memoryRows() == 0 {
				t.Error("the first view the players have of the map was not recorded when it became visible")
			}
		})
	}
}

// The memory belongs to an epoch: a clear bumps it, and a refresh that was running
// builds its memory for the old one and is refused; a row of an older epoch reads
// as empty.
func TestMR036_AClearBeatsALateRefresh(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	ctx := t.Context()
	inputOf := func() fogInput {
		row, err := c.h.svc.queries.GetMap(ctx, mapsdb.GetMapParams{CampaignID: c.campaign, ID: c.mapID})
		if err != nil {
			t.Fatal(err)
		}
		return fogInput{mapID: c.mapID, campaignID: c.campaign, g: caveGrid, epoch: row.VisionEpoch}
	}
	chest := grid.Square{Col: 12, Row: 13} // a square nobody sees
	stale := grid.NewLayer(caveGrid)
	stale.Set(chest.Col, chest.Row, true)

	before := inputOf()
	if _, err := c.master.maps.ForgetMapVision(ctx, connect.NewRequest(&mapsv1.ForgetMapVisionRequest{CampaignId: c.campaign, MapId: c.mapID})); err != nil {
		t.Fatal(err)
	}
	if after := inputOf(); after.epoch != before.epoch+1 {
		t.Fatalf("epoch after the clear = %d, want %d", after.epoch, before.epoch+1)
	}
	// The refresh that started before the clear arrives after it: nothing is written.
	wrote, err := c.h.svc.rememberFor(ctx, before, c.ana.id, stale)
	if err != nil || wrote {
		t.Fatalf("a late refresh wrote = %v, %v; want refused", wrote, err)
	}
	if got := codeAt(t, c.ana.mustVision(c.campaign, c.mapID), chest); got != 0 {
		t.Errorf("the chest chamber reads %d after a refused write, want unseen", got)
	}
	// One built for the current epoch is written, and remembered.
	if wrote, err := c.h.svc.rememberFor(ctx, inputOf(), c.ana.id, stale); err != nil || !wrote {
		t.Fatalf("a refresh for the current epoch wrote = %v, %v; want written", wrote, err)
	}
	if got := codeAt(t, c.ana.mustVision(c.campaign, c.mapID), chest); got != 5 {
		t.Errorf("the chest chamber reads %d after a good write, want remembered", got)
	}
	// A row of an older epoch is empty to every read, even if it is still there.
	if _, err := c.h.pool.Exec(ctx, `UPDATE map_vision_memory SET epoch = epoch - 1 WHERE map_id = $1`, c.mapID); err != nil {
		t.Fatal(err)
	}
	if got := codeAt(t, c.ana.mustVision(c.campaign, c.mapID), chest); got != 0 {
		t.Errorf("a row of an older epoch reads %d, want unseen", got)
	}
	// A new grid bumps it too.
	e := inputOf().epoch
	if _, err := c.master.setGrid(c.campaign, c.mapID, 25); err != nil {
		t.Fatal(err)
	}
	if got := inputOf().epoch; got != e+1 {
		t.Errorf("epoch after a new grid = %d, want %d", got, e+1)
	}
}

// After a restart (or when the server let a map go) nobody is told for nothing: a
// goblin moving between squares nobody sees sends nothing, and one moving in view still does.
func TestMR036_AColdStartTellsNobodyForNothing(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	m := c.master
	watchers := map[string]*watcher{"Ana": c.ana.watch(c.campaign), "Caio": c.caio.watch(c.campaign), "Bia": c.bia.watch(c.campaign)}
	drain := func() map[string][]*playv1.WatchGameSessionResponse {
		m.setMapRevealed(c.campaign, c.probeMap, !m.mustGetMap(c.campaign, c.probeMap).GetMap().GetRevealed())
		out := map[string][]*playv1.WatchGameSessionResponse{}
		for name, w := range watchers {
			out[name] = w.drain(c.probeMap)
		}
		return out
	}
	drain()
	c.h.svc.seen.mu.Lock()
	c.h.svc.seen.last, c.h.svc.seen.order = nil, nil // the process just started
	c.h.svc.seen.mu.Unlock()

	m.placeAt(c.campaign, c.mapID, c.goblin1.GetId(), grid.Square{Col: 18, Row: 6})
	for name, evs := range drain() {
		if len(evs) != 0 {
			t.Errorf("%s got %v after a restart for a goblin moving in the dark, want nothing", name, evs)
		}
	}
	m.placeAt(c.campaign, c.mapID, c.goblin2.GetId(), grid.Square{Col: 21, Row: 7})
	for name, evs := range drain() {
		if len(evs) != 1 || evs[0].GetVisionChanged().GetMapId() != c.mapID {
			t.Errorf("%s got %v for a goblin moving in view after a restart, want one vision_changed", name, evs)
		}
	}
}

// The table of what each player was told keeps the last maps used, and lets the
// oldest go, never all at once.
func TestVisionStateEvictsTheOldestMap(t *testing.T) {
	t.Parallel()
	var v visionState
	for i := range maxTrackedMaps + 10 {
		v.swap("map-"+strconv.Itoa(i), "user", 7)
	}
	if _, had := v.swap("map-0", "user", 7); had {
		t.Error("the oldest map is still tracked")
	}
	if before, had := v.swap("map-"+strconv.Itoa(maxTrackedMaps+9), "user", 8); !had || before != 7 {
		t.Errorf("the newest map = %d, %v; want it kept", before, had)
	}
	if len(v.last) > maxTrackedMaps {
		t.Errorf("%d maps tracked, want at most %d", len(v.last), maxTrackedMaps)
	}
}

// A parent is named ("Submapa de ...") only through a submap point on a square the
// player knows, and never for a parent they do not see; "Ver como" gets the same.
func TestMR036_ParentsAreNamedOnlyThroughWhatThePlayerKnows(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	m := c.master
	child := m.createMap(c.campaign, "A cripta", m.newImage(c.campaign))
	m.setMapRevealed(c.campaign, child.GetId(), true)
	hiddenParent := m.createMap(c.campaign, "O mapa escondido", m.newImage(c.campaign))
	submap := func(mapID string, col, row int) {
		x, y := at(col, row)
		p := m.createPoint(&mapsv1.CreateMapPointRequest{
			CampaignId: c.campaign, MapId: mapID, Kind: mapsv1.MapPointKind_MAP_POINT_KIND_SUBMAP, Name: "Para a cripta",
			XBp: x, YBp: y, TargetMapId: child.GetId(),
		})
		m.setPointRevealed(c.campaign, p, true)
	}
	parents := func(u *user) []string {
		var out []string
		for _, p := range u.mustGetMap(c.campaign, child.GetId()).GetMap().GetParentMaps() {
			out = append(out, p.GetName())
		}
		return out
	}
	submap(c.mapID, 22, 13) // a square nobody sees
	submap(hiddenParent.GetId(), 3, 3)
	if got := parents(c.ana); len(got) != 0 {
		t.Errorf("Ana's parents = %v with the point on a square she does not know, want none", got)
	}
	if got := parents(m); len(got) != 2 {
		t.Errorf("the master's parents = %v, want both", got)
	}
	if asJSON(t, c.ana.mustGetMap(c.campaign, child.GetId())) != asJSON(t, m.mustGetMapAs(c.campaign, child.GetId(), c.pens.GetId())) {
		t.Error("the master seeing as Pensantus gets another map than Ana (a hidden parent)")
	}
	listed := func(u *user) []string {
		for _, mp := range u.listMaps(c.campaign) {
			if mp.GetId() == child.GetId() {
				var out []string
				for _, p := range mp.GetParentMaps() {
					out = append(out, p.GetName())
				}
				return out
			}
		}
		return nil
	}
	if got := listed(c.ana); len(got) != 0 {
		t.Errorf("ListMaps names the parents %v to Ana, want none", got)
	}
	// A point on the entrance, which Pensantus sees with darkvision and Toren does not.
	submap(c.mapID, 2, 7)
	if got := parents(c.ana); !slices.Equal(got, []string{"A caverna do Vale Seco"}) {
		t.Errorf("Ana's parents = %v, want the cave through the point she sees", got)
	}
	if got := parents(c.caio); len(got) != 0 {
		t.Errorf("Caio's parents = %v, want none: the point is on a square he does not see", got)
	}
}

// A combat running on the map: the players see from the combatants' squares, and
// the NPC map tokens (the squares of before the fight) are left out.
func TestMR036_AFogMapDuringACombat(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	ctx := t.Context()
	if _, err := c.master.combat.StartEncounter(ctx, connect.NewRequest(&playv1.StartEncounterRequest{
		CampaignId: c.campaign, IdempotencyKey: newKey(), Name: "A guarita",
		Participants: []*playv1.Participant{{CharacterId: c.toren.GetId()}, {CharacterId: c.goblin2.GetId()}},
	})); err != nil {
		t.Fatalf("StartEncounter() error = %v", err)
	}
	for _, name := range []string{"Ana", "Caio"} {
		u := map[string]*user{"Ana": c.ana, "Caio": c.caio}[name]
		if names := tokenNames(u.mustGetMap(c.campaign, c.mapID)); slices.Contains(names, "Goblin 2") || !slices.Contains(names, "Toren") {
			t.Errorf("%s's tokens during a combat = %v, want the party and no NPC", name, names)
		}
	}
	// Toren's combatant stands elsewhere than his token: he sees from there.
	if _, err := c.h.pool.Exec(ctx, `UPDATE combatants SET grid_col = 10, grid_row = 8 WHERE character_id = $1`, c.toren.GetId()); err != nil {
		t.Fatal(err)
	}
	res := c.caio.mustVision(c.campaign, c.mapID)
	if got := codeAt(t, res, grid.Square{Col: 10, Row: 8}); got != 2 {
		t.Errorf("the square of Toren's combatant reads %d, want 2 (his own, in the dark)", got)
	}
	if got := codeAt(t, res, sqToren); got == 2 || got == 3 || got == 4 {
		t.Errorf("Toren's token square reads %d during the combat, want not seen from there", got)
	}
}

// The master's painting reaches the players whose layers changed, even a square they
// only remember; the revision folds the layers in.
func TestMR036_PaintingTellsThePlayersWhoseLayersChanged(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	m := c.master
	ana, caio := c.ana.watch(c.campaign), c.caio.watch(c.campaign)
	drain := func() (a, k []*playv1.WatchGameSessionResponse) {
		m.setMapRevealed(c.campaign, c.probeMap, !m.mustGetMap(c.campaign, c.probeMap).GetMap().GetRevealed())
		return ana.drain(c.probeMap), caio.drain(c.probeMap)
	}
	drain()
	rev := c.ana.mustVision(c.campaign, c.mapID).GetRevision()
	// Rubble on a square Pensantus sees (the entrance) and Toren does not.
	m.mustPaint(c.campaign, c.mapID, mapsv1.MapLayer_MAP_LAYER_DIFFICULT_TERRAIN, 1, [2]int32{2, 7})
	a, k := drain()
	if len(a) != 1 || a[0].GetVisionChanged() == nil || len(k) != 0 {
		t.Errorf("after rubble on the entrance: Ana got %v, Caio got %v; want one vision_changed and nothing", a, k)
	}
	if c.ana.mustVision(c.campaign, c.mapID).GetRevision() == rev {
		t.Error("the revision did not change with the rubble")
	}
	// Pensantus leaves; a wall painted on a square she only remembers is news.
	m.placeAt(c.campaign, c.mapID, c.pens.GetId(), grid.Square{Col: 15, Row: 7})
	drain()
	m.mustPaint(c.campaign, c.mapID, mapsv1.MapLayer_MAP_LAYER_WALL, 1, [2]int32{3, 10})
	a, k = drain()
	if len(a) != 1 || a[0].GetVisionChanged() == nil || len(k) != 0 {
		t.Errorf("after a wall on a remembered square: Ana got %v, Caio got %v; want one vision_changed and nothing", a, k)
	}
}

// A paint the gate holds (painting is told at most once a second for a map) still
// reaches the players when the gate lets it go, after its own request ended.
func TestMR036_APaintHeldByTheGateStillReachesThePlayers(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	m := c.master
	c.h.svc.layerHintEvery = 300 * time.Millisecond // the harness makes it 0
	ana := c.ana.watch(c.campaign)
	visionHints := func() int {
		m.setMapRevealed(c.campaign, c.probeMap, !m.mustGetMap(c.campaign, c.probeMap).GetMap().GetRevealed())
		n := 0
		for _, ev := range ana.drain(c.probeMap) {
			if ev.GetVisionChanged() != nil {
				n++
			}
		}
		return n
	}
	time.Sleep(400 * time.Millisecond) // the setup's own paints are past the gate
	visionHints()
	// Rubble on the entrance, which Pensantus sees: told at once.
	m.mustPaint(c.campaign, c.mapID, mapsv1.MapLayer_MAP_LAYER_DIFFICULT_TERRAIN, 1, [2]int32{2, 7})
	if got := visionHints(); got != 1 {
		t.Fatalf("after the first paint Ana got %d vision_changed, want 1", got)
	}
	// The rubble cleared within the gate's interval: held, then told when it ends.
	m.mustPaint(c.campaign, c.mapID, mapsv1.MapLayer_MAP_LAYER_DIFFICULT_TERRAIN, 0, [2]int32{2, 7})
	time.Sleep(700 * time.Millisecond)
	if got := visionHints(); got != 1 {
		t.Errorf("after the held paint Ana got %d vision_changed, want 1", got)
	}
}

// ListMaps counts the points a player receives of a fog map, and the count follows
// a point revealed on a square they know.
func TestMR036_ListMapsCountFollowsThePoints(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	count := func() int32 { return c.ana.listMaps(c.campaign)[0].GetPointCount() }
	if got := count(); got != 1 {
		t.Fatalf("Ana's count = %d, want 1 (the entrance)", got)
	}
	if got := count(); got != 1 { // from the cache
		t.Fatalf("Ana's count the second time = %d, want 1", got)
	}
	x, y := at(5, 9)
	p := c.master.createPoint(&mapsv1.CreateMapPointRequest{
		CampaignId: c.campaign, MapId: c.mapID, Kind: mapsv1.MapPointKind_MAP_POINT_KIND_SCENE, Name: "Entulho", XBp: x, YBp: y,
	})
	if got := count(); got != 1 {
		t.Errorf("Ana's count with a hidden point = %d, want 1", got)
	}
	c.master.setPointRevealed(c.campaign, p, true)
	if got := count(); got != 2 {
		t.Errorf("Ana's count after the master revealed a point she sees = %d, want 2", got)
	}
}

// A player's own character is never hidden from them on a fog map, so they may set
// its light even if the master hid its token.
func TestMR036_ThePlayerSetsTheLightOfAHiddenOwnToken(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	c.master.setTokenHidden(c.campaign, c.mapID, c.toren.GetId(), true)
	_, err := c.caio.maps.SetCarriedLight(t.Context(), connect.NewRequest(&mapsv1.SetCarriedLightRequest{
		CampaignId: c.campaign, MapId: c.mapID, CharacterId: c.toren.GetId(), LightKey: "light:torch",
	}))
	if err != nil {
		t.Errorf("SetCarriedLight on a hidden own token of a fog map: %v", err)
	}
}

// RN-10, D6: a fog map's image is its own. The image the session shows, an image
// left with the players, a portrait and a map without fog each get a copy when it is
// the background of a fog map, and the route refuses only the fog map's own.
func TestRN10_AFogMapsImageIsNeverReusedRaw(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, ana := h.newUser("Mestre"), h.newUser("Ana")
	campaign := h.newCampaign(master, ana)
	x := master.newImage(campaign)
	fog := master.createMap(campaign, "Com névoa", x)
	master.mustSetGrid(campaign, fog.GetId(), 12)
	master.setMapRevealed(campaign, fog.GetId(), true)
	if _, err := master.maps.SetMapFog(t.Context(), connect.NewRequest(&mapsv1.SetMapFogRequest{CampaignId: campaign, MapId: fog.GetId(), FogEnabled: new(true)})); err != nil {
		t.Fatal(err)
	}
	if got := master.getMapImage(campaign, fog.GetId()); got != x {
		t.Fatalf("a fog map with an image of its own has %q, want %q kept", got, x)
	}
	master.start(campaign)
	fetch := func(id string) int { return ana.get("/images/" + id).status }

	// Shown: the session shows a copy.
	shown, err := master.play.SetShownImage(t.Context(), connect.NewRequest(&playv1.SetShownImageRequest{CampaignId: campaign, ImageId: x, Keep: true}))
	if err != nil {
		t.Fatal(err)
	}
	copyShown := shown.Msg.GetShownImage().GetId()
	if copyShown == "" || copyShown == x {
		t.Fatalf("the session shows %q, want a copy of %q", copyShown, x)
	}
	if fetch(copyShown) != http.StatusOK || fetch(x) != http.StatusNotFound {
		t.Errorf("the shown copy = %d, the fog map's image = %d; want 200 and 404", fetch(copyShown), fetch(x))
	}
	// Left: stop showing it, and the copy is what stays with the players.
	if _, err := master.play.SetShownImage(t.Context(), connect.NewRequest(&playv1.SetShownImageRequest{CampaignId: campaign})); err != nil {
		t.Fatal(err)
	}
	left, err := ana.play.ListLeftImages(t.Context(), connect.NewRequest(&playv1.ListLeftImagesRequest{CampaignId: campaign}))
	if err != nil || len(left.Msg.GetImages()) != 1 || left.Msg.GetImages()[0].GetId() != copyShown {
		t.Fatalf("the left images = %v, %v; want the copy %q", left, err, copyShown)
	}
	if fetch(copyShown) != http.StatusOK || fetch(x) != http.StatusNotFound {
		t.Errorf("after leaving: the copy = %d, the fog map's image = %d; want 200 and 404", fetch(copyShown), fetch(x))
	}
	// Reused for a map without fog: created, then updated.
	other := master.createMap(campaign, "Sem névoa", x)
	master.setMapRevealed(campaign, other.GetId(), true)
	otherImage := master.getMapImage(campaign, other.GetId())
	if otherImage == x || fetch(otherImage) != http.StatusOK {
		t.Errorf("a new map on the fog map's image has %q (fetch %d), want a copy that players fetch", otherImage, fetch(otherImage))
	}
	third := master.createMap(campaign, "Terceiro", master.newImage(campaign))
	master.setMapRevealed(campaign, third.GetId(), true)
	if _, err := master.maps.UpdateMap(t.Context(), connect.NewRequest(&mapsv1.UpdateMapRequest{
		CampaignId: campaign, MapId: third.GetId(), Revision: third.GetRevision(), ImageId: new(x),
	})); err != nil {
		t.Fatal(err)
	}
	if got := master.getMapImage(campaign, third.GetId()); got == x || fetch(got) != http.StatusOK {
		t.Errorf("a map moved to the fog map's image has %q (fetch %d), want a copy that players fetch", got, fetch(got))
	}
	// A portrait gets a copy too.
	npc := master.createNPC(campaign, "Mira", x)
	if got := npc.GetSheet().GetBasic().GetPortraitImageId(); got == "" || got == x {
		t.Errorf("an NPC's portrait on the fog map's image = %q, want a copy", got)
	}
	// The fog map keeps its own image through all of it.
	if got := master.getMapImage(campaign, fog.GetId()); got != x || fetch(x) != http.StatusNotFound {
		t.Errorf("the fog map's image = %q (fetch %d), want %q kept and refused", got, fetch(x), x)
	}
}

// The copy a fog map's image needs takes a place in the gallery: a full gallery
// refuses with resource_exhausted, and nothing is shown.
func TestRN10_AFogMapsImageCopyNeedsRoom(t *testing.T) {
	t.Parallel()
	h := newHarness(t, func(c *Config) { c.MaxImages = 1 })
	master := h.newUser("Mestre")
	campaign := h.newCampaign(master)
	x := master.newImage(campaign)
	fog := master.createMap(campaign, "Com névoa", x)
	master.mustSetGrid(campaign, fog.GetId(), 12)
	if _, err := master.maps.SetMapFog(t.Context(), connect.NewRequest(&mapsv1.SetMapFogRequest{CampaignId: campaign, MapId: fog.GetId(), FogEnabled: new(true)})); err != nil {
		t.Fatal(err)
	}
	master.start(campaign)
	_, err := master.play.SetShownImage(t.Context(), connect.NewRequest(&playv1.SetShownImageRequest{CampaignId: campaign, ImageId: x}))
	wantCode(t, "showing a fog map's image with a full gallery", err, connect.CodeResourceExhausted)
	_, err = master.maps.CreateMap(t.Context(), connect.NewRequest(&mapsv1.CreateMapRequest{CampaignId: campaign, Name: "Outro", ImageId: x}))
	wantCode(t, "a map on a fog map's image with a full gallery", err, connect.CodeResourceExhausted)
}
