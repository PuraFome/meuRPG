package maps

import (
	"sync"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	notesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/notes/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/dbtest"
)

// The audit of 07/10/2026 (F7): a retried create, after a timeout or a second tap, must not make a
// second map, point, action, clue, note or dungeon. Each test sends the same key twice (one
// resource and the same answer), the same key with another request (refused), and a new key or
// none (another resource, as before).

func TestCreateMapIsIdempotent(t *testing.T) {
	t.Parallel()
	s := newScenes(t, false)
	image := s.master.newImage(s.campaign)
	call := func(req *mapsv1.CreateMapRequest) (*mapsv1.Map, error) {
		res, err := s.master.maps.CreateMap(t.Context(), connect.NewRequest(req))
		if err != nil {
			return nil, err
		}
		return res.Msg.GetMap(), nil
	}
	req := &mapsv1.CreateMapRequest{CampaignId: s.campaign, Name: "Vale Seco", ImageId: image, IdempotencyKey: "key-1"}
	before := len(s.master.listMaps(s.campaign))
	first, err := call(req)
	if err != nil {
		t.Fatalf("CreateMap() error = %v", err)
	}
	again, err := call(req)
	if err != nil || !proto.Equal(first, again) {
		t.Errorf("retry = %v, %v; want the first map %v", again, err, first)
	}
	other := proto.Clone(req).(*mapsv1.CreateMapRequest)
	other.Name = "Outro"
	_, err = call(other)
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("CreateMap(same key, other name) error = %v, want invalid_argument", err)
	}
	if got := len(s.master.listMaps(s.campaign)); got != before+1 {
		t.Errorf("maps = %d, want %d", got, before+1)
	}
	next := proto.Clone(req).(*mapsv1.CreateMapRequest)
	next.IdempotencyKey = "key-2"
	if made, err := call(next); err != nil || made.GetId() == first.GetId() {
		t.Errorf("new key = %v, %v; want another map", made, err)
	}
	next.IdempotencyKey = ""
	a, errA := call(next)
	b, errB := call(next)
	if errA != nil || errB != nil || a.GetId() == b.GetId() {
		t.Errorf("no key made %v and %v (%v, %v), want two maps", a, b, errA, errB)
	}
}

func TestCreateMapPointIsIdempotent(t *testing.T) {
	t.Parallel()
	s := newScenes(t, false)
	call := func(req *mapsv1.CreateMapPointRequest) (*mapsv1.MapPoint, error) {
		res, err := s.master.maps.CreateMapPoint(t.Context(), connect.NewRequest(req))
		if err != nil {
			return nil, err
		}
		return res.Msg.GetPoint(), nil
	}
	req := &mapsv1.CreateMapPointRequest{
		CampaignId: s.campaign, MapId: s.mapID, Kind: mapsv1.MapPointKind_MAP_POINT_KIND_BATTLE, Name: "Ponte", XBp: 1000, YBp: 2000, IdempotencyKey: "key-1",
	}
	count := func() int { return len(pointIDs(s.master.mustGetMap(s.campaign, s.mapID))) }
	before := count()
	first, err := call(req)
	if err != nil {
		t.Fatalf("CreateMapPoint() error = %v", err)
	}
	again, err := call(req)
	if err != nil || !proto.Equal(first, again) {
		t.Errorf("retry = %v, %v; want the first point %v", again, err, first)
	}
	other := proto.Clone(req).(*mapsv1.CreateMapPointRequest)
	other.XBp = 5000
	_, err = call(other)
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("CreateMapPoint(same key, other place) error = %v, want invalid_argument", err)
	}
	if got := count(); got != before+1 {
		t.Errorf("points = %d, want %d", got, before+1)
	}
	next := proto.Clone(req).(*mapsv1.CreateMapPointRequest)
	next.IdempotencyKey = ""
	a, errA := call(next)
	b, errB := call(next)
	if errA != nil || errB != nil || a.GetId() == b.GetId() {
		t.Errorf("no key made %v and %v (%v, %v), want two points", a, b, errA, errB)
	}

	// Racing with the same key makes one point.
	dbtest.PoolSize(t, 4)
	racing := proto.Clone(req).(*mapsv1.CreateMapPointRequest)
	racing.IdempotencyKey, racing.Name = "racing", "Torre"
	ids := make([]string, 4)
	var wg sync.WaitGroup
	for i := range ids {
		wg.Go(func() {
			p, err := call(racing)
			if err != nil {
				t.Errorf("CreateMapPoint() racing error = %v", err)
				return
			}
			ids[i] = p.GetId()
		})
	}
	wg.Wait()
	for _, id := range ids {
		if id != ids[0] {
			t.Fatalf("ids = %v, want one point", ids)
		}
	}
}

func TestAddSceneActionAndClueAreIdempotent(t *testing.T) {
	t.Parallel()
	s := newScenes(t, false)
	action := func(req *mapsv1.AddSceneActionRequest) (*mapsv1.AddSceneActionResponse, error) {
		res, err := s.master.maps.AddSceneAction(t.Context(), connect.NewRequest(req))
		if err != nil {
			return nil, err
		}
		return res.Msg, nil
	}
	areq := &mapsv1.AddSceneActionRequest{
		CampaignId: s.campaign, MapId: s.mapID, PointId: s.point.GetId(), Key: "skill:investigation", Name: "Procurar pistas", Dc: 15, IdempotencyKey: "key-1",
	}
	first, err := action(areq)
	if err != nil {
		t.Fatalf("AddSceneAction() error = %v", err)
	}
	again, err := action(areq)
	if err != nil || again.GetAction().GetId() != first.GetAction().GetId() || len(again.GetActions()) != 1 {
		t.Errorf("retry = %v, %v; want the first action and one in the list", again, err)
	}
	other := proto.Clone(areq).(*mapsv1.AddSceneActionRequest)
	other.Dc = 20
	_, err = action(other)
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("AddSceneAction(same key, other DC) error = %v, want invalid_argument", err)
	}
	next := proto.Clone(areq).(*mapsv1.AddSceneActionRequest)
	next.IdempotencyKey = "key-2"
	if made, err := action(next); err != nil || made.GetAction().GetId() == first.GetAction().GetId() || len(made.GetActions()) != 2 {
		t.Errorf("new key = %v, %v; want a second action", made, err)
	}

	clue := func(text, key string) (*mapsv1.AddSceneClueResponse, error) {
		res, err := s.master.maps.AddSceneClue(t.Context(), connect.NewRequest(&mapsv1.AddSceneClueRequest{
			CampaignId: s.campaign, MapId: s.mapID, PointId: s.point.GetId(), Text: text, IdempotencyKey: key,
		}))
		if err != nil {
			return nil, err
		}
		return res.Msg, nil
	}
	c1, err := clue("Marcas de garras", "key-1")
	if err != nil {
		t.Fatalf("AddSceneClue() error = %v", err)
	}
	c2, err := clue("Marcas de garras", "key-1")
	if err != nil || c2.GetClue().GetId() != c1.GetClue().GetId() || len(c2.GetClues()) != 1 {
		t.Errorf("retry = %v, %v; want the first clue and one in the list", c2, err)
	}
	_, err = clue("Outra pista", "key-1")
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("AddSceneClue(same key, other text) error = %v, want invalid_argument", err)
	}
	if made, err := clue("Marcas de garras", ""); err != nil || len(made.GetClues()) != 2 {
		t.Errorf("no key = %v, %v; want a second clue", made, err)
	}
}

func TestCreateNoteIsIdempotent(t *testing.T) {
	t.Parallel()
	s := newScenes(t, false)
	call := func(text, key string) (*notesv1.Note, error) {
		res, err := s.ana.notes.CreateNote(t.Context(), connect.NewRequest(&notesv1.CreateNoteRequest{CampaignId: s.campaign, Text: text, IdempotencyKey: key}))
		if err != nil {
			return nil, err
		}
		return res.Msg.GetNote(), nil
	}
	first, err := call("O mapa tem uma cruz.", "key-1")
	if err != nil {
		t.Fatalf("CreateNote() error = %v", err)
	}
	again, err := call("O mapa tem uma cruz.", "key-1")
	if err != nil || !proto.Equal(first, again) {
		t.Errorf("retry = %v, %v; want the first note %v", again, err, first)
	}
	_, err = call("Outro texto.", "key-1")
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("CreateNote(same key, other text) error = %v, want invalid_argument", err)
	}
	if got := noteTexts(s.ana.listNotes(s.campaign, "")); len(got) != 1 {
		t.Errorf("notes = %v, want 1", got)
	}
	// The key is the author's own: Caio's same key is Caio's note.
	theirs, err := s.caio.notes.CreateNote(t.Context(), connect.NewRequest(&notesv1.CreateNoteRequest{CampaignId: s.campaign, Text: "O mapa tem uma cruz.", IdempotencyKey: "key-1"}))
	if err != nil || theirs.Msg.GetNote().GetId() == first.GetId() {
		t.Errorf("another author's key = %v, %v; want their own note", theirs, err)
	}
	a, errA := call("Sem chave", "")
	b, errB := call("Sem chave", "")
	if errA != nil || errB != nil || a.GetId() == b.GetId() {
		t.Errorf("no key made %v and %v (%v, %v), want two notes", a, b, errA, errB)
	}
}

func TestCreateDungeonMapAndPlaceSceneAreIdempotent(t *testing.T) {
	t.Parallel()
	d := newDungeonTable(t)
	seed, _ := testDungeonSeed(t)
	call := func(name, key string, seed *uint64) (*mapsv1.CreateDungeonMapResponse, error) {
		res, err := d.master.dungeons.CreateDungeonMap(t.Context(), connect.NewRequest(&mapsv1.CreateDungeonMapRequest{
			CampaignId: d.campaign, Name: name, Options: testDungeonOptions(), Seed: seed, IdempotencyKey: key,
		}))
		if err != nil {
			return nil, err
		}
		return res.Msg, nil
	}
	first, err := call("Masmorra", "key-1", &seed)
	if err != nil {
		t.Fatalf("CreateDungeonMap() error = %v", err)
	}
	again, err := call("Masmorra", "key-1", &seed)
	if err != nil || again.GetMap().GetId() != first.GetMap().GetId() || again.GetSeed() != first.GetSeed() || again.GetRoomCount() != first.GetRoomCount() {
		t.Errorf("retry = %v, %v; want the first dungeon %v", again, err, first)
	}
	_, err = call("Outra masmorra", "key-1", &seed)
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("CreateDungeonMap(same key, other name) error = %v, want invalid_argument", err)
	}
	if got := d.master.listMaps(d.campaign); len(got) != 1 {
		t.Errorf("maps = %d, want 1", len(got))
	}
	// Two at once with a new key: one dungeon (the race is decided in the transaction).
	dbtest.PoolSize(t, 4)
	ids := make([]string, 3)
	var wg sync.WaitGroup
	for i := range ids {
		wg.Go(func() {
			res, err := call("Corrida", "racing", &seed)
			if err != nil {
				t.Errorf("CreateDungeonMap() racing error = %v", err)
				return
			}
			ids[i] = res.GetMap().GetId()
		})
	}
	wg.Wait()
	for _, id := range ids {
		if id != ids[0] {
			t.Fatalf("ids = %v, want one dungeon", ids)
		}
	}
	if got := d.master.listMaps(d.campaign); len(got) != 2 {
		t.Errorf("maps = %d, want 2", len(got))
	}

	// PlaceDungeonScene: the same key and room is the same scene.
	place := func(room int32, key string) (*mapsv1.PlaceDungeonSceneResponse, error) {
		res, err := d.master.dungeons.PlaceDungeonScene(t.Context(), connect.NewRequest(&mapsv1.PlaceDungeonSceneRequest{
			CampaignId: d.campaign, MapId: first.GetMap().GetId(), RoomId: room, IdempotencyKey: key,
		}))
		if err != nil {
			return nil, err
		}
		return res.Msg, nil
	}
	p1, err := place(1, "scene-1")
	if err != nil {
		t.Fatalf("PlaceDungeonScene() error = %v", err)
	}
	p2, err := place(1, "scene-1")
	if err != nil || p2.GetPoint().GetId() != p1.GetPoint().GetId() {
		t.Errorf("retry = %v, %v; want the first scene %v", p2, err, p1)
	}
	_, err = place(2, "scene-1")
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("PlaceDungeonScene(same key, other room) error = %v, want invalid_argument", err)
	}
	rooms, err := d.master.dungeonRooms(d.campaign, first.GetMap().GetId())
	if err != nil {
		t.Fatalf("GetDungeonRooms() error = %v", err)
	}
	total := 0
	for _, r := range rooms.GetRooms() {
		total += len(r.GetScenePointIds())
	}
	if total != 1 {
		t.Errorf("scenes = %d, want 1", total)
	}
}
