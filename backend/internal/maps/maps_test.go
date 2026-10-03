package maps

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1/playv1connect"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// The maps (MR-008, MR-009, MR-012; RN-10). The stories' own criteria are
// in stories_test.go; these tests cover the rest of MapService, the
// session's current map (PlayService.SetCurrentMap) and the gallery's side
// of maps. They need the database (MEURPG_TEST_DATABASE_URL).

// Helpers: each one calls the API as u, and fails the test on an error.

func (u *user) newImage(campaignID string) string {
	u.h.t.Helper()
	return u.mustUpload(campaignID, "mapa.png", pngImage(u.h.t, 40, 30)).GetId()
}

func (u *user) createMap(campaignID, name, imageID string) *mapsv1.Map {
	u.h.t.Helper()
	res, err := u.maps.CreateMap(u.h.t.Context(), connect.NewRequest(&mapsv1.CreateMapRequest{CampaignId: campaignID, Name: name, ImageId: imageID}))
	if err != nil {
		u.h.t.Fatalf("CreateMap(%q) error = %v", name, err)
	}
	return res.Msg.GetMap()
}

func (u *user) getMap(campaignID, mapID string) (*mapsv1.GetMapResponse, error) {
	res, err := u.maps.GetMap(u.h.t.Context(), connect.NewRequest(&mapsv1.GetMapRequest{CampaignId: campaignID, MapId: mapID}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func (u *user) mustGetMap(campaignID, mapID string) *mapsv1.GetMapResponse {
	u.h.t.Helper()
	res, err := u.getMap(campaignID, mapID)
	if err != nil {
		u.h.t.Fatalf("GetMap() error = %v", err)
	}
	return res
}

func (u *user) listMaps(campaignID string) []*mapsv1.Map {
	u.h.t.Helper()
	res, err := u.maps.ListMaps(u.h.t.Context(), connect.NewRequest(&mapsv1.ListMapsRequest{CampaignId: campaignID}))
	if err != nil {
		u.h.t.Fatalf("ListMaps() error = %v", err)
	}
	return res.Msg.GetMaps()
}

func (u *user) setMapRevealed(campaignID, mapID string, revealed bool) *mapsv1.Map {
	u.h.t.Helper()
	res, err := u.maps.SetMapRevealed(u.h.t.Context(), connect.NewRequest(&mapsv1.SetMapRevealedRequest{CampaignId: campaignID, MapId: mapID, Revealed: revealed}))
	if err != nil {
		u.h.t.Fatalf("SetMapRevealed(%v) error = %v", revealed, err)
	}
	return res.Msg.GetMap()
}

func (u *user) createPoint(req *mapsv1.CreateMapPointRequest) *mapsv1.MapPoint {
	u.h.t.Helper()
	res, err := u.maps.CreateMapPoint(u.h.t.Context(), connect.NewRequest(req))
	if err != nil {
		u.h.t.Fatalf("CreateMapPoint(%q) error = %v", req.GetName(), err)
	}
	return res.Msg.GetPoint()
}

func (u *user) setPointRevealed(campaignID string, p *mapsv1.MapPoint, revealed bool) *mapsv1.MapPoint {
	u.h.t.Helper()
	res, err := u.maps.SetMapPointRevealed(u.h.t.Context(), connect.NewRequest(&mapsv1.SetMapPointRevealedRequest{
		CampaignId: campaignID, MapId: p.GetMapId(), PointId: p.GetId(), Revealed: revealed,
	}))
	if err != nil {
		u.h.t.Fatalf("SetMapPointRevealed(%v) error = %v", revealed, err)
	}
	return res.Msg.GetPoint()
}

func (u *user) placeToken(campaignID, mapID, characterID string, x, y int32) *mapsv1.MapToken {
	u.h.t.Helper()
	res, err := u.maps.PlaceMapToken(u.h.t.Context(), connect.NewRequest(&mapsv1.PlaceMapTokenRequest{
		CampaignId: campaignID, MapId: mapID, CharacterId: characterID, XBp: x, YBp: y,
	}))
	if err != nil {
		u.h.t.Fatalf("PlaceMapToken() error = %v", err)
	}
	return res.Msg.GetToken()
}

func (u *user) setTokenHidden(campaignID, mapID, characterID string, hidden bool) *mapsv1.MapToken {
	u.h.t.Helper()
	res, err := u.maps.SetMapTokenHidden(u.h.t.Context(), connect.NewRequest(&mapsv1.SetMapTokenHiddenRequest{
		CampaignId: campaignID, MapId: mapID, CharacterId: characterID, Hidden: hidden,
	}))
	if err != nil {
		u.h.t.Fatalf("SetMapTokenHidden(%v) error = %v", hidden, err)
	}
	return res.Msg.GetToken()
}

// createCharacter creates a level 1 character as u: a full sheet, or a
// basic one for a minion.
func (u *user) createCharacter(campaignID string, kind charactersv1.CharacterKind, name string) *charactersv1.Character {
	u.h.t.Helper()
	sheet := &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
		BaseScores: &rulesv1.AbilityScores{Strength: 10, Dexterity: 14, Constitution: 12, Intelligence: 16, Wisdom: 10, Charisma: 8},
		RaceKey:    "race:gnome",
		Classes:    []*charactersv1.ClassLevel{{ClassKey: "class:wizard", Level: 1}},
	}}}
	if kind == charactersv1.CharacterKind_CHARACTER_KIND_MINION {
		sheet = &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Basic{Basic: &charactersv1.BasicSheet{HitPointsMax: 7, ArmorClass: 12, SpeedFt: 30}}}
	}
	res, err := u.characters.CreateCharacter(u.h.t.Context(), connect.NewRequest(&charactersv1.CreateCharacterRequest{CampaignId: campaignID, Kind: kind, Name: name, Sheet: sheet}))
	if err != nil {
		u.h.t.Fatalf("CreateCharacter(%v) error = %v", kind, err)
	}
	return res.Msg.GetCharacter()
}

// start opens a game session as u.
func (u *user) start(campaignID string) *playv1.GameSession {
	u.h.t.Helper()
	res, err := u.play.StartGameSession(u.h.t.Context(), connect.NewRequest(&playv1.StartGameSessionRequest{CampaignId: campaignID}))
	if err != nil {
		u.h.t.Fatalf("StartGameSession() error = %v", err)
	}
	return res.Msg.GetGameSession()
}

func (u *user) setCurrentMap(campaignID, mapID string) (string, error) {
	res, err := u.play.SetCurrentMap(u.h.t.Context(), connect.NewRequest(&playv1.SetCurrentMapRequest{CampaignId: campaignID, MapId: mapID}))
	if err != nil {
		return "", err
	}
	return res.Msg.GetCurrentMapId(), nil
}

func (u *user) liveCurrentMap(campaignID string) string {
	u.h.t.Helper()
	res, err := u.play.GetLiveSession(u.h.t.Context(), connect.NewRequest(&playv1.GetLiveSessionRequest{CampaignId: campaignID}))
	if err != nil {
		u.h.t.Fatalf("GetLiveSession() error = %v", err)
	}
	return res.Msg.GetCurrentMapId()
}

// waitLimit bounds every wait on a stream, so a broken stream fails the
// test instead of hanging it.
const waitLimit = 5 * time.Second

// watcher reads one WatchGameSession stream in the background.
type watcher struct {
	t      *testing.T
	events chan *playv1.WatchGameSessionResponse
}

// watch opens WatchGameSession as u and reads its `ready`, so every change
// made after watch returns reaches the stream.
func (u *user) watch(campaignID string) *watcher {
	u.h.t.Helper()
	return watchWith(u.h.t, u.play, campaignID)
}

func watchWith(t *testing.T, client playv1connect.PlayServiceClient, campaignID string) *watcher {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	stream, err := client.WatchGameSession(ctx, connect.NewRequest(&playv1.WatchGameSessionRequest{CampaignId: campaignID}))
	if err != nil {
		t.Fatalf("WatchGameSession() error = %v", err)
	}
	w := &watcher{t: t, events: make(chan *playv1.WatchGameSessionResponse, 64)}
	go func() {
		defer close(w.events)
		defer func() { _ = stream.Close() }()
		for stream.Receive() {
			w.events <- stream.Msg()
		}
	}()
	if ev := w.next(); ev.GetReady() == nil {
		t.Fatalf("first event = %v, want ready", ev)
	}
	return w
}

// next returns the stream's next event that is not a heartbeat.
func (w *watcher) next() *playv1.WatchGameSessionResponse {
	w.t.Helper()
	timeout := time.After(waitLimit)
	for {
		select {
		case ev, ok := <-w.events:
			if !ok {
				w.t.Fatal("the stream ended, want another event")
			}
			if ev.GetHeartbeat() == nil {
				return ev
			}
		case <-timeout:
			w.t.Fatal("no event within the time limit")
		}
	}
}

// mapChanged reads the next event, which must be map_changed for mapID.
func (w *watcher) mapChanged(mapID string) {
	w.t.Helper()
	if ev := w.next(); ev.GetMapChanged().GetMapId() != mapID {
		w.t.Fatalf("event = %v, want map_changed for %s", ev, mapID)
	}
}

// tokenMoved reads the next event, which must be token_moved.
func (w *watcher) tokenMoved(characterID string, x, y int32) {
	w.t.Helper()
	ev := w.next().GetTokenMoved()
	if ev.GetCharacterId() != characterID || ev.GetXBp() != x || ev.GetYBp() != y {
		w.t.Fatalf("event = %v, want token_moved of %s to (%d, %d)", ev, characterID, x, y)
	}
}

// currentMapChanged reads the next event, which must be
// current_map_changed to mapID.
func (w *watcher) currentMapChanged(mapID string) {
	w.t.Helper()
	ev := w.next()
	if ev.GetCurrentMapChanged() == nil || ev.GetCurrentMapChanged().GetMapId() != mapID {
		w.t.Fatalf("event = %v, want current_map_changed to %q", ev, mapID)
	}
}

// pointIDs lists the IDs of a map's points.
func pointIDs(res *mapsv1.GetMapResponse) []string {
	var ids []string
	for _, p := range res.GetPoints() {
		ids = append(ids, p.GetId())
	}
	return ids
}

// tokenIDs lists the characters of a map's tokens.
func tokenIDs(res *mapsv1.GetMapResponse) []string {
	var ids []string
	for _, tk := range res.GetTokens() {
		ids = append(ids, tk.GetCharacterId())
	}
	return ids
}

func mapIDs(maps []*mapsv1.Map) []string {
	var ids []string
	for _, m := range maps {
		ids = append(ids, m.GetId())
	}
	return ids
}

// TestMapServiceAuthorizationMatrix calls every MapService method as each
// kind of caller (ADR-0011): only the master changes maps; any member reads
// them (filtered); a non-member and a pending member (RN-15) get not_found;
// anonymous, unauthenticated.
func TestMapServiceAuthorizationMatrix(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, player, pending := h.newUser("Mestre"), h.newUser("Jogadora"), h.newUser("Pendente")
	campaign := h.newCampaign(master, player)
	h.join(master, campaign, true, pending)
	image := master.newImage(campaign)
	shown := master.createMap(campaign, "Mirathel", image)
	master.setMapRevealed(campaign, shown.GetId(), true)
	point := master.createPoint(&mapsv1.CreateMapPointRequest{CampaignId: campaign, MapId: shown.GetId(), Kind: mapsv1.MapPointKind_MAP_POINT_KIND_SCENE, Name: "Taverna"})
	pc := player.createCharacter(campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Pensantus")
	npc := master.createCharacter(campaign, charactersv1.CharacterKind_CHARACTER_KIND_MINION, "Goblin")
	master.placeToken(campaign, shown.GetId(), pc.GetId(), 5000, 5000)

	type call = func(ctx context.Context, u *user) error
	rows := []struct {
		name string
		call call
		// master, player, non-member, anonymous, pending
		want [5]connect.Code
	}{
		{"ListMaps", func(ctx context.Context, u *user) error {
			_, err := u.maps.ListMaps(ctx, connect.NewRequest(&mapsv1.ListMapsRequest{CampaignId: campaign}))
			return err
		}, [5]connect.Code{allowed, allowed, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound}},
		{"GetMap", func(ctx context.Context, u *user) error {
			_, err := u.maps.GetMap(ctx, connect.NewRequest(&mapsv1.GetMapRequest{CampaignId: campaign, MapId: shown.GetId()}))
			return err
		}, [5]connect.Code{allowed, allowed, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound}},
		{"CreateMap", func(ctx context.Context, u *user) error {
			_, err := u.maps.CreateMap(ctx, connect.NewRequest(&mapsv1.CreateMapRequest{CampaignId: campaign, Name: "Novo", ImageId: image}))
			return err
		}, [5]connect.Code{allowed, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound}},
		{"UpdateMap", func(ctx context.Context, u *user) error {
			revision := master.mustGetMap(campaign, shown.GetId()).GetMap().GetRevision()
			_, err := u.maps.UpdateMap(ctx, connect.NewRequest(&mapsv1.UpdateMapRequest{CampaignId: campaign, MapId: shown.GetId(), Revision: revision, Name: proto.String("Mirathel")}))
			return err
		}, [5]connect.Code{allowed, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound}},
		// Each caller deletes a map of its own, so an allowed delete does not
		// change the next caller's row.
		{"DeleteMap", func(ctx context.Context, u *user) error {
			doomed := master.createMap(campaign, "Apagar", image)
			_, err := u.maps.DeleteMap(ctx, connect.NewRequest(&mapsv1.DeleteMapRequest{CampaignId: campaign, MapId: doomed.GetId()}))
			return err
		}, [5]connect.Code{allowed, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound}},
		{"SetMapRevealed", func(ctx context.Context, u *user) error {
			_, err := u.maps.SetMapRevealed(ctx, connect.NewRequest(&mapsv1.SetMapRevealedRequest{CampaignId: campaign, MapId: shown.GetId(), Revealed: true}))
			return err
		}, [5]connect.Code{allowed, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound}},
		{"SetMapGrid", func(ctx context.Context, u *user) error {
			_, err := u.maps.SetMapGrid(ctx, connect.NewRequest(&mapsv1.SetMapGridRequest{CampaignId: campaign, MapId: shown.GetId(), Columns: 20}))
			return err
		}, [5]connect.Code{allowed, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound}},
		{"CreateMapPoint", func(ctx context.Context, u *user) error {
			_, err := u.maps.CreateMapPoint(ctx, connect.NewRequest(&mapsv1.CreateMapPointRequest{CampaignId: campaign, MapId: shown.GetId(), Kind: mapsv1.MapPointKind_MAP_POINT_KIND_BATTLE, Name: "Emboscada"}))
			return err
		}, [5]connect.Code{allowed, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound}},
		{"UpdateMapPoint", func(ctx context.Context, u *user) error {
			_, err := u.maps.UpdateMapPoint(ctx, connect.NewRequest(&mapsv1.UpdateMapPointRequest{CampaignId: campaign, MapId: shown.GetId(), PointId: point.GetId(), XBp: proto.Int32(10), YBp: proto.Int32(20)}))
			return err
		}, [5]connect.Code{allowed, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound}},
		{"DeleteMapPoint", func(ctx context.Context, u *user) error {
			doomed := master.createPoint(&mapsv1.CreateMapPointRequest{CampaignId: campaign, MapId: shown.GetId(), Kind: mapsv1.MapPointKind_MAP_POINT_KIND_BATTLE, Name: "Apagar"})
			_, err := u.maps.DeleteMapPoint(ctx, connect.NewRequest(&mapsv1.DeleteMapPointRequest{CampaignId: campaign, MapId: shown.GetId(), PointId: doomed.GetId()}))
			return err
		}, [5]connect.Code{allowed, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound}},
		{"SetMapPointRevealed", func(ctx context.Context, u *user) error {
			_, err := u.maps.SetMapPointRevealed(ctx, connect.NewRequest(&mapsv1.SetMapPointRevealedRequest{CampaignId: campaign, MapId: shown.GetId(), PointId: point.GetId(), Revealed: true}))
			return err
		}, [5]connect.Code{allowed, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound}},
		// The scene actions of the point above (a SCENE point). Each caller adds,
		// changes, moves and removes one of its own.
		{"AddSceneAction", func(ctx context.Context, u *user) error {
			_, err := u.maps.AddSceneAction(ctx, connect.NewRequest(&mapsv1.AddSceneActionRequest{CampaignId: campaign, MapId: shown.GetId(), PointId: point.GetId(), Key: "skill:insight"}))
			return err
		}, [5]connect.Code{allowed, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound}},
		{"UpdateSceneAction", func(ctx context.Context, u *user) error {
			a := master.addAction(campaign, shown.GetId(), point.GetId(), "skill:arcana", "", 0)
			_, err := u.maps.UpdateSceneAction(ctx, connect.NewRequest(&mapsv1.UpdateSceneActionRequest{CampaignId: campaign, MapId: shown.GetId(), PointId: point.GetId(), ActionId: a.GetId(), Dc: proto.Int32(12)}))
			return err
		}, [5]connect.Code{allowed, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound}},
		{"MoveSceneAction", func(ctx context.Context, u *user) error {
			a := master.addAction(campaign, shown.GetId(), point.GetId(), "ability:str", "", 0)
			_, err := u.maps.MoveSceneAction(ctx, connect.NewRequest(&mapsv1.MoveSceneActionRequest{CampaignId: campaign, MapId: shown.GetId(), PointId: point.GetId(), ActionId: a.GetId(), Direction: mapsv1.SceneActionDirection_SCENE_ACTION_DIRECTION_UP}))
			return err
		}, [5]connect.Code{allowed, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound}},
		{"RemoveSceneAction", func(ctx context.Context, u *user) error {
			a := master.addAction(campaign, shown.GetId(), point.GetId(), "save:wis", "", 0)
			_, err := u.maps.RemoveSceneAction(ctx, connect.NewRequest(&mapsv1.RemoveSceneActionRequest{CampaignId: campaign, MapId: shown.GetId(), PointId: point.GetId(), ActionId: a.GetId()}))
			return err
		}, [5]connect.Code{allowed, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound}},
		// The clues of the same point (MR-029); each caller works on a clue of
		// its own, and the reveal goes to the player's character.
		{"AddSceneClue", func(ctx context.Context, u *user) error {
			_, err := u.maps.AddSceneClue(ctx, connect.NewRequest(&mapsv1.AddSceneClueRequest{CampaignId: campaign, MapId: shown.GetId(), PointId: point.GetId(), Text: "Uma pista"}))
			return err
		}, [5]connect.Code{allowed, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound}},
		{"UpdateSceneClue", func(ctx context.Context, u *user) error {
			c := master.addClue(campaign, shown.GetId(), point.GetId(), "Outra pista")
			_, err := u.maps.UpdateSceneClue(ctx, connect.NewRequest(&mapsv1.UpdateSceneClueRequest{CampaignId: campaign, MapId: shown.GetId(), PointId: point.GetId(), ClueId: c.GetId(), Text: "Mudada"}))
			return err
		}, [5]connect.Code{allowed, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound}},
		{"MoveSceneClue", func(ctx context.Context, u *user) error {
			c := master.addClue(campaign, shown.GetId(), point.GetId(), "Movida")
			_, err := u.maps.MoveSceneClue(ctx, connect.NewRequest(&mapsv1.MoveSceneClueRequest{CampaignId: campaign, MapId: shown.GetId(), PointId: point.GetId(), ClueId: c.GetId(), Direction: mapsv1.SceneActionDirection_SCENE_ACTION_DIRECTION_UP}))
			return err
		}, [5]connect.Code{allowed, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound}},
		{"RemoveSceneClue", func(ctx context.Context, u *user) error {
			c := master.addClue(campaign, shown.GetId(), point.GetId(), "Apagada")
			_, err := u.maps.RemoveSceneClue(ctx, connect.NewRequest(&mapsv1.RemoveSceneClueRequest{CampaignId: campaign, MapId: shown.GetId(), PointId: point.GetId(), ClueId: c.GetId()}))
			return err
		}, [5]connect.Code{allowed, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound}},
		{"RevealSceneClue", func(_ context.Context, u *user) error {
			c := master.addClue(campaign, shown.GetId(), point.GetId(), "Revelada")
			_, err := u.revealClue(campaign, c.GetId(), pc.GetId())
			return err
		}, [5]connect.Code{allowed, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound}},
		{"PlaceMapToken", func(ctx context.Context, u *user) error {
			_, err := u.maps.PlaceMapToken(ctx, connect.NewRequest(&mapsv1.PlaceMapTokenRequest{CampaignId: campaign, MapId: shown.GetId(), CharacterId: pc.GetId(), XBp: 5100, YBp: 5100}))
			return err
		}, [5]connect.Code{allowed, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound}},
		{"SetMapTokenHidden", func(ctx context.Context, u *user) error {
			_, err := u.maps.SetMapTokenHidden(ctx, connect.NewRequest(&mapsv1.SetMapTokenHiddenRequest{CampaignId: campaign, MapId: shown.GetId(), CharacterId: pc.GetId(), Hidden: false}))
			return err
		}, [5]connect.Code{allowed, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound}},
		{"RemoveMapToken", func(ctx context.Context, u *user) error {
			master.placeToken(campaign, shown.GetId(), npc.GetId(), 100, 100)
			_, err := u.maps.RemoveMapToken(ctx, connect.NewRequest(&mapsv1.RemoveMapTokenRequest{CampaignId: campaign, MapId: shown.GetId(), CharacterId: npc.GetId()}))
			return err
		}, [5]connect.Code{allowed, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound}},
	}

	covered := map[string]bool{}
	for _, r := range rows {
		covered[r.name] = true
	}
	methods := mapsv1.File_meurpg_maps_v1_maps_proto.Services().ByName("MapService").Methods()
	for i := range methods.Len() {
		if name := string(methods.Get(i).Name()); !covered[name] {
			t.Errorf("MapService.%s is missing from the authorization matrix", name)
		}
	}

	callers := []struct {
		name string
		user *user
	}{
		{"master", master},
		{"player", player},
		{"non-member", h.newUser("De fora")},
		{"anonymous", h.anonymous()},
		{"pending", pending},
	}
	for _, r := range rows {
		for i, caller := range callers {
			wantCode(t, r.name+" as "+caller.name, r.call(t.Context(), caller.user), r.want[i])
		}
	}
}

// PlayService.SetCurrentMap: only the master, only during a session, only a
// map of the campaign. Setting it reveals the map; GetLiveSession and the
// stream carry it; deleting the map clears it; a new session starts without
// one.
func TestSetCurrentMap(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, player := h.newUser("Mestre"), h.newUser("Jogadora")
	campaign := h.newCampaign(master, player)
	other := h.newCampaign(master)
	image := master.newImage(campaign)
	hidden := master.createMap(campaign, "Torre de Mirathel", image)
	elsewhere := master.createMap(other, "Outro", master.newImage(other))

	// Outside a session: NO_OPEN_SESSION, and the map stays hidden.
	_, err := master.setCurrentMap(campaign, hidden.GetId())
	wantBlocked(t, "SetCurrentMap without a session", err, playv1.GameSessionBlockedReason_GAME_SESSION_BLOCKED_REASON_NO_OPEN_SESSION)
	if master.mustGetMap(campaign, hidden.GetId()).GetMap().GetRevealed() {
		t.Error("a refused SetCurrentMap revealed the map")
	}

	session := master.start(campaign)
	if got := player.liveCurrentMap(campaign); got != "" {
		t.Errorf("a new session's current map = %q, want none", got)
	}
	pw := player.watch(campaign)

	for _, tc := range []struct {
		name   string
		caller *user
		mapID  string
		want   connect.Code
	}{
		{"a player", player, hidden.GetId(), connect.CodePermissionDenied},
		{"another campaign's map", master, elsewhere.GetId(), connect.CodeNotFound},
		{"a map that does not exist", master, uuid.New().String(), connect.CodeNotFound},
		{"not a UUID", master, "mapa", connect.CodeNotFound},
	} {
		_, err := tc.caller.setCurrentMap(campaign, tc.mapID)
		wantCode(t, "SetCurrentMap with "+tc.name, err, tc.want)
	}

	// Setting it reveals the map, and everyone hears.
	if got, err := master.setCurrentMap(campaign, hidden.GetId()); err != nil || got != hidden.GetId() {
		t.Fatalf("SetCurrentMap() = %q, %v; want %s", got, err, hidden.GetId())
	}
	pw.currentMapChanged(hidden.GetId())
	if got := player.liveCurrentMap(campaign); got != hidden.GetId() {
		t.Errorf("GetLiveSession current_map_id = %q, want %s", got, hidden.GetId())
	}
	if m := master.mustGetMap(campaign, hidden.GetId()).GetMap(); !m.GetRevealed() || !m.GetCurrent() {
		t.Errorf("the current map = revealed %v, current %v; want both", m.GetRevealed(), m.GetCurrent())
	}
	if got := player.mustGetMap(campaign, hidden.GetId()).GetMap(); !got.GetCurrent() {
		t.Errorf("the player's current map has current = false")
	}

	// Setting it again is fine; clearing it too.
	if _, err := master.setCurrentMap(campaign, hidden.GetId()); err != nil {
		t.Errorf("SetCurrentMap() again error = %v", err)
	}
	pw.currentMapChanged(hidden.GetId())
	if got, err := master.setCurrentMap(campaign, ""); err != nil || got != "" {
		t.Errorf("SetCurrentMap(\"\") = %q, %v; want cleared", got, err)
	}
	pw.currentMapChanged("")

	// Deleting the current map clears it, and everyone hears.
	if _, err := master.setCurrentMap(campaign, hidden.GetId()); err != nil {
		t.Fatalf("SetCurrentMap() error = %v", err)
	}
	pw.currentMapChanged(hidden.GetId())
	if _, err := master.maps.DeleteMap(t.Context(), connect.NewRequest(&mapsv1.DeleteMapRequest{CampaignId: campaign, MapId: hidden.GetId()})); err != nil {
		t.Fatalf("DeleteMap() error = %v", err)
	}
	pw.mapChanged(hidden.GetId())
	pw.currentMapChanged("")
	if got := player.liveCurrentMap(campaign); got != "" {
		t.Errorf("after deleting the current map, current_map_id = %q, want none", got)
	}

	// A new session starts without a current map.
	next := master.createMap(campaign, "Mirathel", image)
	if _, err := master.setCurrentMap(campaign, next.GetId()); err != nil {
		t.Fatalf("SetCurrentMap() error = %v", err)
	}
	if _, err := master.play.EndGameSession(t.Context(), connect.NewRequest(&playv1.EndGameSessionRequest{CampaignId: campaign, GameSessionId: session.GetId()})); err != nil {
		t.Fatalf("EndGameSession() error = %v", err)
	}
	master.start(campaign)
	if got := player.liveCurrentMap(campaign); got != "" {
		t.Errorf("the next session's current map = %q, want none", got)
	}
}

// MR-019's third criterion, with the real maps: an image a map uses cannot
// be deleted, the answer names the maps, and the files stay. Once no map
// uses it, it can go.
func TestDeletingAnImageAMapUses(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master := h.newUser("Mestre")
	campaign := h.newCampaign(master)
	img := master.mustUpload(campaign, "mirathel.png", pngImage(t, 30, 30))
	other := master.newImage(campaign)
	region := master.createMap(campaign, "Mirathel e arredores", img.GetId())
	night := master.createMap(campaign, "Mirathel à noite", img.GetId())
	ctx := t.Context()

	_, err := master.gallery.DeleteGalleryImage(ctx, connect.NewRequest(&mapsv1.DeleteGalleryImageRequest{CampaignId: campaign, ImageId: img.GetId()}))
	wantCode(t, "DeleteGalleryImage of an image in use", err, connect.CodeFailedPrecondition)
	var inUse *mapsv1.ImageInUse
	if ce, ok := errors.AsType[*connect.Error](err); ok {
		for _, d := range ce.Details() {
			if msg, derr := d.Value(); derr == nil {
				if detail, ok := msg.(*mapsv1.ImageInUse); ok {
					inUse = detail
				}
			}
		}
	}
	want := []*mapsv1.MapRef{{Id: region.GetId(), Name: "Mirathel e arredores"}, {Id: night.GetId(), Name: "Mirathel à noite"}}
	if inUse == nil || !slices.EqualFunc(inUse.GetMaps(), want, func(a, b *mapsv1.MapRef) bool { return proto.Equal(a, b) }) {
		t.Errorf("ImageInUse = %v, want the two maps, oldest first", inUse)
	}
	if strings.Contains(err.Error(), "Mirathel") {
		t.Errorf("the error message %q names a map: map names are free text, only the detail carries them", err)
	}
	if res := master.get(img.GetUrl()); res.status != http.StatusOK {
		t.Errorf("GET the image in use: status %d, want 200: its files must stay", res.status)
	}

	// Change both maps' image: now it can go.
	for _, m := range []*mapsv1.Map{region, night} {
		if _, err := master.maps.UpdateMap(ctx, connect.NewRequest(&mapsv1.UpdateMapRequest{CampaignId: campaign, MapId: m.GetId(), Revision: m.GetRevision(), ImageId: proto.String(other)})); err != nil {
			t.Fatalf("UpdateMap(image) error = %v", err)
		}
	}
	if _, err := master.gallery.DeleteGalleryImage(ctx, connect.NewRequest(&mapsv1.DeleteGalleryImageRequest{CampaignId: campaign, ImageId: img.GetId()})); err != nil {
		t.Errorf("DeleteGalleryImage of an image no map uses: %v", err)
	}
}

// The gallery shows, on each image, the maps that use it ("Usada em
// Mirathel e arredores"), and renaming an image keeps them in the answer.
func TestListGalleryImagesShowsWhereEachIsUsed(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master := h.newUser("Mestre")
	campaign := h.newCampaign(master)
	used := master.newImage(campaign)
	unused := master.newImage(campaign)
	region := master.createMap(campaign, "Mirathel e arredores", used)

	byID := map[string]*mapsv1.GalleryImage{}
	for _, img := range master.list(campaign).GetImages() {
		byID[img.GetId()] = img
	}
	if got := byID[used].GetUsedInMaps(); len(got) != 1 || got[0].GetId() != region.GetId() || got[0].GetName() != "Mirathel e arredores" {
		t.Errorf("used image's used_in_maps = %v, want the map", got)
	}
	if got := byID[unused].GetUsedInMaps(); len(got) != 0 {
		t.Errorf("unused image's used_in_maps = %v, want none", got)
	}
	res, err := master.gallery.RenameGalleryImage(t.Context(), connect.NewRequest(&mapsv1.RenameGalleryImageRequest{CampaignId: campaign, ImageId: used, Name: "Mapa de Mirathel"}))
	if err != nil {
		t.Fatalf("RenameGalleryImage() error = %v", err)
	}
	if got := res.Msg.GetImage().GetUsedInMaps(); len(got) != 1 || got[0].GetId() != region.GetId() {
		t.Errorf("renamed image's used_in_maps = %v, want the map", got)
	}
	if got := master.mustGetMap(campaign, region.GetId()).GetMap().GetImage().GetName(); got != "Mapa de Mirathel" {
		t.Errorf("the master's map image name = %q, want the new name", got)
	}
}

// UpdateMap checks the revision (AIP-154): a stale one is aborted and
// changes nothing.
func TestUpdateMapRefusesAStaleRevision(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master := h.newUser("Mestre")
	campaign := h.newCampaign(master)
	other := h.newCampaign(master)
	first, second := master.newImage(campaign), master.newImage(campaign)
	foreign := master.newImage(other)
	m := master.createMap(campaign, "Mirathel", first)
	ctx := t.Context()
	update := func(req *mapsv1.UpdateMapRequest) (*mapsv1.Map, error) {
		req.CampaignId, req.MapId = campaign, m.GetId()
		res, err := master.maps.UpdateMap(ctx, connect.NewRequest(req))
		if err != nil {
			return nil, err
		}
		return res.Msg.GetMap(), nil
	}

	if m.GetRevision() != 1 {
		t.Fatalf("a new map's revision = %d, want 1", m.GetRevision())
	}
	renamed, err := update(&mapsv1.UpdateMapRequest{Revision: 1, Name: proto.String("Mirathel e arredores")})
	if err != nil || renamed.GetRevision() != 2 || renamed.GetName() != "Mirathel e arredores" {
		t.Fatalf("UpdateMap(name) = %v, %v; want the new name at revision 2", renamed, err)
	}
	_, err = update(&mapsv1.UpdateMapRequest{Revision: 1, Name: proto.String("Outro nome")})
	wantCode(t, "UpdateMap with a stale revision", err, connect.CodeAborted)
	if got := master.mustGetMap(campaign, m.GetId()).GetMap(); got.GetName() != "Mirathel e arredores" || got.GetRevision() != 2 {
		t.Errorf("after the stale update, map = %q at revision %d; want it unchanged", got.GetName(), got.GetRevision())
	}
	moved, err := update(&mapsv1.UpdateMapRequest{Revision: 2, ImageId: proto.String(second)})
	if err != nil || moved.GetRevision() != 3 || moved.GetImage().GetId() != second || moved.GetName() != "Mirathel e arredores" {
		t.Errorf("UpdateMap(image) = %v, %v; want the new image, same name, revision 3", moved, err)
	}

	for name, req := range map[string]*mapsv1.UpdateMapRequest{
		"nothing to change":           {Revision: 3},
		"revision 0":                  {Name: proto.String("X")},
		"an empty name":               {Revision: 3, Name: proto.String("  ")},
		"another campaign's image":    {Revision: 3, ImageId: proto.String(foreign)},
		"an image ID that is no UUID": {Revision: 3, ImageId: proto.String("imagem")},
	} {
		_, err := update(req)
		wantCode(t, "UpdateMap with "+name, err, connect.CodeInvalidArgument)
	}
}

// Deleting a map takes its points and tokens; a submap point of another map
// loses its target; the image stays; a second delete is not_found.
func TestDeletingAMap(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, player := h.newUser("Mestre"), h.newUser("Jogadora")
	campaign := h.newCampaign(master, player)
	image := master.newImage(campaign)
	region := master.createMap(campaign, "Mirathel e arredores", image)
	tower := master.createMap(campaign, "Torre de Mirathel", image)
	stairs := master.createPoint(&mapsv1.CreateMapPointRequest{
		CampaignId: campaign, MapId: region.GetId(), Kind: mapsv1.MapPointKind_MAP_POINT_KIND_SUBMAP, Name: "Torre", TargetMapId: tower.GetId(),
	})
	master.createPoint(&mapsv1.CreateMapPointRequest{CampaignId: campaign, MapId: tower.GetId(), Kind: mapsv1.MapPointKind_MAP_POINT_KIND_SCENE, Name: "Biblioteca"})
	pc := player.createCharacter(campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Pensantus")
	master.placeToken(campaign, tower.GetId(), pc.GetId(), 5000, 5000)
	ctx := t.Context()

	if _, err := master.maps.DeleteMap(ctx, connect.NewRequest(&mapsv1.DeleteMapRequest{CampaignId: campaign, MapId: tower.GetId()})); err != nil {
		t.Fatalf("DeleteMap() error = %v", err)
	}
	for _, table := range []string{"map_points", "map_tokens"} {
		var n int
		if err := h.pool.QueryRow(ctx, "SELECT count(*) FROM "+table+" WHERE map_id = $1", tower.GetId()).Scan(&n); err != nil || n != 0 {
			t.Errorf("%s rows of the deleted map = %d, %v; want 0", table, n, err)
		}
	}
	got := master.mustGetMap(campaign, region.GetId())
	if len(got.GetPoints()) != 1 || got.GetPoints()[0].GetId() != stairs.GetId() || got.GetPoints()[0].GetTargetMap() != nil {
		t.Errorf("the submap point after its target was deleted = %v, want it without a target", got.GetPoints())
	}
	if len(master.list(campaign).GetImages()) != 1 {
		t.Error("deleting a map deleted a gallery image")
	}
	_, err := master.maps.DeleteMap(ctx, connect.NewRequest(&mapsv1.DeleteMapRequest{CampaignId: campaign, MapId: tower.GetId()}))
	wantCode(t, "DeleteMap again", err, connect.CodeNotFound)
	_, err = master.getMap(campaign, tower.GetId())
	wantCode(t, "GetMap of a deleted map", err, connect.CodeNotFound)
}

// Deleting a campaign (today, by deleting its master's account) deletes its
// maps, points and tokens, although maps.image_id is ON DELETE RESTRICT:
// CockroachDB checks RESTRICT at the end of the statement, when the images
// and the maps are both gone.
func TestDeletingTheCampaignDeletesItsMaps(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, player := h.newUser("Mestre"), h.newUser("Jogadora")
	campaign := h.newCampaign(master, player)
	image := master.newImage(campaign)
	region := master.createMap(campaign, "Mirathel e arredores", image)
	tower := master.createMap(campaign, "Torre de Mirathel", image)
	master.createPoint(&mapsv1.CreateMapPointRequest{
		CampaignId: campaign, MapId: region.GetId(), Kind: mapsv1.MapPointKind_MAP_POINT_KIND_SUBMAP, Name: "Torre", TargetMapId: tower.GetId(),
	})
	pc := player.createCharacter(campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Pensantus")
	master.placeToken(campaign, region.GetId(), pc.GetId(), 5000, 5000)
	master.start(campaign)
	if _, err := master.setCurrentMap(campaign, region.GetId()); err != nil {
		t.Fatalf("SetCurrentMap() error = %v", err)
	}

	ctx := t.Context()
	if _, err := h.pool.Exec(ctx, "DELETE FROM users WHERE id = $1", master.id); err != nil {
		t.Fatalf("delete the master's account: %v", err)
	}
	for _, table := range []string{"maps", "map_points", "map_tokens", "gallery_images", "game_sessions"} {
		var n int
		if err := h.pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&n); err != nil || n != 0 {
			t.Errorf("%s rows after the campaign was deleted = %d, %v; want 0", table, n, err)
		}
	}
}

// The input rules: names, descriptions, positions, kinds, targets, and the
// limits on maps and points.
func TestMapInputRules(t *testing.T) {
	t.Parallel()
	h := newHarness(t, func(c *Config) { c.MaxMaps, c.MaxPointsPerMap = 2, 2 })
	master := h.newUser("Mestre")
	campaign := h.newCampaign(master)
	other := h.newCampaign(master)
	image := master.newImage(campaign)
	foreignImage := master.newImage(other)
	m := master.createMap(campaign, "Mirathel", image)
	foreignMap := master.createMap(other, "Outro", foreignImage)
	ctx := t.Context()

	for name, req := range map[string]*mapsv1.CreateMapRequest{
		"an empty name":                {Name: " ", ImageId: image},
		"a name of 81 characters":      {Name: strings.Repeat("é", 81), ImageId: image},
		"a line break in the name":     {Name: "Mira\nthel", ImageId: image},
		"another campaign's image":     {Name: "Mapa", ImageId: foreignImage},
		"an image that does not exist": {Name: "Mapa", ImageId: uuid.New().String()},
		"an image ID that is no UUID":  {Name: "Mapa", ImageId: "imagem"},
	} {
		req.CampaignId = campaign
		_, err := master.maps.CreateMap(ctx, connect.NewRequest(req))
		wantCode(t, "CreateMap with "+name, err, connect.CodeInvalidArgument)
	}

	point := func(edit func(*mapsv1.CreateMapPointRequest)) *mapsv1.CreateMapPointRequest {
		req := &mapsv1.CreateMapPointRequest{CampaignId: campaign, MapId: m.GetId(), Kind: mapsv1.MapPointKind_MAP_POINT_KIND_SCENE, Name: "Taverna", XBp: 10000, YBp: 0}
		edit(req)
		return req
	}
	for name, req := range map[string]*mapsv1.CreateMapPointRequest{
		"no kind":                     point(func(r *mapsv1.CreateMapPointRequest) { r.Kind = mapsv1.MapPointKind_MAP_POINT_KIND_UNSPECIFIED }),
		"an empty name":               point(func(r *mapsv1.CreateMapPointRequest) { r.Name = "" }),
		"x below 0":                   point(func(r *mapsv1.CreateMapPointRequest) { r.XBp = -1 }),
		"y above 10000":               point(func(r *mapsv1.CreateMapPointRequest) { r.YBp = 10001 }),
		"a description of 2001 chars": point(func(r *mapsv1.CreateMapPointRequest) { r.Description = strings.Repeat("a", 2001) }),
		"a control character":         point(func(r *mapsv1.CreateMapPointRequest) { r.Description = "O lich\x00" }),
		"a target on a scene":         point(func(r *mapsv1.CreateMapPointRequest) { r.TargetMapId = m.GetId() }),
		"a submap leading to its own map": point(func(r *mapsv1.CreateMapPointRequest) {
			r.Kind, r.TargetMapId = mapsv1.MapPointKind_MAP_POINT_KIND_SUBMAP, m.GetId()
		}),
		"a submap leading to another campaign's map": point(func(r *mapsv1.CreateMapPointRequest) {
			r.Kind, r.TargetMapId = mapsv1.MapPointKind_MAP_POINT_KIND_SUBMAP, foreignMap.GetId()
		}),
	} {
		_, err := master.maps.CreateMapPoint(ctx, connect.NewRequest(req))
		wantCode(t, "CreateMapPoint with "+name, err, connect.CodeInvalidArgument)
	}
	// Edges: 0 and 10000, a description of 2000 characters over several
	// lines.
	p := master.createPoint(point(func(r *mapsv1.CreateMapPointRequest) {
		r.Description = "Onde a Velha Odra\nconta o que sabe.\n" + strings.Repeat("é", 1964)
	}))
	if p.GetXBp() != 10000 || p.GetYBp() != 0 || !strings.Contains(p.GetDescription(), "\n") || p.GetRevealed() {
		t.Errorf("point = %v, want it at (10000, 0), with its lines, hidden", p)
	}

	for name, req := range map[string]*mapsv1.UpdateMapPointRequest{
		"nothing to change":    {},
		"an unspecified kind":  {Kind: mapsv1.MapPointKind_MAP_POINT_KIND_UNSPECIFIED.Enum()},
		"x above 10000":        {XBp: proto.Int32(10001)},
		"a target on a scene":  {TargetMapId: proto.String(foreignMap.GetId())},
		"a name with a tab":    {Name: proto.String("Ta\tverna")},
		"an empty description": nil, // an empty description is fine: checked below
	} {
		if req == nil {
			continue
		}
		req.CampaignId, req.MapId, req.PointId = campaign, m.GetId(), p.GetId()
		_, err := master.maps.UpdateMapPoint(ctx, connect.NewRequest(req))
		wantCode(t, "UpdateMapPoint with "+name, err, connect.CodeInvalidArgument)
	}
	if _, err := master.maps.UpdateMapPoint(ctx, connect.NewRequest(&mapsv1.UpdateMapPointRequest{CampaignId: campaign, MapId: m.GetId(), PointId: p.GetId(), Description: proto.String("")})); err != nil {
		t.Errorf("UpdateMapPoint with an empty description: %v", err)
	}
	_, err := master.maps.UpdateMapPoint(ctx, connect.NewRequest(&mapsv1.UpdateMapPointRequest{CampaignId: campaign, MapId: foreignMap.GetId(), PointId: p.GetId(), Name: proto.String("X")}))
	wantCode(t, "UpdateMapPoint on another campaign's map", err, connect.CodeNotFound)

	// The limits (2 and 2 in this test): resource_exhausted.
	master.createMap(campaign, "Segundo", image)
	_, err = master.maps.CreateMap(ctx, connect.NewRequest(&mapsv1.CreateMapRequest{CampaignId: campaign, Name: "Terceiro", ImageId: image}))
	wantCode(t, "CreateMap over the limit", err, connect.CodeResourceExhausted)
	master.createPoint(point(func(*mapsv1.CreateMapPointRequest) {}))
	_, err = master.maps.CreateMapPoint(ctx, connect.NewRequest(point(func(*mapsv1.CreateMapPointRequest) {})))
	wantCode(t, "CreateMapPoint over the limit", err, connect.CodeResourceExhausted)
}

// A new token of a player's character is visible, an NPC's hidden
// (question 31). Moving keeps the state; hiding and showing work; the
// player sees only visible tokens; removing takes the token off.
func TestTokensDefaults(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, player := h.newUser("Mestre"), h.newUser("Jogadora")
	campaign := h.newCampaign(master, player)
	m := master.createMap(campaign, "Mirathel", master.newImage(campaign))
	master.setMapRevealed(campaign, m.GetId(), true)
	pc := player.createCharacter(campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Pensantus")
	boss := master.createCharacter(campaign, charactersv1.CharacterKind_CHARACTER_KIND_BOSS, "Capitão Goblin")

	if tk := master.placeToken(campaign, m.GetId(), pc.GetId(), 5000, 5000); tk.GetHidden() || tk.GetName() != "Pensantus" || tk.GetKind() != charactersv1.CharacterKind_CHARACTER_KIND_PLAYER {
		t.Errorf("a player's new token = %v, want it visible, named, of kind PLAYER", tk)
	}
	if tk := master.placeToken(campaign, m.GetId(), boss.GetId(), 3700, 6000); !tk.GetHidden() {
		t.Errorf("an NPC's new token = %v, want it hidden", tk)
	}
	if tk := master.placeToken(campaign, m.GetId(), boss.GetId(), 3800, 6100); !tk.GetHidden() || tk.GetXBp() != 3800 || tk.GetYBp() != 6100 {
		t.Errorf("the moved NPC token = %v, want it hidden at (3800, 6100)", tk)
	}
	if got := tokenIDs(master.mustGetMap(campaign, m.GetId())); !slices.Equal(got, []string{pc.GetId(), boss.GetId()}) {
		t.Errorf("master's tokens = %v, want the player's character, then the NPC", got)
	}
	playerView := player.mustGetMap(campaign, m.GetId())
	if got := tokenIDs(playerView); !slices.Equal(got, []string{pc.GetId()}) {
		t.Errorf("player's tokens = %v, want only their visible character", got)
	}
	if tk := playerView.GetTokens()[0]; !tk.GetMine() {
		t.Error("the player's own token has mine = false")
	}

	master.setTokenHidden(campaign, m.GetId(), boss.GetId(), false)
	master.setTokenHidden(campaign, m.GetId(), pc.GetId(), true)
	playerView = player.mustGetMap(campaign, m.GetId())
	if got := tokenIDs(playerView); !slices.Equal(got, []string{boss.GetId()}) || playerView.GetTokens()[0].GetMine() {
		t.Errorf("player's tokens after the swap = %v, want only the NPC, not theirs", got)
	}

	if _, err := master.maps.RemoveMapToken(t.Context(), connect.NewRequest(&mapsv1.RemoveMapTokenRequest{CampaignId: campaign, MapId: m.GetId(), CharacterId: boss.GetId()})); err != nil {
		t.Fatalf("RemoveMapToken() error = %v", err)
	}
	_, err := master.maps.RemoveMapToken(t.Context(), connect.NewRequest(&mapsv1.RemoveMapTokenRequest{CampaignId: campaign, MapId: m.GetId(), CharacterId: boss.GetId()}))
	wantCode(t, "RemoveMapToken again", err, connect.CodeNotFound)
	_, err = master.maps.SetMapTokenHidden(t.Context(), connect.NewRequest(&mapsv1.SetMapTokenHiddenRequest{CampaignId: campaign, MapId: m.GetId(), CharacterId: boss.GetId(), Hidden: true}))
	wantCode(t, "SetMapTokenHidden of a removed token", err, connect.CodeNotFound)
}

// Only the campaign's living characters stand on its maps: not another
// campaign's, not a dead one, not one waiting for approval. A character who
// dies stays on the map, but its token is not listed.
func TestTokensOnlyForTheCampaignsCharacters(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, player, fallen, waiting := h.newUser("Mestre"), h.newUser("Jogadora"), h.newUser("Caída"), h.newUser("Pendente")
	campaign := h.newCampaign(master, player, fallen)
	h.join(master, campaign, true, waiting)
	other := h.newCampaign(master, player)
	m := master.createMap(campaign, "Mirathel", master.newImage(campaign))
	stranger := player.createCharacter(other, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "De outra mesa")
	dead := fallen.createCharacter(campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Brisa")
	pending := waiting.createCharacter(campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Toren")
	ctx := t.Context()

	master.placeToken(campaign, m.GetId(), dead.GetId(), 5000, 5000)
	if _, err := master.characters.MarkCharacterDead(ctx, connect.NewRequest(&charactersv1.MarkCharacterDeadRequest{CampaignId: campaign, CharacterId: dead.GetId()})); err != nil {
		t.Fatalf("MarkCharacterDead() error = %v", err)
	}
	if got := tokenIDs(master.mustGetMap(campaign, m.GetId())); len(got) != 0 {
		t.Errorf("tokens after the character died = %v, want none listed", got)
	}

	for name, id := range map[string]string{
		"another campaign's character":     stranger.GetId(),
		"a dead character":                 dead.GetId(),
		"a character waiting for approval": pending.GetId(),
		"a character that does not exist":  uuid.New().String(),
		"an ID that is no UUID":            "personagem",
	} {
		_, err := master.maps.PlaceMapToken(ctx, connect.NewRequest(&mapsv1.PlaceMapTokenRequest{CampaignId: campaign, MapId: m.GetId(), CharacterId: id, XBp: 1, YBp: 1}))
		wantCode(t, "PlaceMapToken for "+name, err, connect.CodeNotFound)
	}
	// The dead character's token can still be taken off.
	if _, err := master.maps.RemoveMapToken(ctx, connect.NewRequest(&mapsv1.RemoveMapTokenRequest{CampaignId: campaign, MapId: m.GetId(), CharacterId: dead.GetId()})); err != nil {
		t.Errorf("RemoveMapToken of a dead character's token: %v", err)
	}
}

// wantBlocked checks a GameSessionBlocked failed_precondition.
func wantBlocked(t *testing.T, call string, err error, reason playv1.GameSessionBlockedReason) {
	t.Helper()
	wantCode(t, call, err, connect.CodeFailedPrecondition)
	ce, _ := errors.AsType[*connect.Error](err)
	for _, d := range ce.Details() {
		if msg, derr := d.Value(); derr == nil {
			if b, ok := msg.(*playv1.GameSessionBlocked); ok && b.GetReason() == reason {
				return
			}
		}
	}
	t.Errorf("%s error = %v, want a GameSessionBlocked detail with %v", call, err, reason)
}

// The battle grid of a map (MR-013, RN-21): squares of 1.5 m across the
// image's width, rows from the image's proportions. Only the master sets it;
// 0 clears it; a player who sees the map reads it and hears about a change;
// a battle point may lead to another map, like a submap.
func TestMR013_SetMapGrid(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, player := h.newUser("Mestre"), h.newUser("Jogadora")
	campaign := h.newCampaign(master, player)
	image := master.newImage(campaign)
	m := master.createMap(campaign, "Mirathel", image)
	master.setMapRevealed(campaign, m.GetId(), true)
	ctx := t.Context()
	if m.GetGridColumns() != 0 || m.GetGridRows() != 0 {
		t.Fatalf("a new map has grid %dx%d, want none", m.GetGridColumns(), m.GetGridRows())
	}

	master.start(campaign)
	stream := player.watch(campaign)

	set := func(columns int32) (*mapsv1.Map, error) {
		res, err := master.maps.SetMapGrid(ctx, connect.NewRequest(&mapsv1.SetMapGridRequest{CampaignId: campaign, MapId: m.GetId(), Columns: columns}))
		if err != nil {
			return nil, err
		}
		return res.Msg.GetMap(), nil
	}
	got, err := set(24)
	if err != nil {
		t.Fatalf("SetMapGrid(24) error = %v", err)
	}
	w, h2 := got.GetImage().GetWidth(), got.GetImage().GetHeight()
	wantRows := (24*h2 + w/2) / w // round(columns * height / width)
	if wantRows < 1 {
		wantRows = 1
	}
	if got.GetGridColumns() != 24 || got.GetGridRows() != wantRows || got.GetRevision() != m.GetRevision() {
		t.Errorf("grid = %dx%d at revision %d, want 24x%d, revision unchanged (%d)", got.GetGridColumns(), got.GetGridRows(), got.GetRevision(), wantRows, m.GetRevision())
	}
	stream.mapChanged(m.GetId()) // a player who sees the map hears about it
	if seen := player.mustGetMap(campaign, m.GetId()).GetMap(); seen.GetGridColumns() != 24 {
		t.Errorf("a player reads grid_columns = %d, want 24", seen.GetGridColumns())
	}

	for _, bad := range []int32{1, 3, 201, -4} {
		_, err := set(bad)
		wantCode(t, "SetMapGrid("+strconv.Itoa(int(bad))+")", err, connect.CodeInvalidArgument)
	}
	cleared, err := set(0)
	if err != nil || cleared.GetGridColumns() != 0 || cleared.GetGridRows() != 0 {
		t.Errorf("SetMapGrid(0) = %v, %v; want no grid", cleared, err)
	}

	// A battle point leads to a map of the campaign, like a submap; a scene does not.
	other := master.createMap(campaign, "Estrada", image)
	point := master.createPoint(&mapsv1.CreateMapPointRequest{
		CampaignId: campaign, MapId: m.GetId(), Kind: mapsv1.MapPointKind_MAP_POINT_KIND_BATTLE, Name: "Emboscada", TargetMapId: other.GetId(),
	})
	if point.GetTargetMap().GetId() != other.GetId() {
		t.Errorf("battle point target = %v, want %s", point.GetTargetMap(), other.GetId())
	}
	_, err = master.maps.CreateMapPoint(ctx, connect.NewRequest(&mapsv1.CreateMapPointRequest{
		CampaignId: campaign, MapId: m.GetId(), Kind: mapsv1.MapPointKind_MAP_POINT_KIND_SCENE, Name: "Taverna", TargetMapId: other.GetId(),
	}))
	wantCode(t, "CreateMapPoint(scene with a target)", err, connect.CodeInvalidArgument)
	// A battle's target is not a "parent": the other map is not a submap of this one.
	if parents := master.mustGetMap(campaign, other.GetId()).GetMap().GetParentMaps(); len(parents) != 0 {
		t.Errorf("parent maps of the battle's map = %v, want none", parents)
	}
}
