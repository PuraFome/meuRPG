package play

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1/mapsv1connect"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/maps"
	"github.com/PuraFome/meuRPG/backend/internal/platform/httpserver"
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

// Combat on a map with the fog of war on (MR-036, RN-10, RN-20, Etapa 9, slice
// 9.7). The fixture is the cave of the guard-room fight, with the actual maps
// service behind it: the walls painted, Pensantus (gnome, darkvision 18 m), Toren
// (human, a torch in his hand), Brisa (halfling, nothing) and the NPCs in the
// dark. Every player's answer is read as the app's JSON (protojson), so a field
// that leaks shows up whatever its name. The numbers: Pensantus sees 12 squares
// in the dark; the torch lights 4 squares bright and 4 dim.

// fogCave is the cave with the fog on.
type fogCave struct {
	*cave
	msvc   *maps.Service
	server *httptest.Server
}

var caveGridSize = grid.Grid{Columns: 24, Rows: 16}

// atBP is a square's center in basis points, as a map token wants it.
func atBP(sq grid.Square) (x, y int32) {
	xBP, yBP := caveGridSize.CenterOf(sq)
	return int32(xBP), int32(yBP) //nolint:gosec // G115: at most 10000
}

// newFogCave builds the cave of the fight (see newCave) with the fog on: the actual
// maps service paints the walls, the rubble and the cover, and Toren carries a
// torch. The base light is dark and no other light shines.
func newFogCave(t *testing.T) *fogCave {
	t.Helper()
	c := newCave(t)
	content, err := testRules()
	if err != nil {
		t.Fatalf("rules.LoadSRD() error = %v", err)
	}
	msvc, err := maps.New(maps.Config{Pool: c.h.pool, Characters: c.h.chars, Live: c.h.svc, Rules: content, Combats: c.h.svc, Logger: slog.New(slog.DiscardHandler)})
	if err != nil {
		t.Fatalf("maps.New() error = %v", err)
	}
	c.h.svc.SetTerrain(msvc)
	c.h.svc.SetFog(msvc)
	srv := httpserver.New(httpserver.Config{Logger: slog.New(slog.DiscardHandler)})
	msvc.Mount(srv.Handle, mapsSessions{testSessions}, c.h.camps, connect.WithRequireConnectProtocolHeader())
	server := httptest.NewServer(srv.Handler())
	t.Cleanup(server.Close)
	f := &fogCave{cave: c, msvc: msvc, server: server}

	var walls, rubble, crates, column []*mapsv1.MapSquare
	for row, line := range caveRows {
		for col, ch := range line {
			sq := &mapsv1.MapSquare{Col: int32(col), Row: int32(row)} //nolint:gosec // G115: a cave of 24 x 16
			switch ch {
			case '#':
				walls = append(walls, sq)
			case ':':
				rubble = append(rubble, sq)
			case 'h':
				crates = append(crates, sq)
			case 'q':
				column = append(column, sq)
			}
		}
	}
	for _, p := range []struct {
		layer mapsv1.MapLayer
		value int32
		sq    []*mapsv1.MapSquare
	}{
		{mapsv1.MapLayer_MAP_LAYER_WALL, 1, walls},
		{mapsv1.MapLayer_MAP_LAYER_DIFFICULT_TERRAIN, 1, rubble},
		{mapsv1.MapLayer_MAP_LAYER_COVER, 1, crates},
		{mapsv1.MapLayer_MAP_LAYER_COVER, 2, column},
	} {
		f.paint(t, p.layer, p.value, p.sq...)
	}
	// Toren's token carries a torch (the combat sees from his combatant's square).
	x, y := atBP(grid.Square{Col: 6, Row: 7})
	c.h.placeToken(c.mapID, c.toren.GetId(), int(x), int(y))
	f.torch(t, true)
	if _, err := c.h.pool.Exec(t.Context(), `UPDATE maps SET fog_enabled = true WHERE id = $1`, c.mapID); err != nil {
		t.Fatalf("turn the fog on: %v", err)
	}
	return f
}

func (f *fogCave) mapsAs(u *user) mapsv1connect.MapServiceClient {
	return mapsv1connect.NewMapServiceClient(&http.Client{Transport: userTransport{userID: u.id, next: f.server.Client().Transport}}, f.server.URL)
}

// paint paints squares of a layer as the master.
func (f *fogCave) paint(t *testing.T, layer mapsv1.MapLayer, value int32, squares ...*mapsv1.MapSquare) {
	t.Helper()
	if _, err := f.mapsAs(f.master).PaintMapCells(t.Context(), connect.NewRequest(&mapsv1.PaintMapCellsRequest{
		CampaignId: f.campaignID, MapId: f.mapID, Layer: layer, Value: value, Squares: squares,
	})); err != nil {
		t.Fatalf("PaintMapCells(%v) error = %v", layer, err)
	}
}

// torch lights or puts out Toren's torch.
func (f *fogCave) torch(t *testing.T, on bool) {
	t.Helper()
	key := ""
	if on {
		key = "light:torch"
	}
	if _, err := f.mapsAs(f.caio).SetCarriedLight(t.Context(), connect.NewRequest(&mapsv1.SetCarriedLightRequest{
		CampaignId: f.campaignID, MapId: f.mapID, CharacterId: f.toren.GetId(), LightKey: key,
	})); err != nil {
		t.Fatalf("SetCarriedLight(%q) error = %v", key, err)
	}
}

// groupVision turns the map's "Visão do grupo" on or off.
func (f *fogCave) groupVision(t *testing.T, on bool) {
	t.Helper()
	if _, err := f.mapsAs(f.master).SetMapFog(t.Context(), connect.NewRequest(&mapsv1.SetMapFogRequest{CampaignId: f.campaignID, MapId: f.mapID, GroupVision: proto.Bool(on)})); err != nil {
		t.Fatalf("SetMapFog(group vision) error = %v", err)
	}
}

// seesSquare is the maps module's own answer (GetMapVision, as the app reads it):
// whether the player sees the square now, the oracle the combat is checked against.
func (f *fogCave) seesSquare(t *testing.T, u *user, col, row int) bool {
	t.Helper()
	res, err := f.mapsAs(u).GetMapVision(t.Context(), connect.NewRequest(&mapsv1.GetMapVisionRequest{CampaignId: f.campaignID, MapId: f.mapID}))
	if err != nil {
		t.Fatalf("GetMapVision() error = %v", err)
	}
	n := row*int(res.Msg.GetGridColumns()) + col
	code := res.Msg.GetStates()[n/2] >> (4 * (n % 2)) & 0xf
	return code >= 2 && code <= 4 // seen in grey, dim or bright; not a wall, not remembered
}

// npcLabels lists, sorted, the NPC combatants the player receives.
func npcLabels(e *playv1.Encounter) []string {
	var out []string
	for _, c := range e.GetCombatants() {
		if c.GetKind() == playv1.CombatantKind_COMBATANT_KIND_NPC {
			out = append(out, c.GetLabel())
		}
	}
	slices.Sort(out)
	return out
}

// wantNPCs checks the NPCs a player receives.
func wantNPCs(t *testing.T, who string, e *playv1.Encounter, want ...string) {
	t.Helper()
	slices.Sort(want)
	if got := npcLabels(e); !slices.Equal(got, want) {
		t.Errorf("%s receives the NPCs %v, want %v", who, got, want)
	}
}

// noLeak fails when the text (a player's response, as JSON) has the label or the
// ID of a combatant the player does not see.
func (f *fogCave) noLeak(t *testing.T, what, text string, unseen ...string) {
	t.Helper()
	for _, label := range unseen {
		if strings.Contains(text, label) {
			t.Errorf("%s has the name of %s, an NPC the player does not see: %s", what, label, text)
		}
		if id := f.id(t, label); strings.Contains(text, id) {
			t.Errorf("%s has the ID of %s, an NPC the player does not see: %s", what, label, text)
		}
	}
}

// allNPCs are the NPCs of the fight.
var allNPCs = []string{"Capitão Goblin", "Escudeiro", "Goblin 1", "Goblin 2", "Goblin 3", "Ogro"}

// stage puts the NPCs where the tests want them: the Capitão in the corridor, 11
// squares from Pensantus and 10 from Toren (inside her darkvision, outside his
// torch), Goblin 2 two squares from Toren, in his light, and the squire behind
// the party.
func (f *fogCave) stage(t *testing.T) {
	t.Helper()
	f.mustMove(t, f.master, "Capitão Goblin", 16, 7)
	f.mustMove(t, f.master, "Goblin 2", 8, 8)
}

// TestRN10_FogCombatEachPlayerSeesOnlyTheirNPCs: on the cave in the guard-room
// fight, every response a player reads (the order, the turn, the targets, the
// log, the refusals) has only the NPCs their character sees, as the maps module
// says it sees them: Pensantus with her darkvision sees the Capitão, 11 squares
// away in the dark, and Toren with his torch does not; the master sees all; the
// party and its people are always there.
func TestRN10_FogCombatEachPlayerSeesOnlyTheirNPCs(t *testing.T) {
	t.Parallel()
	f := newFogCave(t)
	f.fight(t)
	f.stage(t)

	e := f.get(t, f.master)
	wantNPCs(t, "the master", e, allNPCs...)
	players := []struct {
		name   string
		u      *user
		sees   []string
		unseen []string
	}{
		{"Pensantus's player", f.ana, []string{"Capitão Goblin", "Escudeiro", "Goblin 2"}, []string{"Goblin 1", "Goblin 3", "Ogro"}},
		{"Toren's player", f.caio, []string{"Escudeiro", "Goblin 2"}, []string{"Capitão Goblin", "Goblin 1", "Goblin 3", "Ogro"}},
		{"Brisa's player", f.bia, []string{"Escudeiro", "Goblin 2"}, []string{"Capitão Goblin", "Goblin 1", "Goblin 3", "Ogro"}},
	}
	for _, p := range players {
		got := f.get(t, p.u)
		wantNPCs(t, p.name, got, p.sees...)
		// The party is always there.
		for _, pc := range []string{"Toren", "Pensantus", "Brisa"} {
			if byLabel(t, got, pc) == nil {
				t.Errorf("%s does not receive %s", p.name, pc)
			}
		}
		// The maps module agrees, square by square.
		for _, npc := range allNPCs {
			at := f.who(t, f.master, npc)
			seen := f.seesSquare(t, p.u, int(at.GetCol()), int(at.GetRow()))
			if listed := slices.Contains(npcLabels(got), npc); listed != seen {
				t.Errorf("%s: %s listed %v, but the map says seen %v", p.name, npc, listed, seen)
			}
		}
		f.noLeak(t, p.name+"'s encounter", asJSON(t, got), p.unseen...)
		f.noLeak(t, p.name+"'s log", asJSON(t, f.log(t, p.u, got)), p.unseen...)
	}

	// The targets Toren is offered on his turn.
	opts := f.mustOptions(t, f.caio, e, "Toren")
	for _, at := range opts.GetAttackTargets() {
		var got []string
		for _, tg := range at.GetTargets() {
			got = append(got, tg.GetLabel())
		}
		slices.Sort(got)
		if want := []string{"Brisa", "Escudeiro", "Goblin 2", "Pensantus"}; !slices.Equal(got, want) {
			t.Errorf("Toren's targets for %s = %v, want %v", at.GetAttackKey(), got, want)
		}
	}
	f.noLeak(t, "Toren's turn options", asJSON(t, opts), "Capitão Goblin", "Goblin 1", "Goblin 3", "Ogro")

	// Naming an NPC the player does not see is not found, as naming one that does not exist.
	for _, hidden := range []string{"Capitão Goblin", "Goblin 1", "Ogro"} {
		_, err := f.attack(t, f.caio, e, "Toren", battleaxe, hidden, d20(15))
		wantCode(t, "RollAttack on "+hidden, err, connect.CodeNotFound)
	}
	if _, err := f.options(t, f.caio, "Capitão Goblin"); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("GetMoveOptions of an NPC the player does not see = %v, want not_found", err)
	}
	// ...and a seen one is found (here refused only for another reason: it is not Toren's to move).
	if _, err := f.options(t, f.caio, "Goblin 2"); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("GetMoveOptions of an NPC the player sees = %v, want permission_denied", err)
	}

	// The turn of the NPCs: the master's group, and each player reads the members they see.
	for range 6 {
		cur := f.get(t, f.master)
		if cur.GetStatus() != playv1.EncounterStatus_ENCOUNTER_STATUS_ACTIVE {
			break
		}
		if byID(cur, cur.GetCurrentCombatantId()).GetKind() == playv1.CombatantKind_COMBATANT_KIND_NPC {
			break
		}
		f.mustEndTurn(t, f.master, cur)
	}
	e = f.get(t, f.master)
	if byID(e, e.GetCurrentCombatantId()).GetKind() != playv1.CombatantKind_COMBATANT_KIND_NPC {
		t.Fatalf("the turn did not reach the NPCs: %v", labels(e))
	}
	for _, p := range players {
		got := f.get(t, p.u)
		var group []string
		for _, id := range got.GetTurnGroupIds() {
			group = append(group, byID(got, id).GetLabel())
		}
		slices.Sort(group)
		want := slices.Clone(p.sees)
		slices.Sort(want)
		if !slices.Equal(group, want) {
			t.Errorf("%s reads the NPCs' turn as %v, want the ones they see %v", p.name, group, want)
		}
		f.noLeak(t, p.name+"'s turn", asJSON(t, got), p.unseen...)
	}
}

func byID(e *playv1.Encounter, id string) *playv1.Combatant {
	for _, c := range e.GetCombatants() {
		if c.GetId() == id {
			return c
		}
	}
	return &playv1.Combatant{}
}

// fogStreams are the four streams of the table, ready.
type fogStreams struct {
	master, caio, ana, bia *watcher
}

func (f *fogCave) watchAll(t *testing.T) fogStreams {
	t.Helper()
	s := fogStreams{f.master.watch(t, f.campaignID), f.caio.watch(t, f.campaignID), f.ana.watch(t, f.campaignID), f.bia.watch(t, f.campaignID)}
	for _, w := range []*watcher{s.master, s.caio, s.ana, s.bia} {
		w.ready(t)
	}
	return s
}

// collect reads a stream up to the marker: a move of Brisa, which every player
// hears, so everything the change before it published has come by then.
func (f *fogCave) collect(t *testing.T, w *watcher, marker int32) []*playv1.WatchGameSessionResponse {
	t.Helper()
	var out []*playv1.WatchGameSessionResponse
	for {
		ev := w.nextChange(t)
		if m := ev.GetCombatantMoved(); m != nil && m.GetCol() == marker && m.GetCombatantId() == f.id(t, "Brisa") {
			return out
		}
		out = append(out, ev)
	}
}

// markEnd moves Brisa to a column nobody else uses, the end of what a change published.
func (f *fogCave) markEnd(t *testing.T, col int32) {
	t.Helper()
	f.mustMove(t, f.master, "Brisa", col, 7)
}

func kinds(evs []*playv1.WatchGameSessionResponse) []string {
	var out []string
	for _, ev := range evs {
		switch {
		case ev.GetCombatantMoved() != nil:
			out = append(out, "combatant_moved")
		case ev.GetEncounterChanged() != nil:
			out = append(out, "encounter_changed")
		case ev.GetTurnChanged() != nil:
			out = append(out, "turn_changed")
		case ev.GetVisionChanged() != nil:
			out = append(out, "vision_changed")
		case ev.GetCombatLogChanged() != nil:
			out = append(out, "combat_log_changed")
		default:
			out = append(out, "other")
		}
	}
	return out
}

func asJSONAll(t *testing.T, evs []*playv1.WatchGameSessionResponse) string {
	t.Helper()
	var b strings.Builder
	for _, ev := range evs {
		b.WriteString(asJSON(t, ev))
	}
	return b.String()
}

// TestRN10_FogCombatGroupVisionAndTheStream: the stream tells each player of an
// NPC's move only if they see its new square (the others get the content-free
// encounter_changed), the turn is told as each player sees it, and "Visão do
// grupo" makes everyone see what the party sees.
func TestRN10_FogCombatGroupVisionAndTheStream(t *testing.T) {
	t.Parallel()
	f := newFogCave(t)
	f.fight(t)
	f.stage(t)

	// The Capitão steps in the corridor, still 10 squares from Pensantus and 9 from
	// Toren: only Pensantus sees the new square.
	s := f.watchAll(t)
	f.mustMove(t, f.master, "Capitão Goblin", 15, 7)
	f.markEnd(t, 3)
	captain := f.id(t, "Capitão Goblin")
	pens := f.collect(t, s.ana, 3)
	var moved *playv1.WatchGameSessionResponse_CombatantMoved
	for _, ev := range pens {
		if m := ev.GetCombatantMoved(); m != nil && m.GetCombatantId() == captain {
			moved = m
		}
	}
	if moved == nil || moved.GetCol() != 15 || moved.GetRow() != 7 {
		t.Errorf("Pensantus's player got %v for the Capitão's step, want combatant_moved to 15, 7", kinds(pens))
	}
	for name, w := range map[string]*watcher{"Toren's player": s.caio, "Brisa's player": s.bia} {
		got := f.collect(t, w, 3)
		if text := asJSONAll(t, got); strings.Contains(text, captain) || strings.Contains(text, "Capitão") {
			t.Errorf("%s's stream has the Capitão, whom they do not see: %s", name, text)
		}
		if !slices.Contains(kinds(got), "encounter_changed") || slices.Contains(kinds(got), "vision_changed") {
			t.Errorf("%s got %v for a move they do not see, want the content-free encounter_changed and no vision_changed", name, kinds(got))
		}
	}
	if got := f.collect(t, s.master, 3); !slices.Contains(kinds(got), "combatant_moved") {
		t.Errorf("the master got %v, want combatant_moved", kinds(got))
	}

	// The turn passes to the NPCs: each player is told the turn as they see it.
	s = f.watchAll(t)
	for range 6 {
		cur := f.get(t, f.master)
		if byID(cur, cur.GetCurrentCombatantId()).GetKind() == playv1.CombatantKind_COMBATANT_KIND_NPC {
			break
		}
		f.mustEndTurn(t, f.master, cur)
	}
	f.markEnd(t, 2)
	for name, w := range map[string]*watcher{"Toren's player": s.caio, "Brisa's player": s.bia, "Pensantus's player": s.ana} {
		got := f.collect(t, w, 2)
		if text := asJSONAll(t, got); strings.Contains(text, captain) || strings.Contains(text, f.id(t, "Goblin 1")) || strings.Contains(text, f.id(t, "Ogro")) {
			t.Errorf("%s's turn events have an NPC they do not see: %s", name, text)
		}
	}

	// "Visão do grupo": what Pensantus sees, everyone sees.
	f.groupVision(t, true)
	for name, u := range map[string]*user{"Toren's player": f.caio, "Brisa's player": f.bia} {
		wantNPCs(t, name+" with the group's sight", f.get(t, u), "Capitão Goblin", "Escudeiro", "Goblin 2")
	}
	f.groupVision(t, false)
	wantNPCs(t, "Toren's player without it", f.get(t, f.caio), "Escudeiro", "Goblin 2")
}

// TestRN10_FogCombatMovesPlanOnWhatThePlayerKnows: a player's reach treats the
// squares they do not see as floor, the actual move is cut short where a wall they
// did not see blocks it, the answer only says it was cut short, what the move
// showed is remembered, and only the player whose view changed hears of it.
func TestRN10_FogCombatMovesPlanOnWhatThePlayerKnows(t *testing.T) {
	t.Parallel()
	f := newFogCave(t)
	f.torch(t, false) // Toren walks in the dark
	// A wall in the corridor that Toren, with no light, does not see.
	f.paint(t, mapsv1.MapLayer_MAP_LAYER_WALL, 1, &mapsv1.MapSquare{Col: 9, Row: 7})
	f.fight(t)

	wallsOf := func(u *user) *grid.Layer {
		res, err := f.mapsAs(u).GetMapLayers(t.Context(), connect.NewRequest(&mapsv1.GetMapLayersRequest{CampaignId: f.campaignID, MapId: f.mapID}))
		if err != nil {
			t.Fatalf("GetMapLayers() error = %v", err)
		}
		l, err := grid.DecodeLayer(caveGridSize, res.Msg.GetWall())
		if err != nil {
			t.Fatalf("DecodeLayer() error = %v", err)
		}
		return l
	}
	if wallsOf(f.caio).Get(9, 7) {
		t.Fatalf("Toren knows the wall at 9, 7 before he saw it")
	}
	if !wallsOf(f.ana).Get(9, 7) {
		t.Fatalf("Pensantus, with her darkvision, does not know the wall at 9, 7")
	}

	// The reach: Toren's plan reaches through the wall, and no refusal names it.
	plan, err := f.options(t, f.caio, "Toren")
	if err != nil {
		t.Fatalf("GetMoveOptions(Toren) error = %v", err)
	}
	reaches := func(o *playv1.GetMoveOptionsResponse, col, row int32) bool {
		return slices.ContainsFunc(o.GetReachable(), func(r *playv1.ReachableSquare) bool { return r.GetCol() == col && r.GetRow() == row })
	}
	if !reaches(plan, 9, 7) || !reaches(plan, 12, 7) {
		t.Errorf("Toren's reach lacks 9, 7 and 12, 7: the squares he does not see are floor to him")
	}
	for _, r := range plan.GetRefused() {
		if r.GetCol() == 9 && r.GetRow() == 7 {
			t.Errorf("Toren's plan refuses 9, 7 (%v): it names a wall he has not seen", r.GetReason())
		}
	}
	actual, err := f.options(t, f.master, "Toren")
	if err != nil {
		t.Fatalf("GetMoveOptions(master) error = %v", err)
	}
	if reaches(actual, 9, 7) || reaches(actual, 12, 7) {
		t.Errorf("the master's reach goes through the wall")
	}
	if !slices.ContainsFunc(actual.GetRefused(), func(r *playv1.RefusedSquare) bool {
		return r.GetCol() == 9 && r.GetRow() == 7 && r.GetReason() == playv1.MoveRefusal_MOVE_REFUSAL_WALL
	}) {
		t.Errorf("the master's plan does not refuse 9, 7 as a wall: %v", actual.GetRefused())
	}

	// The move: cut short before the wall, with the cost of what was walked.
	s := f.watchAll(t)
	res, err := f.moveResponse(t, f.caio, "Toren", 12, 7)
	if err != nil {
		t.Fatalf("MoveCombatant(Toren to 12, 7) error = %v", err)
	}
	if !res.GetStoppedEarly() {
		t.Errorf("the move through a wall Toren did not see was not cut short")
	}
	if got := byLabel(t, res.GetEncounter(), "Toren"); got.GetCol() != 8 || got.GetRow() != 7 || got.GetMovementUsedDft() != 100 {
		t.Errorf("Toren stands on %d, %d with %d dft used, want 8, 7 and 100 (two squares)", got.GetCol(), got.GetRow(), got.GetMovementUsedDft())
	}
	f.noLeak(t, "the cut-short answer", asJSON(t, res), "Capitão Goblin")

	// What the move showed is remembered, and only Toren's player heard of it.
	if !wallsOf(f.caio).Get(9, 7) {
		t.Errorf("Toren did not remember the wall he stopped at")
	}
	f.markEnd(t, 2)
	for name, w := range map[string]*watcher{"Toren's player": s.caio, "Pensantus's player": s.ana, "Brisa's player": s.bia} {
		got := f.collect(t, w, 2)
		n := 0
		for _, k := range kinds(got) {
			if k == "vision_changed" {
				n++
			}
		}
		if want := map[string]int{"Toren's player": 1}[name]; n != want {
			t.Errorf("%s got %d vision_changed for Toren's move, want %d (%v)", name, n, want, kinds(got))
		}
	}

	// Now the wall is known: the plan refuses it, like the actual move.
	_, err = f.move(t, f.caio, "Toren", 12, 7)
	wantEncounterBlocked(t, err, reasonMoveBlocked)
}

// TestRN10_FogCombatAnUnseenCreatureCutsTheMoveShort: a creature the player does
// not see is floor to their plan and stops the actual move without a name; one they
// see is refused by name as before.
func TestRN10_FogCombatAnUnseenCreatureCutsTheMoveShort(t *testing.T) {
	t.Parallel()
	f := newFogCave(t)
	f.torch(t, false)
	f.fight(t)
	f.mustMove(t, f.master, "Capitão Goblin", 9, 7) // 3 squares from Toren, in the dark
	if len(npcLabels(f.get(t, f.caio))) != 0 {
		t.Fatalf("Toren sees %v in the dark", npcLabels(f.get(t, f.caio)))
	}
	plan, err := f.options(t, f.caio, "Toren")
	if err != nil {
		t.Fatalf("GetMoveOptions(Toren) error = %v", err)
	}
	if !slices.ContainsFunc(plan.GetReachable(), func(r *playv1.ReachableSquare) bool { return r.GetCol() == 9 && r.GetRow() == 7 }) {
		t.Errorf("the square of the creature Toren does not see is not in his reach")
	}
	for _, r := range plan.GetRefused() {
		if r.GetCol() == 9 && r.GetRow() == 7 {
			t.Errorf("Toren's plan refuses %d, %d as %v: it names a creature he does not see", r.GetCol(), r.GetRow(), r.GetReason())
		}
	}
	res, err := f.moveResponse(t, f.caio, "Toren", 12, 7)
	if err != nil {
		t.Fatalf("MoveCombatant() error = %v", err)
	}
	if got := byLabel(t, res.GetEncounter(), "Toren"); !res.GetStoppedEarly() || got.GetCol() != 8 {
		t.Errorf("Toren ends on %d, stopped early %v, want 8 and cut short before the creature he did not see", got.GetCol(), res.GetStoppedEarly())
	}
	f.noLeak(t, "the cut-short answer", asJSON(t, res), "Capitão Goblin")

	// With the torch lit he sees it, and the refusal names its kind as it always did.
	f2 := newFogCave(t)
	f2.fight(t)
	f2.mustMove(t, f2.master, "Capitão Goblin", 9, 7)
	_, err = f2.move(t, f2.caio, "Toren", 12, 7)
	wantEncounterBlocked(t, err, reasonEnemy)
}

// TestRN10_FogCombatTheLogKeepsWhoSawIt: each line of the log is the players' who
// saw its NPCs when it happened. A line from when Toren saw the goblin stays his
// after the goblin walks into the dark; a line from when nobody saw the Capitão
// never appears later, when the Capitão is in plain sight.
func TestRN10_FogCombatTheLogKeepsWhoSawIt(t *testing.T) {
	t.Parallel()
	f := newFogCave(t)
	e := f.fight(t)
	f.mustMove(t, f.master, "Goblin 2", 7, 7) // next to Toren, in his light

	// The master marks the Capitão in the south room, where nobody sees him.
	f.setConditions(t, e, "Capitão Goblin", "condition:prone")
	// Toren attacks the goblin he sees.
	if _, err := f.attack(t, f.caio, e, "Toren", battleaxe, "Goblin 2", d20(15)); err != nil {
		t.Fatalf("RollAttack() error = %v", err)
	}
	// The goblin runs into the dark, and the Capitão comes into the light.
	f.mustMove(t, f.master, "Goblin 2", 12, 12)
	f.mustMove(t, f.master, "Capitão Goblin", 7, 8)

	for name, u := range map[string]*user{"Toren's player": f.caio, "Pensantus's player": f.ana, "Brisa's player": f.bia} {
		got := f.get(t, u)
		wantNPCs(t, name, got, "Capitão Goblin", "Escudeiro")
		kindsSeen := map[playv1.CombatLogKind]string{}
		for _, round := range f.log(t, u, got).GetRounds() {
			for _, entry := range round.GetEntries() {
				kindsSeen[entry.GetKind()] = entry.GetTargetLabel()
			}
		}
		if kindsSeen[playv1.CombatLogKind_COMBAT_LOG_KIND_ATTACK] != "Goblin 2" {
			t.Errorf("%s lost the line of the attack on the goblin they saw: %v", name, kindsSeen)
		}
		if _, ok := kindsSeen[playv1.CombatLogKind_COMBAT_LOG_KIND_CONDITIONS_CHANGED]; ok {
			t.Errorf("%s got the line about the Capitão from when nobody saw him: %v", name, kindsSeen)
		}
	}
	// The master has both, and the one nobody saw says so.
	hiddenFromAll, shown := 0, 0
	for _, round := range f.log(t, f.master, f.get(t, f.master)).GetRounds() {
		for _, entry := range round.GetEntries() {
			switch {
			case entry.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_CONDITIONS_CHANGED && entry.GetHidden():
				hiddenFromAll++
			case entry.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_ATTACK && !entry.GetHidden():
				shown++
			}
		}
	}
	if hiddenFromAll != 1 || shown != 1 {
		t.Errorf("the master's log has %d lines hidden from the players and %d shown attacks, want 1 and 1", hiddenFromAll, shown)
	}
}

// darkNPC creates a dwarf NPC (darkvision 18 m) with a battleaxe.
func (u *user) darkNPC(t *testing.T, campaignID, name string) *charactersv1.Character {
	t.Helper()
	sheet := &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
		BaseScores: &rulesv1.AbilityScores{Strength: 14, Dexterity: 12, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8},
		RaceKey:    "race:dwarf", Classes: []*charactersv1.ClassLevel{{ClassKey: "class:fighter", Level: 1}}, WeaponKeys: []string{battleaxe},
	}}}
	res, err := u.characters.CreateCharacter(t.Context(), connect.NewRequest(&charactersv1.CreateCharacterRequest{
		CampaignId: campaignID, Kind: charactersv1.CharacterKind_CHARACTER_KIND_BOSS, Name: name, Sheet: sheet,
	}))
	if err != nil {
		t.Fatalf("CreateCharacter(%s) error = %v", name, err)
	}
	return res.Msg.GetCharacter()
}

// TestRN10_FogCombatReactorsMustSeeTheMover: leaving the reach of two goblins in
// the dark, a goblin with plain sight does not see Toren (no light) and does not
// provoke; a dwarf with darkvision does, and Toren's player is told it is waiting
// for the master but never its name.
func TestRN10_FogCombatReactorsMustSeeTheMover(t *testing.T) {
	t.Parallel()
	f := newFogCave(t)
	f.torch(t, false)
	dwarf := f.master.darkNPC(t, f.campaignID, "Anão")
	f.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: f.goblins.GetId(), Count: 1}, {CharacterId: dwarf.GetId()}},
		npcRolls: []int{2, 2},
		players:  map[string]int32{"Toren": 18, "Pensantus": 10, "Brisa": 1},
		reveal:   []string{"Goblin", "Anão"},
		at:       map[string][2]int32{"Toren": {6, 7}, "Pensantus": {5, 8}, "Brisa": {4, 7}, "Goblin": {6, 6}, "Anão": {7, 7}},
	})
	if got := npcLabels(f.get(t, f.caio)); len(got) != 0 {
		t.Fatalf("Toren sees %v in the dark", got)
	}
	// His warning (the squares that provoke) names nobody he does not see.
	warn, err := f.options(t, f.caio, "Toren")
	if err != nil {
		t.Fatalf("GetMoveOptions(Toren) error = %v", err)
	}
	for _, r := range warn.GetReachable() {
		if len(r.GetProvokesReactorIds()) != 0 {
			t.Errorf("Toren's plan warns of %v at %d, %d: reactors he does not see", r.GetProvokesReactorIds(), r.GetCol(), r.GetRow())
		}
	}

	res, err := f.moveResponse(t, f.caio, "Toren", 4, 6)
	if err != nil {
		t.Fatalf("MoveCombatant(Toren to 4, 6) error = %v", err)
	}
	if !res.GetProvoked() {
		t.Errorf("the dwarf, who sees in the dark, was not offered the attack")
	}
	offers := f.get(t, f.master).GetOpportunityOffers()
	if len(offers) != 1 || offers[0].GetReactorLabel() != "Anão" {
		t.Fatalf("the master's offers = %v, want only the dwarf's: the goblin does not see Toren in the dark", offers)
	}
	mine := f.get(t, f.caio).GetOpportunityOffers()
	if len(mine) != 1 || mine[0].GetMoverId() != f.id(t, "Toren") || mine[0].GetReactorId() != "" || mine[0].GetReactorLabel() != "" {
		t.Errorf("Toren's player reads %v, want the wait with no name for the reactor", mine)
	}
	f.noLeak(t, "Toren's encounter", asJSON(t, f.get(t, f.caio)), "Anão", "Goblin")
	f.noLeak(t, "Toren's answer", asJSON(t, res), "Anão", "Goblin")
}
