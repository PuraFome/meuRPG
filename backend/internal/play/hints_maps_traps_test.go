package play

import (
	"net/http"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1/mapsv1connect"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The writes of the maps module that change a trap, a treasure or a scene's clues run under play's funnel (Service.InSession,
// #735 part 2, 4d maps): one number for each write, carried by what it tells; a write that changes nothing moves nothing; a
// refused call tells nothing; a keyed one replays without a new number and tells again; with no open session nothing moves.
// A trap or a treasure the players do not see tells the master alone: the players' number moves the same.

func (r *trapRig) treasureClient(u *user) mapsv1connect.TreasureServiceClient {
	return mapsv1connect.NewTreasureServiceClient(&http.Client{Transport: userTransport{userID: u.id, next: r.server.Client().Transport}}, r.server.URL)
}

func (r *trapRig) placeTreasureCall(t *testing.T, key string, col int32) func() error {
	t.Helper()
	content, err := testRules()
	if err != nil {
		t.Fatalf("rules.LoadSRD() error = %v", err)
	}
	version := content.Version()
	return func() error {
		_, err := r.treasureClient(r.master).PlaceTreasure(t.Context(), connect.NewRequest(&mapsv1.PlaceTreasureRequest{
			CampaignId: r.campaignID, MapId: r.mapID, Mode: mapsv1.TreasureMode_TREASURE_MODE_HOARD, PartyLevel: 4,
			Seed: proto.Uint64(2209), Column: col, Row: 5, IdempotencyKey: key, ContentVersion: version,
		}))
		return err
	}
}

func TestPlaceTreasureHints(t *testing.T) {
	t.Parallel()
	r, g := mapRigHints(t, "Caio", "Ana", "Bia") // a new treasure is hidden: the master alone hears
	key := newKey()
	g.keyed(r.placeTreasureCall(t, key, 7), r.placeTreasureCall(t, key, 7), r.placeTreasureCall(t, key, 8))
	g.lost(r.placeTreasureCall(t, newKey(), 9))
}

func TestRevealTrapHints(t *testing.T) {
	t.Parallel()
	r := newTrapRig(t)
	p := r.trap(t, "Poço", 12, 7)
	g := newHintRig(t, r.h, r.campaignID, r.master, map[string]*user{"Caio": r.caio, "Ana": r.ana, "Bia": r.bia}, "Ana", "Bia")
	reveal := func(ids ...string) func() error {
		return func() error {
			_, err := r.mc(r.master).RevealTrap(t.Context(), connect.NewRequest(&mapsv1.RevealTrapRequest{
				CampaignId: r.campaignID, MapId: r.mapID, PointId: p.GetId(), CharacterIds: ids, All: len(ids) == 0,
			}))
			return err
		}
	}
	// Toren's player (Caio) and the master hear; the others hear nothing but their number moves the same.
	g.first(reveal(r.toren.GetId()), true)
	// What everyone already knew changes nothing.
	before := g.seq()
	if err := reveal(r.toren.GetId())(); err != nil {
		t.Fatalf("RevealTrap(again) error = %v", err)
	}
	if got := g.seq(); got != before {
		t.Errorf("hint_seq = %d after a reveal that changed nothing, want %d", got, before)
	}
	g.refused(func() error {
		_, err := r.mc(r.master).RevealTrap(t.Context(), connect.NewRequest(&mapsv1.RevealTrapRequest{CampaignId: r.campaignID, MapId: r.mapID, PointId: newKey(), All: true}))
		return err
	})
}

func TestDisarmTrapHints(t *testing.T) {
	t.Parallel()
	r := newTrapRig(t)
	p := r.trap(t, "Poço", 12, 7)
	g := newHintRig(t, r.h, r.campaignID, r.master, map[string]*user{"Caio": r.caio, "Ana": r.ana, "Bia": r.bia}, "Caio", "Ana", "Bia") // hidden: the master alone
	disarm := func() error {
		_, err := r.mc(r.master).DisarmTrap(t.Context(), connect.NewRequest(&mapsv1.DisarmTrapRequest{CampaignId: r.campaignID, MapId: r.mapID, PointId: p.GetId()}))
		return err
	}
	g.first(disarm, true)
	before := g.seq()
	if err := disarm(); err != nil {
		t.Fatalf("DisarmTrap(again) error = %v", err)
	}
	if got := g.seq(); got != before {
		t.Errorf("hint_seq = %d after a disarm that changed nothing, want %d", got, before)
	}
}

func TestTreasureFoundHints(t *testing.T) {
	t.Parallel()
	r, g := mapRigHints(t)
	if err := r.placeTreasureCall(t, newKey(), 7)(); err != nil {
		t.Fatalf("PlaceTreasure() error = %v", err)
	}
	var id string
	if err := r.h.pool.QueryRow(t.Context(), `SELECT id::TEXT FROM map_points WHERE map_id = $1 AND kind = 'treasure'`, r.mapID).Scan(&id); err != nil {
		t.Fatalf("find the treasure: %v", err)
	}
	g.drain()
	mc := r.mc(r.master)
	// A found treasure is visible to everyone who sees the map: all hear, with the number.
	g.first(func() error {
		_, err := mc.MarkTreasureFound(t.Context(), connect.NewRequest(&mapsv1.MarkTreasureFoundRequest{CampaignId: r.campaignID, MapId: r.mapID, PointId: id, CharacterIds: []string{r.toren.GetId()}}))
		return err
	}, true)
	g.first(func() error {
		_, err := mc.UnmarkTreasureFound(t.Context(), connect.NewRequest(&mapsv1.UnmarkTreasureFoundRequest{CampaignId: r.campaignID, MapId: r.mapID, PointId: id}))
		return err
	}, true)
	g.refused(func() error {
		_, err := mc.MarkTreasureFound(t.Context(), connect.NewRequest(&mapsv1.MarkTreasureFoundRequest{CampaignId: r.campaignID, MapId: r.mapID, PointId: newKey(), CharacterIds: []string{r.toren.GetId()}}))
		return err
	})
}

func TestRevealSceneClueHints(t *testing.T) {
	t.Parallel()
	r, g := mapRigHints(t, "Caio", "Bia")
	x, y := sq(3, 3)
	pt, err := r.mc(r.master).CreateMapPoint(t.Context(), connect.NewRequest(&mapsv1.CreateMapPointRequest{CampaignId: r.campaignID, MapId: r.mapID, Kind: mapsv1.MapPointKind_MAP_POINT_KIND_SCENE, Name: "Biblioteca", XBp: x, YBp: y}))
	if err != nil {
		t.Fatalf("CreateMapPoint() error = %v", err)
	}
	clue, err := r.mc(r.master).AddSceneClue(t.Context(), connect.NewRequest(&mapsv1.AddSceneClueRequest{CampaignId: r.campaignID, MapId: r.mapID, PointId: pt.Msg.GetPoint().GetId(), Text: "Uma chave"}))
	if err != nil {
		t.Fatalf("AddSceneClue() error = %v", err)
	}
	g.drain()
	reveal := func() error {
		_, err := r.mc(r.master).RevealSceneClue(t.Context(), connect.NewRequest(&mapsv1.RevealSceneClueRequest{CampaignId: r.campaignID, ClueId: clue.Msg.GetClue().GetId(), CharacterIds: []string{r.pens.GetId()}}))
		return err
	}
	// Pensantus's player (Ana) hears that her notes changed; nobody else does.
	g.first(reveal, true)
	before := g.seq()
	if err := reveal(); err != nil {
		t.Fatalf("RevealSceneClue(again) error = %v", err)
	}
	if got := g.seq(); got != before {
		t.Errorf("hint_seq = %d after a reveal of what she had, want %d", got, before)
	}
	g.refused(func() error {
		_, err := r.mc(r.master).RevealSceneClue(t.Context(), connect.NewRequest(&mapsv1.RevealSceneClueRequest{CampaignId: r.campaignID, ClueId: newKey(), CharacterIds: []string{r.pens.GetId()}}))
		return err
	})
}

// A token dropped within reach of a trap its character notices is two writes: the token's and the notice's reveal. Each moves the
// number once; the notice is told to the finder alone (trap_noticed, and the map's change), and the others' number moves the same.
func TestNoticeTellsTheFinderAloneWithTheNumber(t *testing.T) {
	t.Parallel()
	r := newTrapRig(t)
	r.trap(t, "Fosso Dourado", 9, 7, func(s *mapsv1.TrapSpec) { s.NoticeDc = 10 })
	g := newHintRig(t, r.h, r.campaignID, r.master, map[string]*user{"Caio": r.caio, "Ana": r.ana, "Bia": r.bia})
	before := g.seq()
	r.place(t, r.toren.GetId(), 7, 7)
	if got := g.seq(); got != before+2 {
		t.Errorf("hint_seq = %d, want %d (the token, then the notice)", got, before+2)
	}
	heard := g.drain()
	noticed := func(name string) bool {
		for _, ev := range heard[name] {
			if _, ok := ev.GetEvent().(*playv1.WatchGameSessionResponse_TrapNoticed_); ok {
				if ev.GetSessionSeq() == 0 {
					t.Errorf("%s heard the notice with no number", name)
				}
				return true
			}
		}
		return false
	}
	if !noticed("Caio") {
		t.Errorf("Toren's player did not hear that he noticed the trap")
	}
	for _, name := range []string{"Ana", "Bia", "master"} {
		if noticed(name) {
			t.Errorf("%s heard of a trap Toren noticed", name)
		}
	}
}

// A token dropped on an armed trap is two writes: the token's and the trap's firing. The master's `map_changed` the firing tells
// (TrapChanged, called by play after its own commit) never carries the drop's number: a stale number is worse than none.
func TestATokenDropThatFiresATrapTellsTheFiringWithoutTheDropsNumber(t *testing.T) {
	t.Parallel()
	r := newTrapRig(t)
	r.trap(t, "Poço de entrada", 12, 7, pit("1d6"))
	g := newHintRig(t, r.h, r.campaignID, r.master, map[string]*user{"Caio": r.caio, "Ana": r.ana, "Bia": r.bia})
	before := g.seq()
	r.h.roller.queue(3)
	r.place(t, r.toren.GetId(), 12, 7)
	if got := g.seq(); got < before+2 {
		t.Fatalf("hint_seq = %d, want at least %d (the token, then the firing)", got, before+2)
	}
	stale := 0
	for _, ev := range g.drain()["master"] {
		if _, ok := ev.GetEvent().(*playv1.WatchGameSessionResponse_MapChanged_); ok && ev.GetSessionSeq() == before+1 {
			stale++
		}
	}
	if stale != 1 {
		t.Errorf("the master heard %d map_changed with the drop's number %d, want only the drop's own", stale, before+1)
	}
}
