package maps

import (
	"bytes"
	"slices"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The acceptance criteria of MR-008, MR-009, MR-012 (the map half) and
// MR-028, and RN-10, as far as the server goes
// (docs/product/stories.md). The screens get their Playwright tests with
// the maps UI.

// MR-008: the master puts points of interest of each kind on a map. A
// submap point leads to another map of the same campaign, never to its own
// map.
func TestMR008_MasterCreatesPointsOfEachKind(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master := h.newUser("Mestre")
	campaign := h.newCampaign(master)
	other := h.newCampaign(master)
	image := master.newImage(campaign)
	region := master.createMap(campaign, "Mirathel e arredores", image)
	tower := master.createMap(campaign, "Torre de Mirathel", image)
	foreign := master.createMap(other, "Outra mesa", master.newImage(other))
	if region.GetRevealed() || region.GetRevision() != 1 {
		t.Errorf("a new map = %v, want it hidden at revision 1", region)
	}

	point := func(kind mapsv1.MapPointKind, name, description, target string, x, y int32) *mapsv1.CreateMapPointRequest {
		return &mapsv1.CreateMapPointRequest{CampaignId: campaign, MapId: region.GetId(), Kind: kind, Name: name, Description: description, TargetMapId: target, XBp: x, YBp: y}
	}
	battle := master.createPoint(point(mapsv1.MapPointKind_MAP_POINT_KIND_BATTLE, "Emboscada na estrada", "Goblins atrás das pedras.", "", 3800, 6200))
	submap := master.createPoint(point(mapsv1.MapPointKind_MAP_POINT_KIND_SUBMAP, "Torre de Mirathel", "", tower.GetId(), 6900, 2700))
	scene := master.createPoint(point(mapsv1.MapPointKind_MAP_POINT_KIND_SCENE, "Taverna do Javali", "Onde a Velha Odra conta o que sabe, por um preço.", "", 5200, 4700))

	got := master.mustGetMap(campaign, region.GetId())
	if !slices.Equal(pointIDs(got), []string{battle.GetId(), submap.GetId(), scene.GetId()}) {
		t.Fatalf("the master's points = %v, want the three, oldest first", pointIDs(got))
	}
	for i, want := range []struct {
		kind mapsv1.MapPointKind
		name string
		x, y int32
	}{
		{mapsv1.MapPointKind_MAP_POINT_KIND_BATTLE, "Emboscada na estrada", 3800, 6200},
		{mapsv1.MapPointKind_MAP_POINT_KIND_SUBMAP, "Torre de Mirathel", 6900, 2700},
		{mapsv1.MapPointKind_MAP_POINT_KIND_SCENE, "Taverna do Javali", 5200, 4700},
	} {
		p := got.GetPoints()[i]
		if p.GetKind() != want.kind || p.GetName() != want.name || p.GetXBp() != want.x || p.GetYBp() != want.y || p.GetRevealed() {
			t.Errorf("point %d = %v, want a hidden %v %q at (%d, %d)", i, p, want.kind, want.name, want.x, want.y)
		}
	}
	if target := got.GetPoints()[1].GetTargetMap(); target.GetId() != tower.GetId() || target.GetName() != "Torre de Mirathel" {
		t.Errorf("the submap's target = %v, want the tower", target)
	}
	if got.GetMap().GetPointCount() != 3 {
		t.Errorf("point_count = %d, want 3", got.GetMap().GetPointCount())
	}
	// The tower now is a submap of the region ("Submapa de Mirathel e
	// arredores"); the region is a main map.
	for _, m := range master.listMaps(campaign) {
		var parents []string
		for _, p := range m.GetParentMaps() {
			parents = append(parents, p.GetName())
		}
		switch m.GetId() {
		case region.GetId():
			if len(parents) != 0 {
				t.Errorf("the region's parents = %v, want none", parents)
			}
		case tower.GetId():
			if !slices.Equal(parents, []string{"Mirathel e arredores"}) {
				t.Errorf("the tower's parents = %v, want the region", parents)
			}
		}
	}

	// A submap leads to another map of the same campaign, never to its own.
	for name, target := range map[string]string{
		"its own map":            region.GetId(),
		"another campaign's map": foreign.GetId(),
	} {
		_, err := master.maps.CreateMapPoint(t.Context(), connect.NewRequest(point(mapsv1.MapPointKind_MAP_POINT_KIND_SUBMAP, "Porta", "", target, 1, 1)))
		wantCode(t, "a submap leading to "+name, err, connect.CodeInvalidArgument)
		_, err = master.maps.UpdateMapPoint(t.Context(), connect.NewRequest(&mapsv1.UpdateMapPointRequest{CampaignId: campaign, MapId: region.GetId(), PointId: submap.GetId(), TargetMapId: proto.String(target)}))
		wantCode(t, "moving a submap's target to "+name, err, connect.CodeInvalidArgument)
	}

	// Moving is an update; a kind other than SUBMAP drops the target.
	res, err := master.maps.UpdateMapPoint(t.Context(), connect.NewRequest(&mapsv1.UpdateMapPointRequest{
		CampaignId: campaign, MapId: region.GetId(), PointId: submap.GetId(),
		XBp: proto.Int32(7000), YBp: proto.Int32(2500), Kind: mapsv1.MapPointKind_MAP_POINT_KIND_SCENE.Enum(),
	}))
	if err != nil {
		t.Fatalf("UpdateMapPoint(move, kind) error = %v", err)
	}
	if p := res.Msg.GetPoint(); p.GetXBp() != 7000 || p.GetYBp() != 2500 || p.GetKind() != mapsv1.MapPointKind_MAP_POINT_KIND_SCENE || p.GetTargetMap() != nil || p.GetName() != "Torre de Mirathel" {
		t.Errorf("UpdateMapPoint(move, kind) = %v; want the scene at (7000, 2500), same name, no target", p)
	}
}

// MR-009: a player never receives a hidden point. The map a player opens,
// read as the JSON the app gets, has neither the hidden point's ID, nor its
// name, nor its description. Nor does the live stream: a change to hidden
// things reaches only the master.
func TestMR009_PlayersNeverReceiveHiddenPoints(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, player := h.newUser("Mestre"), h.newUser("Jogadora")
	campaign := h.newCampaign(master, player)
	pc := player.createCharacter(campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Pensantus")
	m := master.createMap(campaign, "Mirathel e arredores", master.newImage(campaign))
	master.setMapRevealed(campaign, m.GetId(), true)
	shown := master.createPoint(&mapsv1.CreateMapPointRequest{
		CampaignId: campaign, MapId: m.GetId(), Kind: mapsv1.MapPointKind_MAP_POINT_KIND_SCENE,
		Name: "Taverna do Javali", Description: "Onde a Velha Odra conta o que sabe.", XBp: 5200, YBp: 4700,
	})
	master.setPointRevealed(campaign, shown, true)
	const hiddenName, hiddenText = "Covil do lich", "O lich dorme sob a torre."
	hidden := master.createPoint(&mapsv1.CreateMapPointRequest{
		CampaignId: campaign, MapId: m.GetId(), Kind: mapsv1.MapPointKind_MAP_POINT_KIND_BATTLE,
		Name: hiddenName, Description: hiddenText, XBp: 1000, YBp: 1000,
	})
	hiddenBits := []string{hidden.GetId(), hiddenName, hiddenText}
	noHiddenBits := func(what string, msg proto.Message) {
		t.Helper()
		raw, err := protojson.Marshal(msg)
		if err != nil {
			t.Fatalf("marshal %s: %v", what, err)
		}
		for _, bit := range hiddenBits {
			if bytes.Contains(raw, []byte(bit)) {
				t.Errorf("%s carries %q: %s", what, bit, raw)
			}
		}
	}

	got := player.mustGetMap(campaign, m.GetId())
	noHiddenBits("the player's GetMap", got)
	if !slices.Equal(pointIDs(got), []string{shown.GetId()}) || got.GetMap().GetPointCount() != 1 {
		t.Errorf("the player's points = %v (count %d), want only the revealed one", pointIDs(got), got.GetMap().GetPointCount())
	}
	noHiddenBits("the player's ListMaps", &mapsv1.ListMapsResponse{Maps: player.listMaps(campaign)})
	_, err := player.maps.UpdateMapPoint(t.Context(), connect.NewRequest(&mapsv1.UpdateMapPointRequest{CampaignId: campaign, MapId: m.GetId(), PointId: hidden.GetId(), Name: proto.String("X")}))
	wantCode(t, "a player changing the hidden point", err, connect.CodePermissionDenied)

	// The stream, during a session. The player's character stands on the
	// map from before it starts.
	master.placeToken(campaign, m.GetId(), pc.GetId(), 5000, 5000)
	master.start(campaign)
	mw, pw := master.watch(campaign), player.watch(campaign)
	var playerEvents []*playv1.WatchGameSessionResponse
	playerNext := func() *playv1.WatchGameSessionResponse {
		ev := pw.next()
		playerEvents = append(playerEvents, ev)
		return ev
	}

	// Changes to hidden things: the master hears about each one...
	ctx := t.Context()
	if _, err := master.maps.UpdateMapPoint(ctx, connect.NewRequest(&mapsv1.UpdateMapPointRequest{CampaignId: campaign, MapId: m.GetId(), PointId: hidden.GetId(), Description: proto.String("O lich acordou.")})); err != nil {
		t.Fatalf("UpdateMapPoint() error = %v", err)
	}
	mw.mapChanged(m.GetId())
	another := master.createPoint(&mapsv1.CreateMapPointRequest{CampaignId: campaign, MapId: m.GetId(), Kind: mapsv1.MapPointKind_MAP_POINT_KIND_SCENE, Name: "Ruínas élficas"})
	mw.mapChanged(m.GetId())
	if _, err := master.maps.DeleteMapPoint(ctx, connect.NewRequest(&mapsv1.DeleteMapPointRequest{CampaignId: campaign, MapId: m.GetId(), PointId: another.GetId()})); err != nil {
		t.Fatalf("DeleteMapPoint() error = %v", err)
	}
	mw.mapChanged(m.GetId())
	// ...and then a change the player sees: their token moves. The player's
	// first event is that move, so none of the hidden changes reached them.
	master.placeToken(campaign, m.GetId(), pc.GetId(), 5100, 5000)
	mw.tokenMoved(pc.GetId(), 5100, 5000)
	if ev := playerNext(); ev.GetTokenMoved().GetCharacterId() != pc.GetId() {
		t.Fatalf("the player's first event = %v, want the token's move: a hidden change reached them", ev)
	}

	// Revealing the point reaches the player (they see it after), and so
	// does hiding it again (they saw it before).
	master.setPointRevealed(campaign, hidden, true)
	mw.mapChanged(m.GetId())
	if ev := playerNext(); ev.GetMapChanged().GetMapId() != m.GetId() {
		t.Fatalf("event after the reveal = %v, want map_changed", ev)
	}
	if got := pointIDs(player.mustGetMap(campaign, m.GetId())); !slices.Contains(got, hidden.GetId()) {
		t.Errorf("after the reveal, the player's points = %v, want the point", got)
	}
	master.setPointRevealed(campaign, hidden, false)
	mw.mapChanged(m.GetId())
	if ev := playerNext(); ev.GetMapChanged().GetMapId() != m.GetId() {
		t.Fatalf("event after hiding again = %v, want map_changed", ev)
	}
	got = player.mustGetMap(campaign, m.GetId())
	noHiddenBits("the player's GetMap after hiding again", got)
	for i, ev := range playerEvents {
		noHiddenBits("the player's stream event "+string(rune('0'+i)), ev)
	}
}

// RN-10: a player cannot open a hidden map. It is not_found, exactly as a
// map that does not exist, and ListMaps leaves it out. Once revealed, or
// made the session's current map, the player sees it. A submap point leads
// a player to its target only while they see the target.
func TestRN10_PlayersCannotOpenHiddenMaps(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, player := h.newUser("Mestre"), h.newUser("Jogadora")
	campaign := h.newCampaign(master, player)
	image := master.newImage(campaign)
	region := master.createMap(campaign, "Mirathel e arredores", image)
	tower := master.createMap(campaign, "Torre de Mirathel", image)
	master.setMapRevealed(campaign, region.GetId(), true)
	stairs := master.createPoint(&mapsv1.CreateMapPointRequest{
		CampaignId: campaign, MapId: region.GetId(), Kind: mapsv1.MapPointKind_MAP_POINT_KIND_SUBMAP,
		Name: "Torre", TargetMapId: tower.GetId(), XBp: 6900, YBp: 2700,
	})
	master.setPointRevealed(campaign, stairs, true)

	// Hidden: not_found, like a map that does not exist, and not listed.
	_, err := player.getMap(campaign, tower.GetId())
	wantCode(t, "GetMap of a hidden map", err, connect.CodeNotFound)
	_, err = player.getMap(campaign, "6f1c7a52-3b5e-4c55-9d0b-2a51f0c1e001")
	wantCode(t, "GetMap of a map that does not exist", err, connect.CodeNotFound)
	if got := mapIDs(player.listMaps(campaign)); !slices.Equal(got, []string{region.GetId()}) {
		t.Errorf("the player's maps = %v, want only the revealed one", got)
	}
	// The submap point is there, but leads nowhere for the player: the
	// tower's ID and name stay on the server.
	view := player.mustGetMap(campaign, region.GetId())
	if len(view.GetPoints()) != 1 || view.GetPoints()[0].GetTargetMap() != nil {
		t.Errorf("the player's submap point = %v, want it without a target", view.GetPoints())
	}
	if raw, _ := protojson.Marshal(view); bytes.Contains(raw, []byte(tower.GetId())) {
		t.Errorf("the player's map carries the hidden tower's ID: %s", raw)
	}

	// Revealed: the player sees it, and the point leads there.
	master.setMapRevealed(campaign, tower.GetId(), true)
	if got := player.mustGetMap(campaign, tower.GetId()).GetMap(); got.GetId() != tower.GetId() || len(got.GetParentMaps()) != 1 {
		t.Errorf("the revealed tower = %v, want it, with the region as its parent", got)
	}
	if target := player.mustGetMap(campaign, region.GetId()).GetPoints()[0].GetTargetMap(); target.GetId() != tower.GetId() || target.GetName() != "Torre de Mirathel" {
		t.Errorf("the submap's target for the player = %v, want the tower", target)
	}
	// A hidden point hides the link: the tower has no parent for the
	// player.
	master.setPointRevealed(campaign, stairs, false)
	if got := player.mustGetMap(campaign, tower.GetId()).GetMap().GetParentMaps(); len(got) != 0 {
		t.Errorf("the tower's parents for the player, through a hidden point = %v, want none", got)
	}
	master.setMapRevealed(campaign, tower.GetId(), false)
	_, err = player.getMap(campaign, tower.GetId())
	wantCode(t, "GetMap of a map hidden again", err, connect.CodeNotFound)

	// The current map: setting it reveals it; while it is current, the
	// player sees it even if the master hides it again.
	master.start(campaign)
	if _, err := master.setCurrentMap(campaign, tower.GetId()); err != nil {
		t.Fatalf("SetCurrentMap() error = %v", err)
	}
	master.setMapRevealed(campaign, tower.GetId(), false)
	if got := player.mustGetMap(campaign, tower.GetId()).GetMap(); got.GetRevealed() || !got.GetCurrent() {
		t.Errorf("the current map hidden again = %v, want the player to see it, hidden but current", got)
	}
	if got := mapIDs(player.listMaps(campaign)); !slices.Contains(got, tower.GetId()) {
		t.Errorf("the player's maps = %v, want the current one", got)
	}
	if _, err := master.setCurrentMap(campaign, ""); err != nil {
		t.Fatalf("SetCurrentMap(\"\") error = %v", err)
	}
	_, err = player.getMap(campaign, tower.GetId())
	wantCode(t, "GetMap of a hidden map that is no longer current", err, connect.CodeNotFound)

	// Someone outside the campaign sees nothing, revealed or not.
	outsider := h.newUser("De fora")
	_, err = outsider.getMap(campaign, region.GetId())
	wantCode(t, "GetMap as a non-member", err, connect.CodeNotFound)
}

// MR-012, the map half: when the master moves a visible token during a
// session, the players see it move without reloading. A hidden NPC token's
// moves reach only the master.
func TestMR012_TokenMovesReachPlayersLive(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, player := h.newUser("Mestre"), h.newUser("Jogadora")
	campaign := h.newCampaign(master, player)
	pc := player.createCharacter(campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Pensantus")
	goblin := master.createCharacter(campaign, charactersv1.CharacterKind_CHARACTER_KIND_ENEMY, "Capitão Goblin")
	m := master.createMap(campaign, "Mirathel e arredores", master.newImage(campaign))
	master.setMapRevealed(campaign, m.GetId(), true)
	master.placeToken(campaign, m.GetId(), pc.GetId(), 5200, 5200)
	master.placeToken(campaign, m.GetId(), goblin.GetId(), 3700, 6000) // hidden: an NPC

	master.start(campaign)
	mw, pw := master.watch(campaign), player.watch(campaign)

	// The hidden NPC moves: only the master hears.
	master.placeToken(campaign, m.GetId(), goblin.GetId(), 3800, 6100)
	mw.tokenMoved(goblin.GetId(), 3800, 6100)
	// The player's character moves: everyone hears, and the player's first
	// event is this move, not the NPC's.
	master.placeToken(campaign, m.GetId(), pc.GetId(), 5400, 5300)
	mw.tokenMoved(pc.GetId(), 5400, 5300)
	pw.tokenMoved(pc.GetId(), 5400, 5300)

	view := player.mustGetMap(campaign, m.GetId())
	if !slices.Equal(tokenIDs(view), []string{pc.GetId()}) {
		t.Fatalf("the player's tokens = %v, want only their character", tokenIDs(view))
	}
	if tk := view.GetTokens()[0]; !tk.GetMine() || tk.GetName() != "Pensantus" || tk.GetXBp() != 5400 || tk.GetYBp() != 5300 {
		t.Errorf("the player's token = %v, want theirs, at the new position", tk)
	}

	// The master reveals the NPC: the player hears, and then its moves.
	master.setTokenHidden(campaign, m.GetId(), goblin.GetId(), false)
	mw.mapChanged(m.GetId())
	pw.mapChanged(m.GetId())
	master.placeToken(campaign, m.GetId(), goblin.GetId(), 4000, 6000)
	mw.tokenMoved(goblin.GetId(), 4000, 6000)
	pw.tokenMoved(goblin.GetId(), 4000, 6000)
	if got := tokenIDs(player.mustGetMap(campaign, m.GetId())); !slices.Equal(got, []string{pc.GetId(), goblin.GetId()}) {
		t.Errorf("the player's tokens after the reveal = %v, want both", got)
	}

	// On a map hidden from the players, even a visible token's moves reach
	// only the master.
	hiddenMap := master.createMap(campaign, "Covil dos goblins", master.newImage(campaign))
	mw.mapChanged(hiddenMap.GetId())
	master.placeToken(campaign, hiddenMap.GetId(), pc.GetId(), 100, 100)
	mw.mapChanged(hiddenMap.GetId())
	master.placeToken(campaign, hiddenMap.GetId(), pc.GetId(), 200, 200)
	mw.tokenMoved(pc.GetId(), 200, 200)
	master.placeToken(campaign, m.GetId(), pc.GetId(), 5500, 5300)
	pw.tokenMoved(pc.GetId(), 5500, 5300) // the first thing the player hears
}

// MR-028: the master shows the players an image of the gallery during the
// session, apart from the current map, and stops showing it. It reveals
// nothing else.
func TestMR028_MasterShowsAnImageToThePlayers(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, player := h.newUser("Mestre"), h.newUser("Jogadora")
	campaign := h.newCampaign(master, player)
	other := h.newCampaign(master)
	portrait := master.mustUpload(campaign, "Velha Odra.png", pngImage(t, 60, 80))
	foreign := master.newImage(other)
	m := master.createMap(campaign, "Mirathel e arredores", master.newImage(campaign))
	show := func(u *user, imageID string) (*playv1.ShownImage, error) {
		res, err := u.play.SetShownImage(t.Context(), connect.NewRequest(&playv1.SetShownImageRequest{CampaignId: campaign, ImageId: imageID}))
		if err != nil {
			return nil, err
		}
		return res.Msg.GetShownImage(), nil
	}
	shownNow := func(u *user) *playv1.ShownImage {
		t.Helper()
		res, err := u.play.GetLiveSession(t.Context(), connect.NewRequest(&playv1.GetLiveSessionRequest{CampaignId: campaign}))
		if err != nil {
			t.Fatalf("GetLiveSession() error = %v", err)
		}
		return res.Msg.GetShownImage()
	}

	// Outside a session: NO_OPEN_SESSION.
	_, err := show(master, portrait.GetId())
	wantBlocked(t, "SetShownImage without a session", err, playv1.GameSessionBlockedReason_GAME_SESSION_BLOCKED_REASON_NO_OPEN_SESSION)

	master.start(campaign)
	if _, err := master.setCurrentMap(campaign, m.GetId()); err != nil {
		t.Fatalf("SetCurrentMap() error = %v", err)
	}
	pw := player.watch(campaign)

	_, err = show(player, portrait.GetId())
	wantCode(t, "SetShownImage as a player", err, connect.CodePermissionDenied)
	_, err = show(master, foreign)
	wantCode(t, "SetShownImage of another campaign's image", err, connect.CodeNotFound)
	_, err = show(master, "imagem")
	wantCode(t, "SetShownImage of an ID that is no UUID", err, connect.CodeNotFound)

	// Show: the player hears at once, and the snapshot has it, next to the
	// current map.
	shown, err := show(master, portrait.GetId())
	want := &playv1.ShownImage{Id: portrait.GetId(), Name: "Velha Odra", Width: 60, Height: 80, Url: portrait.GetUrl(), ThumbnailUrl: portrait.GetThumbnailUrl()}
	if err != nil || !proto.Equal(shown, want) {
		t.Fatalf("SetShownImage() = %v, %v; want %v", shown, err, want)
	}
	if ev := pw.next().GetShownImageChanged(); !proto.Equal(ev.GetImage(), want) {
		t.Errorf("the player's event = %v, want shown_image_changed with the portrait", ev)
	}
	if got := shownNow(player); !proto.Equal(got, want) {
		t.Errorf("the player's GetLiveSession shown_image = %v, want the portrait", got)
	}
	if got := player.liveCurrentMap(campaign); got != m.GetId() {
		t.Errorf("the current map while an image is shown = %q, want the map, still there", got)
	}
	if res := player.get(portrait.GetUrl()); res.status != 200 {
		t.Errorf("the player fetching the shown image: status %d, want 200", res.status)
	}

	// Stop: the player hears, and the snapshot no longer has it.
	if shown, err := show(master, ""); err != nil || shown != nil {
		t.Errorf("SetShownImage(\"\") = %v, %v; want nothing shown", shown, err)
	}
	if ev := pw.next(); ev.GetShownImageChanged() == nil || ev.GetShownImageChanged().GetImage() != nil {
		t.Errorf("the player's event = %v, want shown_image_changed with no image", ev)
	}
	if got := shownNow(player); got != nil {
		t.Errorf("after stopping, shown_image = %v, want none", got)
	}
	// The player kept the ID, but it opens nothing anymore (question 32).
	if res := player.get(portrait.GetUrl()); res.status != 404 {
		t.Errorf("the player fetching the image after it stopped being shown: status %d, want 404", res.status)
	}

	// Deleting the shown image stops showing it, for everyone.
	if _, err := show(master, portrait.GetId()); err != nil {
		t.Fatalf("SetShownImage() error = %v", err)
	}
	pw.next()
	if _, err := master.gallery.DeleteGalleryImage(t.Context(), connect.NewRequest(&mapsv1.DeleteGalleryImageRequest{CampaignId: campaign, ImageId: portrait.GetId()})); err != nil {
		t.Fatalf("DeleteGalleryImage(the shown image) error = %v", err)
	}
	if ev := pw.next(); ev.GetShownImageChanged() == nil || ev.GetShownImageChanged().GetImage() != nil {
		t.Errorf("the player's event after the delete = %v, want shown_image_changed with no image", ev)
	}
	if got := shownNow(player); got != nil {
		t.Errorf("after deleting the image, shown_image = %v, want none", got)
	}
}

// MR-028: the master leaves a shown image with the players ("Deixar com os
// jogadores"). It moves to the campaign's left list when the show stops or
// changes, and when the session ends; the players keep it until the master
// takes it back, and the list is the campaign's, not the session's.
func TestMR028_MasterLeavesAnImageWithThePlayers(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, player, stranger := h.newUser("Mestre"), h.newUser("Jogadora"), h.newUser("De fora")
	campaign := h.newCampaign(master, player)
	letter := master.mustUpload(campaign, "Carta.png", pngImage(t, 60, 80))
	portrait := master.mustUpload(campaign, "Velha Odra.png", pngImage(t, 60, 80))
	tower := master.mustUpload(campaign, "Torre.png", pngImage(t, 60, 80))
	show := func(imageID string, keep bool) *playv1.SetShownImageResponse {
		t.Helper()
		res, err := master.play.SetShownImage(t.Context(), connect.NewRequest(&playv1.SetShownImageRequest{CampaignId: campaign, ImageId: imageID, Keep: keep}))
		if err != nil {
			t.Fatalf("SetShownImage(%q, keep=%v) error = %v", imageID, keep, err)
		}
		return res.Msg
	}
	left := func(u *user) []string {
		t.Helper()
		res, err := u.play.ListLeftImages(t.Context(), connect.NewRequest(&playv1.ListLeftImagesRequest{CampaignId: campaign}))
		if err != nil {
			t.Fatalf("ListLeftImages() error = %v", err)
		}
		var ids []string
		for _, img := range res.Msg.GetImages() {
			ids = append(ids, img.GetId())
		}
		return ids
	}
	keepNow := func(u *user) bool {
		t.Helper()
		res, err := u.play.GetLiveSession(t.Context(), connect.NewRequest(&playv1.GetLiveSessionRequest{CampaignId: campaign}))
		if err != nil {
			t.Fatalf("GetLiveSession() error = %v", err)
		}
		return res.Msg.GetShownImageKeep()
	}
	wantLeft := func(when string, want ...string) {
		t.Helper()
		for _, u := range []*user{master, player} {
			if got := left(u); !slices.Equal(got, want) {
				t.Errorf("%s: left images = %v, want %v", when, got, want)
			}
		}
	}

	master.start(campaign)
	pw := player.watch(campaign)
	wantLeft("at the start")

	// The switch is off by default: stopping takes the image away.
	if res := show(letter.GetId(), false); res.GetKeep() {
		t.Error("keep = true, want false")
	}
	pw.next()
	show("", false)
	pw.next()
	wantLeft("an image shown without keep")

	// On: the switch is the master's, and stopping leaves the image.
	show(letter.GetId(), false)
	pw.next()
	if res := show(letter.GetId(), true); !res.GetKeep() {
		t.Error("keep = false, want true")
	}
	// Only the switch moved: the players hear nothing (the next event is the stop's).
	if !keepNow(master) || keepNow(player) {
		t.Errorf("GetLiveSession shown_image_keep: master %v, player %v; want true, false", keepNow(master), keepNow(player))
	}
	wantLeft("while it is still shown")
	show("", false)
	if ev := pw.next(); ev.GetShownImageChanged() == nil {
		t.Errorf("event = %v, want shown_image_changed", ev)
	}
	if ev := pw.next(); ev.GetLeftImagesChanged() == nil {
		t.Errorf("event = %v, want left_images_changed", ev)
	}
	wantLeft("after stopping with keep on", letter.GetId())
	if res := player.get(letter.GetUrl()); res.status != 200 {
		t.Errorf("the player fetching a left image: status %d, want 200", res.status)
	}
	if res := player.get(letter.GetThumbnailUrl()); res.status != 200 {
		t.Errorf("the player fetching a left image's thumbnail: status %d, want 200", res.status)
	}
	if res := stranger.get(letter.GetUrl()); res.status != 404 {
		t.Errorf("a non-member fetching a left image: status %d, want 404", res.status)
	}

	// Replacing a kept image leaves it too, and the new one starts off.
	show(portrait.GetId(), true)
	pw.next()
	if res := show(tower.GetId(), false); res.GetKeep() {
		t.Error("keep for the next image = true, want false")
	}
	pw.next()
	if ev := pw.next(); ev.GetLeftImagesChanged() == nil {
		t.Errorf("event = %v, want left_images_changed", ev)
	}
	wantLeft("after replacing a kept image", letter.GetId(), portrait.GetId())

	// Ending the session leaves a kept image, and the list outlives it.
	show(tower.GetId(), true)
	live, err := master.play.GetLiveSession(t.Context(), connect.NewRequest(&playv1.GetLiveSessionRequest{CampaignId: campaign}))
	if err != nil {
		t.Fatalf("GetLiveSession() error = %v", err)
	}
	session := live.Msg.GetGameSession()
	if _, err := master.play.EndGameSession(t.Context(), connect.NewRequest(&playv1.EndGameSessionRequest{CampaignId: campaign, GameSessionId: session.GetId()})); err != nil {
		t.Fatalf("EndGameSession() error = %v", err)
	}
	wantLeft("after the session ended", letter.GetId(), portrait.GetId(), tower.GetId())
	if res := player.get(tower.GetUrl()); res.status != 200 {
		t.Errorf("the player fetching a left image after the session: status %d, want 200", res.status)
	}

	// Taking back: only the master, and the player loses the image.
	take := func(u *user, imageID string) error {
		_, err := u.play.TakeBackLeftImage(t.Context(), connect.NewRequest(&playv1.TakeBackLeftImageRequest{CampaignId: campaign, ImageId: imageID}))
		return err
	}
	wantCode(t, "TakeBackLeftImage as a player", take(player, letter.GetId()), connect.CodePermissionDenied)
	if err := take(master, letter.GetId()); err != nil {
		t.Fatalf("TakeBackLeftImage() error = %v", err)
	}
	wantCode(t, "TakeBackLeftImage twice", take(master, letter.GetId()), connect.CodeNotFound)
	wantCode(t, "TakeBackLeftImage of an image not left", take(master, "imagem"), connect.CodeNotFound)
	wantLeft("after taking one back", portrait.GetId(), tower.GetId())
	if res := player.get(letter.GetUrl()); res.status != 404 {
		t.Errorf("the player fetching an image taken back: status %d, want 404", res.status)
	}
	if res := master.get(letter.GetUrl()); res.status != 200 {
		t.Errorf("the master fetching an image taken back: status %d, want 200 (it is still in the gallery)", res.status)
	}

	// Deleting a left image from the gallery removes it from the list.
	if _, err := master.gallery.DeleteGalleryImage(t.Context(), connect.NewRequest(&mapsv1.DeleteGalleryImageRequest{CampaignId: campaign, ImageId: portrait.GetId()})); err != nil {
		t.Fatalf("DeleteGalleryImage() error = %v", err)
	}
	wantLeft("after deleting a left image", tower.GetId())
}

// RN-10, for the image files: the master fetches every image of the
// campaign; a player only an image they see now, the background of a map
// they see or the image the master shows. Knowing its ID is not enough, and
// the player's browser checks again on every use (no-cache), so a map
// hidden again, or an image no longer shown, stops being served.
func TestRN10_PlayersOnlyFetchImagesTheyCanSee(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, player := h.newUser("Mestre"), h.newUser("Jogadora")
	campaign := h.newCampaign(master, player)
	upload := func(name string) *mapsv1.GalleryImage { return master.mustUpload(campaign, name, pngImage(t, 40, 30)) }
	background, handout, unused := upload("mirathel.png"), upload("carta.png"), upload("rascunho.png")
	m := master.createMap(campaign, "Mirathel e arredores", background.GetId())

	type fetch struct {
		who    *user
		img    *mapsv1.GalleryImage
		status int
	}
	check := func(when string, fetches ...fetch) {
		t.Helper()
		for _, f := range fetches {
			for _, path := range []string{f.img.GetUrl(), f.img.GetThumbnailUrl()} {
				res := f.who.get(path)
				if res.status != f.status {
					t.Errorf("%s: GET %s as %s: status %d, want %d", when, path, f.who.id, res.status, f.status)
					continue
				}
				if res.status != 200 {
					continue
				}
				wantCache := "private, max-age=31536000, immutable"
				if f.who == player {
					wantCache = "private, no-cache"
				}
				if got := res.header.Get("Cache-Control"); got != wantCache {
					t.Errorf("%s: GET %s Cache-Control = %q, want %q", when, path, got, wantCache)
				}
				if got := res.header.Get("Vary"); got != "Cookie" {
					t.Errorf("%s: GET %s Vary = %q, want Cookie", when, path, got)
				}
			}
		}
	}

	// The master sees every image; the player none yet: the map is hidden.
	check("the map hidden",
		fetch{master, background, 200}, fetch{master, handout, 200}, fetch{master, unused, 200},
		fetch{player, background, 404}, fetch{player, handout, 404}, fetch{player, unused, 404})

	// Revealed: the player fetches its image, and revalidates cheaply.
	master.setMapRevealed(campaign, m.GetId(), true)
	check("the map revealed", fetch{player, background, 200}, fetch{player, unused, 404})
	etag := `"` + background.GetId() + `"`
	if res := player.get(background.GetUrl(), "If-None-Match", etag); res.status != 304 {
		t.Errorf("revalidating a visible image: status %d, want 304", res.status)
	}

	// Hidden again: the ID the player kept opens nothing, not even a 304.
	master.setMapRevealed(campaign, m.GetId(), false)
	check("the map hidden again", fetch{player, background, 404})
	if res := player.get(background.GetUrl(), "If-None-Match", etag); res.status != 404 {
		t.Errorf("revalidating an image no longer visible: status %d, want 404", res.status)
	}

	// The current map shows its image even while hidden.
	master.start(campaign)
	if _, err := master.setCurrentMap(campaign, m.GetId()); err != nil {
		t.Fatalf("SetCurrentMap() error = %v", err)
	}
	master.setMapRevealed(campaign, m.GetId(), false)
	check("the hidden current map", fetch{player, background, 200})
	if _, err := master.setCurrentMap(campaign, ""); err != nil {
		t.Fatalf("SetCurrentMap(\"\") error = %v", err)
	}
	check("no current map", fetch{player, background, 404})

	// The shown image, while it is shown.
	showImage := func(imageID string) {
		t.Helper()
		if _, err := master.play.SetShownImage(t.Context(), connect.NewRequest(&playv1.SetShownImageRequest{CampaignId: campaign, ImageId: imageID})); err != nil {
			t.Fatalf("SetShownImage(%q) error = %v", imageID, err)
		}
	}
	showImage(handout.GetId())
	check("the image shown", fetch{player, handout, 200}, fetch{player, background, 404})
	showImage("")
	check("the image no longer shown", fetch{player, handout, 404}, fetch{master, handout, 200})

	// A left image (MR-028) is served until the master takes it back.
	if _, err := master.play.SetShownImage(t.Context(), connect.NewRequest(&playv1.SetShownImageRequest{CampaignId: campaign, ImageId: handout.GetId(), Keep: true})); err != nil {
		t.Fatalf("SetShownImage(keep) error = %v", err)
	}
	showImage("")
	check("the image left", fetch{player, handout, 200}, fetch{player, unused, 404})
	if _, err := master.play.TakeBackLeftImage(t.Context(), connect.NewRequest(&playv1.TakeBackLeftImageRequest{CampaignId: campaign, ImageId: handout.GetId()})); err != nil {
		t.Fatalf("TakeBackLeftImage() error = %v", err)
	}
	check("the image taken back", fetch{player, handout, 404}, fetch{master, handout, 200})
}
