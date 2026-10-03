package maps

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	notesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/notes/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The master's hooks and clues, the discoveries and the players' private notes
// (MR-029, MR-030, RN-20; questions 59 to 61 and 63 with their defaults). The
// tests live here because this harness has the maps, the characters, play and
// the notes together, and the tests that prove privacy read what the app
// receives: the responses and the stream events as JSON.

func (u *user) tryAddClue(campaignID, mapID, pointID, text string) (*mapsv1.AddSceneClueResponse, error) {
	res, err := u.maps.AddSceneClue(u.h.t.Context(), connect.NewRequest(&mapsv1.AddSceneClueRequest{CampaignId: campaignID, MapId: mapID, PointId: pointID, Text: text}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func (u *user) addClue(campaignID, mapID, pointID, text string) *mapsv1.SceneClue {
	u.h.t.Helper()
	res, err := u.tryAddClue(campaignID, mapID, pointID, text)
	if err != nil {
		u.h.t.Fatalf("AddSceneClue() error = %v", err)
	}
	return res.GetClue()
}

func (u *user) revealClue(campaignID, clueID string, characterIDs ...string) (*mapsv1.SceneClue, error) {
	res, err := u.maps.RevealSceneClue(u.h.t.Context(), connect.NewRequest(&mapsv1.RevealSceneClueRequest{CampaignId: campaignID, ClueId: clueID, CharacterIds: characterIDs}))
	if err != nil {
		return nil, err
	}
	return res.Msg.GetClue(), nil
}

func (u *user) mustRevealClue(campaignID, clueID string, characterIDs ...string) *mapsv1.SceneClue {
	u.h.t.Helper()
	clue, err := u.revealClue(campaignID, clueID, characterIDs...)
	if err != nil {
		u.h.t.Fatalf("RevealSceneClue() error = %v", err)
	}
	return clue
}

func (u *user) updatePoint(req *mapsv1.UpdateMapPointRequest) (*mapsv1.MapPoint, error) {
	res, err := u.maps.UpdateMapPoint(u.h.t.Context(), connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return res.Msg.GetPoint(), nil
}

func (s *scenes) setHooks(point *mapsv1.MapPoint, hooks string) (*mapsv1.MapPoint, error) {
	return s.master.updatePoint(&mapsv1.UpdateMapPointRequest{CampaignId: s.campaign, MapId: point.GetMapId(), PointId: point.GetId(), Hooks: &hooks})
}

func (u *user) tryListNotes(campaignID, scene string) (*notesv1.ListNotesResponse, error) {
	res, err := u.notes.ListNotes(u.h.t.Context(), connect.NewRequest(&notesv1.ListNotesRequest{CampaignId: campaignID, ScenePointId: scene}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func (u *user) listNotes(campaignID, scene string) *notesv1.ListNotesResponse {
	u.h.t.Helper()
	res, err := u.tryListNotes(campaignID, scene)
	if err != nil {
		u.h.t.Fatalf("ListNotes() error = %v", err)
	}
	return res
}

func (u *user) tryCreateNote(campaignID, text, scene string) (*notesv1.Note, error) {
	res, err := u.notes.CreateNote(u.h.t.Context(), connect.NewRequest(&notesv1.CreateNoteRequest{CampaignId: campaignID, Text: text, ScenePointId: scene}))
	if err != nil {
		return nil, err
	}
	return res.Msg.GetNote(), nil
}

func (u *user) createNote(campaignID, text, scene string) *notesv1.Note {
	u.h.t.Helper()
	n, err := u.tryCreateNote(campaignID, text, scene)
	if err != nil {
		u.h.t.Fatalf("CreateNote() error = %v", err)
	}
	return n
}

func (u *user) noteScenes(campaignID string) []*notesv1.NoteScene {
	u.h.t.Helper()
	res, err := u.notes.ListNoteScenes(u.h.t.Context(), connect.NewRequest(&notesv1.ListNoteScenesRequest{CampaignId: campaignID}))
	if err != nil {
		u.h.t.Fatalf("ListNoteScenes() error = %v", err)
	}
	return res.Msg.GetScenes()
}

func noteTexts(res *notesv1.ListNotesResponse) []string {
	var out []string
	for _, n := range res.GetNotes() {
		out = append(out, n.GetText())
	}
	return out
}

// masterPointOf reads the point as the master's GetMap gives it.
func (s *scenes) masterPointOf(id string) *mapsv1.MapPoint {
	s.h.t.Helper()
	for _, p := range s.master.mustGetMap(s.campaign, s.mapID).GetPoints() {
		if p.GetId() == id {
			return p
		}
	}
	s.h.t.Fatalf("point %s is not on the master's map", id)
	return nil
}

// drain reads the stream until the map_changed of probeMapID, and returns
// everything before it: the master makes that change last, and it reaches the
// players, so what is before it is all they were sent (the hub keeps order).
func (w *watcher) drain(probeMapID string) []*playv1.WatchGameSessionResponse {
	w.t.Helper()
	var out []*playv1.WatchGameSessionResponse
	for {
		ev := w.next()
		if ev.GetMapChanged().GetMapId() == probeMapID {
			return out
		}
		out = append(out, ev)
	}
}

// probe makes a change to a map the players see, to end a drain.
func (s *scenes) probe(probeMapID string) {
	s.h.t.Helper()
	s.master.setMapRevealed(s.campaign, probeMapID, true)
}

func asJSON(t *testing.T, m proto.Message) string {
	t.Helper()
	b, err := protojson.Marshal(m)
	if err != nil {
		t.Fatalf("protojson.Marshal() error = %v", err)
	}
	return string(b)
}

// MR-029, first criteria: the master writes hooks on a scene point and keeps
// clues on it, each with who has it; the master reads both in the map and in
// the open scene; the limits hold; hooks and clues exist only on a SCENE point.
func TestMR029_TheMastersHooksAndClues(t *testing.T) {
	t.Parallel()
	s := newScenes(t, true)

	// Hooks save with the point; only the master reads them, even once the
	// point is revealed.
	const hooks = "## Ganchos\n- A carroça **não** caiu sozinha."
	point, err := s.setHooks(s.point, hooks)
	if err != nil {
		t.Fatalf("UpdateMapPoint(hooks) error = %v", err)
	}
	if point.GetHooks() != hooks {
		t.Errorf("the saved hooks = %q, want %q", point.GetHooks(), hooks)
	}
	s.master.setPointRevealed(s.campaign, s.point, true)
	if got := s.masterPointOf(s.point.GetId()).GetHooks(); got != hooks {
		t.Errorf("the master's GetMap hooks = %q", got)
	}
	if got := s.ana.mustGetMap(s.campaign, s.mapID).GetPoints()[0].GetHooks(); got != "" {
		t.Errorf("Ana's GetMap hooks = %q, want none", got)
	}
	if _, err := s.setHooks(s.point, ""); err != nil {
		t.Errorf("clearing the hooks error = %v", err)
	}
	if _, err := s.setHooks(s.point, strings.Repeat("g", 4001)); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("4001 characters of hooks error = %v, want invalid_argument", err)
	}
	if _, err := s.setHooks(s.point, strings.Repeat("ç", 4000)); err != nil {
		t.Errorf("4000 characters of hooks error = %v, want it saved (characters, not bytes)", err)
	}

	// Hooks and clues only on a SCENE point.
	battle := s.master.createPoint(&mapsv1.CreateMapPointRequest{CampaignId: s.campaign, MapId: s.mapID, Kind: mapsv1.MapPointKind_MAP_POINT_KIND_BATTLE, Name: "Emboscada"})
	if _, err := s.setHooks(battle, "x"); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("hooks on a battle point error = %v, want invalid_argument", err)
	}
	_, err = s.master.maps.CreateMapPoint(t.Context(), connect.NewRequest(&mapsv1.CreateMapPointRequest{
		CampaignId: s.campaign, MapId: s.mapID, Kind: mapsv1.MapPointKind_MAP_POINT_KIND_SUBMAP, Name: "Torre", Hooks: "x",
	}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("creating a submap point with hooks error = %v, want invalid_argument", err)
	}
	if _, err := s.master.tryAddClue(s.campaign, s.mapID, battle.GetId(), "x"); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("a clue on a battle point error = %v, want invalid_argument", err)
	}
	created := s.master.createPoint(&mapsv1.CreateMapPointRequest{
		CampaignId: s.campaign, MapId: s.mapID, Kind: mapsv1.MapPointKind_MAP_POINT_KIND_SCENE, Name: "Poço seco", Hooks: "Ganchos de nascença",
	})
	if created.GetHooks() != "Ganchos de nascença" {
		t.Errorf("a new scene point's hooks = %q", created.GetHooks())
	}

	// Clues: added last, edited, moved, removed; each answer is the whole list.
	pid := s.point.GetId()
	c1 := s.master.addClue(s.campaign, s.mapID, pid, "Marcas de garras na madeira")
	c2 := s.master.addClue(s.campaign, s.mapID, pid, "O cocheiro sumiu")
	c3 := s.master.addClue(s.campaign, s.mapID, pid, "Uma bota pesada\nna lama")
	if len(c1.GetRevealedTo()) != 0 {
		t.Errorf("a new clue is revealed to %v, want nobody (Ninguém ainda)", c1.GetRevealedTo())
	}
	clueTexts := func(clues []*mapsv1.SceneClue) string {
		var out []string
		for _, c := range clues {
			out = append(out, c.GetText())
		}
		return strings.Join(out, "|")
	}
	moveRes, err := s.master.maps.MoveSceneClue(t.Context(), connect.NewRequest(&mapsv1.MoveSceneClueRequest{
		CampaignId: s.campaign, MapId: s.mapID, PointId: pid, ClueId: c3.GetId(), Direction: mapsv1.SceneActionDirection_SCENE_ACTION_DIRECTION_UP,
	}))
	if err != nil {
		t.Fatalf("MoveSceneClue() error = %v", err)
	}
	if got, want := clueTexts(moveRes.Msg.GetClues()), "Marcas de garras na madeira|Uma bota pesada\nna lama|O cocheiro sumiu"; got != want {
		t.Errorf("clues after moving the third up = %q, want %q", got, want)
	}
	upd, err := s.master.maps.UpdateSceneClue(t.Context(), connect.NewRequest(&mapsv1.UpdateSceneClueRequest{
		CampaignId: s.campaign, MapId: s.mapID, PointId: pid, ClueId: c2.GetId(), Text: "O cocheiro fugiu",
	}))
	if err != nil || upd.Msg.GetClue().GetText() != "O cocheiro fugiu" {
		t.Fatalf("UpdateSceneClue() = %v, %v", upd, err)
	}
	if _, err := s.master.maps.RemoveSceneClue(t.Context(), connect.NewRequest(&mapsv1.RemoveSceneClueRequest{
		CampaignId: s.campaign, MapId: s.mapID, PointId: pid, ClueId: c1.GetId(),
	})); err != nil {
		t.Fatalf("RemoveSceneClue() error = %v", err)
	}
	if got, want := clueTexts(s.masterPointOf(pid).GetClues()), "Uma bota pesada\nna lama|O cocheiro fugiu"; got != want {
		t.Errorf("the master's GetMap clues = %q, want %q", got, want)
	}
	// A clue that is not on the point, or not a UUID, is not found.
	_, err = s.master.maps.RemoveSceneClue(t.Context(), connect.NewRequest(&mapsv1.RemoveSceneClueRequest{CampaignId: s.campaign, MapId: s.mapID, PointId: pid, ClueId: c1.GetId()}))
	wantCode(t, "removing a clue twice", err, connect.CodeNotFound)
	_, err = s.master.maps.UpdateSceneClue(t.Context(), connect.NewRequest(&mapsv1.UpdateSceneClueRequest{CampaignId: s.campaign, MapId: s.mapID, PointId: pid, ClueId: "nope", Text: "x"}))
	wantCode(t, "updating a clue that is not a UUID", err, connect.CodeNotFound)

	// The open scene carries the hooks and the clues for the master only.
	if _, err := s.setHooks(s.point, hooks); err != nil {
		t.Fatal(err)
	}
	scene := s.master.mustOpenScene(s.campaign, pid)
	if scene.GetHooks() != hooks || clueTexts(scene.GetClues()) != "Uma bota pesada\nna lama|O cocheiro fugiu" {
		t.Errorf("the master's open scene hooks %q, clues %q", scene.GetHooks(), clueTexts(scene.GetClues()))
	}
	if got := s.ana.getScene(s.campaign); got.GetHooks() != "" || len(got.GetClues()) != 0 {
		t.Errorf("Ana's open scene has hooks %q and clues %v, want none", got.GetHooks(), got.GetClues())
	}

	// A scene stops being a scene: its hooks and clues go with the actions.
	if _, err := s.master.updatePoint(&mapsv1.UpdateMapPointRequest{
		CampaignId: s.campaign, MapId: s.mapID, PointId: pid, Kind: mapsv1.MapPointKind_MAP_POINT_KIND_BATTLE.Enum(),
	}); err != nil {
		t.Fatalf("changing the kind error = %v", err)
	}
	if p := s.masterPointOf(pid); p.GetHooks() != "" || len(p.GetClues()) != 0 {
		t.Errorf("the point that stopped being a scene keeps hooks %q and clues %v", p.GetHooks(), p.GetClues())
	}
}

// The limits: 30 clues per scene, 500 characters each.
func TestMR029_ClueLimits(t *testing.T) {
	t.Parallel()
	s := newScenes(t, false)
	pid := s.point.GetId()
	for _, bad := range []string{"", "   ", strings.Repeat("p", 501), "a\x00b"} {
		_, err := s.master.tryAddClue(s.campaign, s.mapID, pid, bad)
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("AddSceneClue(%d characters) error = %v, want invalid_argument", utf8.RuneCountInString(bad), err)
		}
	}
	if clue := s.master.addClue(s.campaign, s.mapID, pid, strings.Repeat("é", 500)); utf8.RuneCountInString(clue.GetText()) != 500 {
		t.Error("a 500-character clue was not kept whole")
	}
	for i := 1; i < 30; i++ {
		s.master.addClue(s.campaign, s.mapID, pid, "pista")
	}
	_, err := s.master.tryAddClue(s.campaign, s.mapID, pid, "a trigésima primeira")
	wantCode(t, "the 31st clue", err, connect.CodeResourceExhausted)
	if n := len(s.masterPointOf(pid).GetClues()); n != 30 {
		t.Errorf("the scene has %d clues, want 30", n)
	}
	// Another scene has its own 30.
	other := s.master.createPoint(&mapsv1.CreateMapPointRequest{CampaignId: s.campaign, MapId: s.mapID, Kind: mapsv1.MapPointKind_MAP_POINT_KIND_SCENE, Name: "Outra"})
	s.master.addClue(s.campaign, s.mapID, other.GetId(), "pista")
}

// MR-029, question 59: the master reveals a clue to the players it picks. They
// get it once, in their notes, whether they are online or not; the streams of
// the others hear nothing; it is recorded as an event with ids only; there is
// no way back; a clue removed or edited later changes nothing for them.
func TestMR029_ARevealedClueReachesOnlyTheChosenPlayers(t *testing.T) {
	t.Parallel()
	s := newScenes(t, true)
	pid := s.point.GetId()
	clue := s.master.addClue(s.campaign, s.mapID, pid, "Marcas de garras na madeira")
	probeMap := s.master.createMap(s.campaign, "Sonda", s.master.newImage(s.campaign)).GetId()

	anaWatch, caioWatch := s.ana.watch(s.campaign), s.caio.watch(s.campaign)
	revealed := s.master.mustRevealClue(s.campaign, clue.GetId(), s.pens.GetId())
	if len(revealed.GetRevealedTo()) != 1 || revealed.GetRevealedTo()[0].GetCharacterId() != s.pens.GetId() || revealed.GetRevealedTo()[0].GetCharacterName() != "Pensantus" {
		t.Errorf("the clue is revealed to %v, want Pensantus only (Só a Pensantus)", revealed.GetRevealedTo())
	}

	// Ana: notes_changed, and the clue in her notes as the master's, tagged
	// with the scene once the group discovered it (opening it does).
	if ev := anaWatch.next(); ev.GetNotesChanged() == nil {
		t.Errorf("Ana's event = %v, want notes_changed", ev)
	}
	notes := s.ana.listNotes(s.campaign, "")
	if len(notes.GetNotes()) != 1 || notes.GetNotes()[0].GetKind() != notesv1.NoteKind_NOTE_KIND_CLUE || notes.GetNotes()[0].GetText() != "Marcas de garras na madeira" {
		t.Fatalf("Ana's notes = %v, want the clue", notes.GetNotes())
	}
	if notes.GetNoteCount() != 0 {
		t.Errorf("Ana's note_count = %d, want 0: a clue is not a note", notes.GetNoteCount())
	}
	if got := notes.GetNotes()[0]; got.GetScenePointId() != "" {
		t.Errorf("the clue is tagged with %q before the scene is discovered, want no tag", got.GetScenePointId())
	}
	s.master.mustOpenScene(s.campaign, pid)
	if got := s.ana.listNotes(s.campaign, "").GetNotes()[0]; got.GetScenePointId() != pid || got.GetSceneName() != "A carroça tombada" {
		t.Errorf("the clue's tag after the scene was opened = %q / %q", got.GetScenePointId(), got.GetSceneName())
	}

	// Caio: nothing in the stream, nothing in his notes.
	s.probe(probeMap)
	if got := caioWatch.drain(probeMap); hasNotesChanged(got) {
		t.Errorf("Caio's stream got %v, want no notes_changed", got)
	}
	if got := s.caio.listNotes(s.campaign, ""); len(got.GetNotes()) != 0 {
		t.Errorf("Caio's notes = %v, want none", got.GetNotes())
	}
	// Ana got no other notes_changed than the first.
	if got := anaWatch.drain(probeMap); hasNotesChanged(got) {
		t.Errorf("Ana got a second notes_changed: %v", got)
	}

	// Idempotent: revealing again to her changes nothing; to both adds Caio.
	eventsBefore := s.clueEvents()
	again := s.master.mustRevealClue(s.campaign, clue.GetId(), s.pens.GetId())
	if len(again.GetRevealedTo()) != 1 || len(s.ana.listNotes(s.campaign, "").GetNotes()) != 1 {
		t.Errorf("revealing twice: revealed_to %v, Ana's notes %d; want one each", again.GetRevealedTo(), len(s.ana.listNotes(s.campaign, "").GetNotes()))
	}
	if got := s.clueEvents(); len(got) != len(eventsBefore) {
		t.Errorf("a repeated reveal wrote an event: %v", got)
	}
	both := s.master.mustRevealClue(s.campaign, clue.GetId(), s.pens.GetId(), s.other.GetId(), s.other.GetId())
	if len(both.GetRevealedTo()) != 2 {
		t.Errorf("revealed_to = %v, want both players (Todos)", both.GetRevealedTo())
	}
	s.probe(probeMap) // a no-op map change still tells the players: harmless
	events := s.clueEvents()
	if len(events) != 2 {
		t.Fatalf("clue_revealed events = %v, want 2 (the first reveal, and Caio's)", events)
	}
	for _, e := range events {
		if strings.Contains(e, "garras") || strings.Contains(e, "Pensantus") || strings.Contains(e, "Toren") {
			t.Errorf("the event %q carries the clue's text or a name; want ids only", e)
		}
	}
	if !strings.Contains(events[1], s.other.GetId()) || strings.Contains(events[1], s.pens.GetId()) {
		t.Errorf("the second event %q should name Toren only, who was the only new one", events[1])
	}

	// Editing or removing the clue changes nothing for those who have it.
	if _, err := s.master.maps.UpdateSceneClue(t.Context(), connect.NewRequest(&mapsv1.UpdateSceneClueRequest{CampaignId: s.campaign, MapId: s.mapID, PointId: pid, ClueId: clue.GetId(), Text: "Outro texto"})); err != nil {
		t.Fatal(err)
	}
	if got := s.ana.listNotes(s.campaign, "").GetNotes()[0].GetText(); got != "Marcas de garras na madeira" {
		t.Errorf("Ana's clue after the master edited it = %q, want what she was given", got)
	}
	if _, err := s.master.maps.RemoveSceneClue(t.Context(), connect.NewRequest(&mapsv1.RemoveSceneClueRequest{CampaignId: s.campaign, MapId: s.mapID, PointId: pid, ClueId: clue.GetId()})); err != nil {
		t.Fatal(err)
	}
	if got := s.caio.listNotes(s.campaign, ""); len(got.GetNotes()) != 1 || got.GetNotes()[0].GetText() != "Marcas de garras na madeira" {
		t.Errorf("Caio's notes after the master removed the clue = %v, want it kept", got.GetNotes())
	}
}

func hasNotesChanged(events []*playv1.WatchGameSessionResponse) bool {
	for _, ev := range events {
		if ev.GetNotesChanged() != nil {
			return true
		}
	}
	return false
}

// clueEvents reads the clue_revealed session events as "payload" lines.
func (s *scenes) clueEvents() []string {
	s.h.t.Helper()
	rows, err := s.h.pool.Query(s.h.t.Context(), `SELECT payload::TEXT FROM session_events WHERE kind = 'clue_revealed' ORDER BY seq`)
	if err != nil {
		s.h.t.Fatalf("read the clue events: %v", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			s.h.t.Fatal(err)
		}
		out = append(out, payload)
	}
	return out
}

// A reveal needs player characters of the campaign, and works with no session
// (the player reads it next time, offline or not): no event, no stream.
func TestMR029_RevealRules(t *testing.T) {
	t.Parallel()
	s := newScenes(t, false) // no session
	pid := s.point.GetId()
	clue := s.master.addClue(s.campaign, s.mapID, pid, "Uma pista")
	npc := s.master.createCharacter(s.campaign, charactersv1.CharacterKind_CHARACTER_KIND_MINION, "Goblin")

	for name, ids := range map[string][]string{
		"no characters": nil, "an NPC": {npc.GetId()}, "a stranger": {newKey()}, "not a UUID": {"x"}, "one of two is an NPC": {s.pens.GetId(), npc.GetId()},
	} {
		_, err := s.master.revealClue(s.campaign, clue.GetId(), ids...)
		want := connect.CodeNotFound
		if name == "no characters" {
			want = connect.CodeInvalidArgument
		}
		if connect.CodeOf(err) != want {
			t.Errorf("revealing to %s error = %v, want %v", name, err, want)
		}
	}
	if got := s.ana.listNotes(s.campaign, ""); len(got.GetNotes()) != 0 {
		t.Errorf("a refused reveal left %v in Ana's notes", got.GetNotes())
	}
	_, err := s.master.revealClue(s.campaign, newKey(), s.pens.GetId())
	wantCode(t, "a clue that does not exist", err, connect.CodeNotFound)
	_, err = s.master.revealClue(s.campaign, "nope", s.pens.GetId())
	wantCode(t, "a clue that is not a UUID", err, connect.CodeNotFound)
	tooMany := make([]string, 51)
	for i := range tooMany {
		tooMany[i] = s.pens.GetId()
	}
	_, err = s.master.revealClue(s.campaign, clue.GetId(), tooMany...)
	wantCode(t, "51 characters", err, connect.CodeInvalidArgument)

	// Another campaign's clue is not found, even for its own master.
	otherCampaign := s.h.newCampaign(s.master)
	om := s.master.createMap(otherCampaign, "Outra mesa", s.master.newImage(otherCampaign))
	foreign := s.master.createPoint(&mapsv1.CreateMapPointRequest{CampaignId: otherCampaign, MapId: om.GetId(), Kind: mapsv1.MapPointKind_MAP_POINT_KIND_SCENE, Name: "Alheia"})
	foreignClue := s.master.addClue(otherCampaign, om.GetId(), foreign.GetId(), "Alheia")
	_, err = s.master.revealClue(s.campaign, foreignClue.GetId(), s.pens.GetId())
	wantCode(t, "another campaign's clue", err, connect.CodeNotFound)

	// With no session the reveal works and writes no event.
	s.master.mustRevealClue(s.campaign, clue.GetId(), s.pens.GetId())
	if got := s.ana.listNotes(s.campaign, ""); len(got.GetNotes()) != 1 {
		t.Errorf("Ana's notes = %v, want the clue", got.GetNotes())
	}
	if got := s.clueEvents(); len(got) != 0 {
		t.Errorf("a reveal with no session wrote events %v", got)
	}
	// A reveal of a hidden, never opened scene's clue still arrives, with no
	// tag: the scene is not discovered, so its name stays secret.
	if got := s.ana.listNotes(s.campaign, "").GetNotes()[0]; got.GetScenePointId() != "" || got.GetSceneName() != "" {
		t.Errorf("the clue of an undiscovered scene is tagged %q / %q", got.GetScenePointId(), got.GetSceneName())
	}
}

// MR-030: notes are private. Not the master, and not another player, can read,
// change or delete one; the master has no notes to list.
func TestMR030_PlayerNotesArePrivate(t *testing.T) {
	t.Parallel()
	s := newScenes(t, false)
	note := s.ana.createNote(s.campaign, "Desconfio do cocheiro", "")
	if note.GetKind() != notesv1.NoteKind_NOTE_KIND_NOTE || note.GetText() != "Desconfio do cocheiro" {
		t.Errorf("the new note = %v", note)
	}
	if got := noteTexts(s.ana.listNotes(s.campaign, "")); len(got) != 1 || got[0] != "Desconfio do cocheiro" {
		t.Errorf("Ana's notes = %v", got)
	}

	// Another player: nothing in the list, and not_found for the ID.
	if got := s.caio.listNotes(s.campaign, ""); len(got.GetNotes()) != 0 || got.GetNoteCount() != 0 {
		t.Errorf("Caio's notes = %v, want none", got.GetNotes())
	}
	newText := "Reescrito"
	for name, u := range map[string]*user{"another player": s.caio, "the master": s.master} {
		_, err := u.notes.UpdateNote(t.Context(), connect.NewRequest(&notesv1.UpdateNoteRequest{CampaignId: s.campaign, NoteId: note.GetId(), Text: &newText}))
		wantCode(t, name+" updating Ana's note", err, connect.CodeNotFound)
		_, err = u.notes.DeleteNote(t.Context(), connect.NewRequest(&notesv1.DeleteNoteRequest{CampaignId: s.campaign, NoteId: note.GetId()}))
		wantCode(t, name+" deleting Ana's note", err, connect.CodeNotFound)
		_, err = u.tryListNotes(s.campaign, "")
		if name == "the master" {
			wantCode(t, name+" listing notes", err, connect.CodeNotFound)
		}
		_, err = u.notes.ListNoteScenes(t.Context(), connect.NewRequest(&notesv1.ListNoteScenesRequest{CampaignId: s.campaign}))
		if name == "the master" {
			wantCode(t, name+" listing the tag scenes", err, connect.CodeNotFound)
		}
		_, err = u.tryCreateNote(s.campaign, "x", "")
		if name == "the master" {
			wantCode(t, name+" creating a note", err, connect.CodeNotFound)
		}
	}
	if got := noteTexts(s.ana.listNotes(s.campaign, "")); len(got) != 1 || got[0] != "Desconfio do cocheiro" {
		t.Errorf("Ana's note changed after the refusals: %v", got)
	}
	// The same refusals are what an ID that does not exist gets.
	_, err := s.ana.notes.DeleteNote(t.Context(), connect.NewRequest(&notesv1.DeleteNoteRequest{CampaignId: s.campaign, NoteId: newKey()}))
	wantCode(t, "deleting a note that does not exist", err, connect.CodeNotFound)

	// The owner changes and deletes it.
	res, err := s.ana.notes.UpdateNote(t.Context(), connect.NewRequest(&notesv1.UpdateNoteRequest{CampaignId: s.campaign, NoteId: note.GetId(), Text: &newText}))
	if err != nil || res.Msg.GetNote().GetText() != "Reescrito" {
		t.Fatalf("UpdateNote() = %v, %v", res, err)
	}
	_, err = s.ana.notes.UpdateNote(t.Context(), connect.NewRequest(&notesv1.UpdateNoteRequest{CampaignId: s.campaign, NoteId: note.GetId()}))
	wantCode(t, "updating nothing", err, connect.CodeInvalidArgument)
	if _, err := s.ana.notes.DeleteNote(t.Context(), connect.NewRequest(&notesv1.DeleteNoteRequest{CampaignId: s.campaign, NoteId: note.GetId()})); err != nil {
		t.Fatalf("DeleteNote() error = %v", err)
	}
	if got := s.ana.listNotes(s.campaign, ""); len(got.GetNotes()) != 0 {
		t.Errorf("Ana's notes after deleting = %v", got.GetNotes())
	}

	// A received clue is not a note: it cannot be edited or deleted.
	clue := s.master.addClue(s.campaign, s.mapID, s.point.GetId(), "Uma pista")
	s.master.mustRevealClue(s.campaign, clue.GetId(), s.pens.GetId())
	entry := s.ana.listNotes(s.campaign, "").GetNotes()[0]
	_, err = s.ana.notes.UpdateNote(t.Context(), connect.NewRequest(&notesv1.UpdateNoteRequest{CampaignId: s.campaign, NoteId: entry.GetId(), Text: &newText}))
	wantCode(t, "editing a clue", err, connect.CodeNotFound)
	_, err = s.ana.notes.DeleteNote(t.Context(), connect.NewRequest(&notesv1.DeleteNoteRequest{CampaignId: s.campaign, NoteId: entry.GetId()}))
	wantCode(t, "deleting a clue", err, connect.CodeNotFound)
	if got := s.ana.listNotes(s.campaign, ""); len(got.GetNotes()) != 1 {
		t.Errorf("Ana's clue is gone: %v", got.GetNotes())
	}
}

// The limits: 2,000 characters a note, 300 notes a player per campaign (the
// clues do not count), and each player counts their own.
func TestMR030_NoteLimits(t *testing.T) {
	t.Parallel()
	s := newScenes(t, false)
	for _, bad := range []string{"", "  \n ", strings.Repeat("n", 2001), "a\x00b"} {
		_, err := s.ana.tryCreateNote(s.campaign, bad, "")
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("CreateNote(%d characters) error = %v, want invalid_argument", utf8.RuneCountInString(bad), err)
		}
	}
	long := strings.Repeat("ã", 1998) + "\n"
	if n := s.ana.createNote(s.campaign, long+"x", ""); utf8.RuneCountInString(n.GetText()) != 2000 {
		t.Errorf("a 2000-character note was kept as %d characters", utf8.RuneCountInString(n.GetText()))
	}

	// 299 more, written straight into the table to keep the test quick, then
	// the 300th through the API, and the 301st refused.
	if _, err := s.h.pool.Exec(t.Context(), `
		INSERT INTO player_notes (campaign_id, author_user_id, text, created_at, updated_at)
		SELECT $1, $2, 'nota ' || i::TEXT, now(), now() FROM generate_series(1, 298) AS i`, s.campaign, s.ana.id); err != nil {
		t.Fatalf("insert the notes: %v", err)
	}
	if n := s.ana.listNotes(s.campaign, "").GetNoteCount(); n != 299 {
		t.Fatalf("note_count = %d, want 299", n)
	}
	// Clues do not count.
	clue := s.master.addClue(s.campaign, s.mapID, s.point.GetId(), "Uma pista")
	s.master.mustRevealClue(s.campaign, clue.GetId(), s.pens.GetId())
	s.ana.createNote(s.campaign, "a trecentésima", "")
	_, err := s.ana.tryCreateNote(s.campaign, "a seguinte", "")
	wantCode(t, "the 301st note", err, connect.CodeResourceExhausted)
	list := s.ana.listNotes(s.campaign, "")
	if list.GetNoteCount() != 300 || list.GetMaxNotes() != 300 || len(list.GetNotes()) != 301 {
		t.Errorf("note_count %d, max_notes %d, entries %d; want 300, 300 and 301 (with the clue)", list.GetNoteCount(), list.GetMaxNotes(), len(list.GetNotes()))
	}
	// Deleting one makes room, and another player has their own 300.
	if _, err := s.ana.notes.DeleteNote(t.Context(), connect.NewRequest(&notesv1.DeleteNoteRequest{CampaignId: s.campaign, NoteId: list.GetNotes()[1].GetId()})); err != nil {
		t.Fatal(err)
	}
	s.ana.createNote(s.campaign, "cabe de novo", "")
	s.caio.createNote(s.campaign, "a primeira do Caio", "")
}

// MR-030, question 61: a note may be tagged only with a scene the group
// discovered: the point was revealed on the map (and stays discovered when
// hidden again), or the master opened the scene in a session, even hidden. An
// undiscovered scene is refused as one that does not exist, and its name is
// nowhere in what the player gets.
func TestMR030_TagsOnlyDiscoveredScenes(t *testing.T) {
	t.Parallel()
	s := newScenes(t, true)
	pid := s.point.GetId()
	if got := s.ana.noteScenes(s.campaign); len(got) != 0 {
		t.Fatalf("the tag picker = %v before any discovery, want none", got)
	}

	// Undiscovered and nonexistent are refused the same way.
	_, errHidden := s.ana.tryCreateNote(s.campaign, "nota", pid)
	_, errNone := s.ana.tryCreateNote(s.campaign, "nota", newKey())
	_, errJunk := s.ana.tryCreateNote(s.campaign, "nota", "nope")
	for name, err := range map[string]error{"hidden": errHidden, "nonexistent": errNone, "not a UUID": errJunk} {
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("tagging a %s scene error = %v, want invalid_argument", name, err)
		}
	}
	if errHidden.Error() != errNone.Error() {
		t.Errorf("an undiscovered scene is refused with %q, a nonexistent one with %q; want the same", errHidden, errNone)
	}
	// A point that is not a scene is not a scene to tag, even when revealed.
	battle := s.master.createPoint(&mapsv1.CreateMapPointRequest{CampaignId: s.campaign, MapId: s.mapID, Kind: mapsv1.MapPointKind_MAP_POINT_KIND_BATTLE, Name: "Emboscada"})
	s.master.setPointRevealed(s.campaign, battle, true)
	_, err := s.ana.tryCreateNote(s.campaign, "nota", battle.GetId())
	wantCode(t, "tagging a battle point", err, connect.CodeInvalidArgument)

	// Revealing the point discovers the scene, for the whole group, and it
	// stays when the point is hidden again.
	s.master.setPointRevealed(s.campaign, s.point, true)
	s.master.setPointRevealed(s.campaign, s.point, false)
	for name, u := range map[string]*user{"Ana": s.ana, "Caio": s.caio} {
		if got := u.noteScenes(s.campaign); len(got) != 1 || got[0].GetId() != pid || got[0].GetName() != "A carroça tombada" {
			t.Errorf("%s's tag picker = %v, want the revealed scene", name, got)
		}
	}
	tagged := s.ana.createNote(s.campaign, "A carroça", pid)
	if tagged.GetScenePointId() != pid || tagged.GetSceneName() != "A carroça tombada" {
		t.Errorf("the tagged note = %v", tagged)
	}

	// Opening a hidden scene discovers it too (the players see its name in
	// the open scene anyway); so does UpdateMapPoint's revealed flag.
	second := s.master.createPoint(&mapsv1.CreateMapPointRequest{CampaignId: s.campaign, MapId: s.mapID, Kind: mapsv1.MapPointKind_MAP_POINT_KIND_SCENE, Name: "A ponte"})
	third := s.master.createPoint(&mapsv1.CreateMapPointRequest{CampaignId: s.campaign, MapId: s.mapID, Kind: mapsv1.MapPointKind_MAP_POINT_KIND_SCENE, Name: "O vau"})
	undiscovered := s.master.createPoint(&mapsv1.CreateMapPointRequest{CampaignId: s.campaign, MapId: s.mapID, Kind: mapsv1.MapPointKind_MAP_POINT_KIND_SCENE, Name: "A caverna do Vale Seco"})
	s.master.mustOpenScene(s.campaign, second.GetId())
	revealed := true
	if _, err := s.master.updatePoint(&mapsv1.UpdateMapPointRequest{CampaignId: s.campaign, MapId: s.mapID, PointId: third.GetId(), Revealed: &revealed}); err != nil {
		t.Fatal(err)
	}
	picker := s.ana.noteScenes(s.campaign)
	var pickerNames []string
	for _, sc := range picker {
		pickerNames = append(pickerNames, sc.GetName())
	}
	if got, want := strings.Join(pickerNames, "|"), "A carroça tombada|A ponte|O vau"; got != want {
		t.Errorf("the tag picker = %q, want %q, oldest discovery first, without the undiscovered scene", got, want)
	}
	if strings.Contains(asJSON(t, &notesv1.ListNoteScenesResponse{Scenes: picker}), "Vale Seco") {
		t.Error("the undiscovered scene's name is in the tag picker")
	}
	if _, err := s.ana.tryCreateNote(s.campaign, "nota", undiscovered.GetId()); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("tagging the undiscovered scene error = %v", err)
	}

	// Renaming shows in the notes; the filter finds a scene's notes; the tag
	// can be changed and removed.
	newName := "A carroça virada"
	if _, err := s.master.updatePoint(&mapsv1.UpdateMapPointRequest{CampaignId: s.campaign, MapId: s.mapID, PointId: pid, Name: &newName}); err != nil {
		t.Fatal(err)
	}
	s.ana.createNote(s.campaign, "A ponte", second.GetId())
	s.ana.createNote(s.campaign, "Sem cena", "")
	if got := noteTexts(s.ana.listNotes(s.campaign, pid)); len(got) != 1 || got[0] != "A carroça" {
		t.Errorf("the notes of the first scene = %v", got)
	}
	if got := s.ana.listNotes(s.campaign, pid).GetNotes()[0].GetSceneName(); got != newName {
		t.Errorf("the tag's name = %q, want the current %q", got, newName)
	}
	if got := s.ana.listNotes(s.campaign, undiscovered.GetId()); len(got.GetNotes()) != 0 {
		t.Errorf("filtering by the undiscovered scene = %v, want nothing", got.GetNotes())
	}
	if got := s.ana.listNotes(s.campaign, ""); len(got.GetNotes()) != 3 || got.GetNotes()[0].GetText() != "Sem cena" {
		t.Errorf("the whole list = %v, want 3, newest first", noteTexts(got))
	}
	none := ""
	cleared, err := s.ana.notes.UpdateNote(t.Context(), connect.NewRequest(&notesv1.UpdateNoteRequest{CampaignId: s.campaign, NoteId: tagged.GetId(), ScenePointId: &none}))
	if err != nil || cleared.Msg.GetNote().GetScenePointId() != "" || cleared.Msg.GetNote().GetText() != "A carroça" {
		t.Errorf("removing the tag = %v, %v", cleared, err)
	}
	badTag := undiscovered.GetId()
	_, err = s.ana.notes.UpdateNote(t.Context(), connect.NewRequest(&notesv1.UpdateNoteRequest{CampaignId: s.campaign, NoteId: tagged.GetId(), ScenePointId: &badTag}))
	wantCode(t, "re-tagging with an undiscovered scene", err, connect.CodeInvalidArgument)
}

// RN-20, extended (MR-029, MR-030): a player never receives the master's
// hooks, a clue that was not revealed to them, another player's note, or the
// name of a scene the group has not discovered, in any response or stream
// event. It reads everything Ana and Caio receive, as the JSON the app gets.
// The master never gets a note either.
func TestRN20_PlayersNeverGetHooksOrUnrevealedClues(t *testing.T) {
	t.Parallel()
	s := newScenes(t, true)
	const (
		hooksA       = "SEGREDO-GANCHO-A"
		hooksB       = "SEGREDO-GANCHO-B"
		clueForAna   = "PISTA-SO-DA-ANA"
		clueForNoone = "SEGREDO-PISTA-NAO-REVELADA"
		clueLater    = "SEGREDO-PISTA-NOVA"
		clueB        = "SEGREDO-PISTA-DA-CENA-B"
		sceneBName   = "Caverna do Vale Seco"
		anaNote      = "NOTA-PRIVADA-DA-ANA"
		caioNote     = "NOTA-PRIVADA-DO-CAIO"
	)
	pidA := s.point.GetId()
	pointB := s.master.createPoint(&mapsv1.CreateMapPointRequest{
		CampaignId: s.campaign, MapId: s.mapID, Kind: mapsv1.MapPointKind_MAP_POINT_KIND_SCENE, Name: sceneBName,
		Description: "DESCRICAO-DA-CENA-B", Hooks: hooksB, XBp: 8000, YBp: 8000,
	})
	if _, err := s.setHooks(s.point, hooksA); err != nil {
		t.Fatal(err)
	}
	s.master.addAction(s.campaign, s.mapID, pidA, "skill:investigation", "Procurar", 15)
	cAna := s.master.addClue(s.campaign, s.mapID, pidA, clueForAna)
	s.master.addClue(s.campaign, s.mapID, pidA, clueForNoone)
	s.master.addClue(s.campaign, s.mapID, pointB.GetId(), clueB)
	s.master.setPointRevealed(s.campaign, s.point, true) // the map shows A, never B
	probeMap := s.master.createMap(s.campaign, "Sonda", s.master.newImage(s.campaign)).GetId()

	anaWatch, caioWatch, masterWatch := s.ana.watch(s.campaign), s.caio.watch(s.campaign), s.master.watch(s.campaign)
	s.master.mustOpenScene(s.campaign, pidA)
	s.master.mustRevealClue(s.campaign, cAna.GetId(), s.pens.GetId())
	s.ana.createNote(s.campaign, anaNote, pidA)
	s.caio.createNote(s.campaign, caioNote, "")
	// The master keeps working: everything below must stay invisible to them.
	s.master.addClue(s.campaign, s.mapID, pidA, clueLater)
	if _, err := s.setHooks(s.point, hooksA+" v2"); err != nil {
		t.Fatal(err)
	}
	renamed := sceneBName + " II"
	if _, err := s.master.updatePoint(&mapsv1.UpdateMapPointRequest{CampaignId: s.campaign, MapId: s.mapID, PointId: pointB.GetId(), Name: &renamed}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.setHooks(pointB, hooksB+" v2"); err != nil {
		t.Fatal(err)
	}
	s.probe(probeMap)

	// What each player received: every response and every stream event.
	receipts := func(u *user, w *watcher) []string {
		t.Helper()
		var seen []string
		maps, err := u.maps.ListMaps(t.Context(), connect.NewRequest(&mapsv1.ListMapsRequest{CampaignId: s.campaign}))
		if err != nil {
			t.Fatal(err)
		}
		seen = append(seen, asJSON(t, maps.Msg))
		seen = append(seen, asJSON(t, u.mustGetMap(s.campaign, s.mapID)))
		scene, err := u.play.GetOpenScene(t.Context(), connect.NewRequest(&playv1.GetOpenSceneRequest{CampaignId: s.campaign}))
		if err != nil {
			t.Fatal(err)
		}
		seen = append(seen, asJSON(t, scene.Msg))
		seen = append(seen, asJSON(t, u.listNotes(s.campaign, "")), asJSON(t, u.listNotes(s.campaign, pidA)))
		seen = append(seen, asJSON(t, &notesv1.ListNoteScenesResponse{Scenes: u.noteScenes(s.campaign)}))
		live, err := u.play.GetLiveSession(t.Context(), connect.NewRequest(&playv1.GetLiveSessionRequest{CampaignId: s.campaign}))
		if err != nil {
			t.Fatal(err)
		}
		seen = append(seen, asJSON(t, live.Msg))
		for _, ev := range w.drain(probeMap) {
			seen = append(seen, asJSON(t, ev))
		}
		return seen
	}
	secrets := []string{"SEGREDO", "Vale Seco", "DESCRICAO-DA-CENA-B"}
	check := func(who string, seen []string, forbidden ...string) {
		t.Helper()
		all := strings.Join(seen, "\n")
		for _, f := range append(forbidden, secrets...) {
			if strings.Contains(all, f) {
				t.Errorf("%s received %q:\n%s", who, f, all)
			}
		}
	}
	anaSaw, caioSaw := receipts(s.ana, anaWatch), receipts(s.caio, caioWatch)
	check("Ana", anaSaw, caioNote)
	check("Caio", caioSaw, anaNote, clueForAna)
	// Positive controls: Ana got her clue and her note, and her own stream hint.
	allAna := strings.Join(anaSaw, "\n")
	for _, want := range []string{clueForAna, anaNote, "notesChanged", "A carroça tombada"} {
		if !strings.Contains(allAna, want) {
			t.Errorf("Ana's receipts lack %q, the test would prove nothing", want)
		}
	}
	if strings.Contains(strings.Join(caioSaw, "\n"), "notesChanged") {
		t.Error("Caio's stream got notes_changed for a clue he did not receive")
	}

	// The master: no note of anyone, in any read or event.
	masterSaw := []string{asJSON(t, s.master.mustGetMap(s.campaign, s.mapID)), asJSON(t, s.master.getScene(s.campaign))}
	for _, ev := range masterWatch.drain(probeMap) {
		masterSaw = append(masterSaw, asJSON(t, ev))
	}
	allMaster := strings.Join(masterSaw, "\n")
	for _, f := range []string{anaNote, caioNote, "notesChanged"} {
		if strings.Contains(allMaster, f) {
			t.Errorf("the master received %q", f)
		}
	}
	for _, want := range []string{hooksA + " v2", clueLater, hooksB + " v2", clueB} {
		if !strings.Contains(allMaster, want) {
			t.Errorf("the master's receipts lack %q", want)
		}
	}
	if _, err := s.master.tryListNotes(s.campaign, ""); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("the master's ListNotes error = %v, want not_found", err)
	}
}

// Question 63, changed: any SCENE point opens, even with no actions; the old
// refusal is gone and the player's scene has no actions.
func TestScenesWithNoActionsOpen(t *testing.T) {
	t.Parallel()
	s := newScenes(t, true)
	scene, err := s.master.openScene(s.campaign, s.point.GetId())
	if err != nil {
		t.Fatalf("OpenScene() of a point with no actions error = %v, want it open", err)
	}
	if len(scene.GetActions()) != 0 || scene.GetName() != "A carroça tombada" {
		t.Errorf("the opened scene = %v", scene)
	}
	if got := s.ana.getScene(s.campaign); got.GetPointId() != s.point.GetId() || len(got.GetActions()) != 0 || got.GetDescription() != sceneDescription {
		t.Errorf("Ana's scene = %v, want the open scene with its description and no actions", got)
	}
}

// The authorization matrix of the clue RPCs: only the master calls them; a
// player gets permission_denied; an outsider and a pending member not_found; a
// signed-out caller, unauthenticated.
func TestClueAuthorizationMatrix(t *testing.T) {
	t.Parallel()
	s := newScenes(t, true)
	pending := s.h.newUser("Pendente")
	s.h.join(s.master, s.campaign, true, pending)
	pid := s.point.GetId()
	// Each call adds a clue of its own to work on.
	fresh := func() string { return s.master.addClue(s.campaign, s.mapID, pid, "pista").GetId() }

	type call = func(ctx context.Context, u *user) error
	rows := []struct {
		name string
		call call
	}{
		{"AddSceneClue", func(_ context.Context, u *user) error {
			_, err := u.tryAddClue(s.campaign, s.mapID, pid, "nova")
			return err
		}},
		{"UpdateSceneClue", func(ctx context.Context, u *user) error {
			_, err := u.maps.UpdateSceneClue(ctx, connect.NewRequest(&mapsv1.UpdateSceneClueRequest{CampaignId: s.campaign, MapId: s.mapID, PointId: pid, ClueId: fresh(), Text: "x"}))
			return err
		}},
		{"MoveSceneClue", func(ctx context.Context, u *user) error {
			_, err := u.maps.MoveSceneClue(ctx, connect.NewRequest(&mapsv1.MoveSceneClueRequest{
				CampaignId: s.campaign, MapId: s.mapID, PointId: pid, ClueId: fresh(), Direction: mapsv1.SceneActionDirection_SCENE_ACTION_DIRECTION_UP,
			}))
			return err
		}},
		{"RemoveSceneClue", func(ctx context.Context, u *user) error {
			_, err := u.maps.RemoveSceneClue(ctx, connect.NewRequest(&mapsv1.RemoveSceneClueRequest{CampaignId: s.campaign, MapId: s.mapID, PointId: pid, ClueId: fresh()}))
			return err
		}},
		{"RevealSceneClue", func(_ context.Context, u *user) error {
			_, err := u.revealClue(s.campaign, fresh(), s.pens.GetId())
			return err
		}},
	}
	covered := map[string]bool{}
	for _, r := range rows {
		covered[r.name] = true
	}
	methods := mapsv1.File_meurpg_maps_v1_maps_proto.Services().ByName("MapService").Methods()
	for i := range methods.Len() {
		name := string(methods.Get(i).Name())
		if strings.Contains(name, "Clue") && !covered[name] {
			t.Errorf("MapService.%s is missing from the clue authorization matrix", name)
		}
	}
	callers := []*user{s.master, s.ana, s.h.newUser("De fora"), s.h.anonymous(), pending}
	who := []string{"master", "player", "non-member", "anonymous", "pending"}
	want := [5]connect.Code{allowed, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound}
	for _, r := range rows {
		for i, u := range callers {
			wantCode(t, r.name+" as "+who[i], r.call(t.Context(), u), want[i])
		}
	}
}

// The authorization matrix of NotesService: an active player only. The master
// gets not_found (they have no notes, and learn nothing of the players'), an
// outsider and a pending member not_found too, a signed-out caller
// unauthenticated.
func TestNotesAuthorizationMatrix(t *testing.T) {
	t.Parallel()
	s := newScenes(t, false)
	pending := s.h.newUser("Pendente")
	s.h.join(s.master, s.campaign, true, pending)
	c := s.campaign
	mine := s.ana.createNote(c, "minha", "")

	type call = func(ctx context.Context, u *user) error
	rows := []struct {
		name string
		call call
	}{
		{"ListNotes", func(_ context.Context, u *user) error { _, err := u.tryListNotes(c, ""); return err }},
		{"CreateNote", func(_ context.Context, u *user) error { _, err := u.tryCreateNote(c, "nova", ""); return err }},
		{"UpdateNote", func(ctx context.Context, u *user) error {
			text := "outra"
			_, err := u.notes.UpdateNote(ctx, connect.NewRequest(&notesv1.UpdateNoteRequest{CampaignId: c, NoteId: mine.GetId(), Text: &text}))
			return err
		}},
		{"DeleteNote", func(ctx context.Context, u *user) error {
			// Each caller deletes a note of their own, if they may have one.
			id := mine.GetId()
			if u.id == s.ana.id {
				id = s.ana.createNote(c, "para apagar", "").GetId()
			}
			_, err := u.notes.DeleteNote(ctx, connect.NewRequest(&notesv1.DeleteNoteRequest{CampaignId: c, NoteId: id}))
			return err
		}},
		{"ListNoteScenes", func(ctx context.Context, u *user) error {
			_, err := u.notes.ListNoteScenes(ctx, connect.NewRequest(&notesv1.ListNoteScenesRequest{CampaignId: c}))
			return err
		}},
	}
	covered := map[string]bool{}
	for _, r := range rows {
		covered[r.name] = true
	}
	methods := notesv1.File_meurpg_notes_v1_notes_proto.Services().ByName("NotesService").Methods()
	for i := range methods.Len() {
		if name := string(methods.Get(i).Name()); !covered[name] {
			t.Errorf("NotesService.%s is missing from the authorization matrix", name)
		}
	}
	// Caio is a player too: his own call works, on his own notes (UpdateNote
	// of Ana's note is not_found for him, which TestMR030 proves).
	callers := []*user{s.master, s.ana, s.h.newUser("De fora"), s.h.anonymous(), pending}
	who := []string{"master", "player", "non-member", "anonymous", "pending"}
	want := [5]connect.Code{connect.CodeNotFound, allowed, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound}
	for _, r := range rows {
		for i, u := range callers {
			wantCode(t, r.name+" as "+who[i], r.call(t.Context(), u), want[i])
		}
	}
}

// NotesService's answers are never cached, and its two reads are idempotent
// (POST-only, with the campaign in the body).
func TestNotesServiceHeadersAndIdempotency(t *testing.T) {
	t.Parallel()
	s := newScenes(t, false)
	_, err := s.h.anonymous().tryListNotes(s.campaign, "")
	ce, ok := errors.AsType[*connect.Error](err)
	if !ok || connect.CodeOf(err) != connect.CodeUnauthenticated || !slices.Equal(ce.Meta().Values("Cache-Control"), []string{"no-store"}) {
		t.Errorf("a signed-out ListNotes error = %v, want unauthenticated with Cache-Control no-store", err)
	}
	methods := notesv1.File_meurpg_notes_v1_notes_proto.Services().ByName("NotesService").Methods()
	for _, name := range []string{"ListNotes", "ListNoteScenes"} {
		opts, _ := methods.ByName(protoreflect.Name(name)).Options().(*descriptorpb.MethodOptions)
		if opts.GetIdempotencyLevel() != descriptorpb.MethodOptions_IDEMPOTENT {
			t.Errorf("%s idempotency_level = %v, want IDEMPOTENT", name, opts.GetIdempotencyLevel())
		}
	}
}

// Deleting the account deletes the notes, and what the player received.
func TestMR030_DeletingTheAccountDeletesTheNotes(t *testing.T) {
	t.Parallel()
	s := newScenes(t, false)
	s.ana.createNote(s.campaign, "para sumir", "")
	clue := s.master.addClue(s.campaign, s.mapID, s.point.GetId(), "Uma pista")
	s.master.mustRevealClue(s.campaign, clue.GetId(), s.pens.GetId())
	if _, err := s.h.pool.Exec(t.Context(), `DELETE FROM users WHERE id = $1`, s.ana.id); err != nil {
		t.Fatalf("delete the account: %v", err)
	}
	var notes, reveals int
	if err := s.h.pool.QueryRow(t.Context(), `SELECT count(*) FROM player_notes`).Scan(&notes); err != nil {
		t.Fatal(err)
	}
	if err := s.h.pool.QueryRow(t.Context(), `SELECT count(*) FROM scene_clue_reveals`).Scan(&reveals); err != nil {
		t.Fatal(err)
	}
	if notes != 0 || reveals != 0 {
		t.Errorf("after deleting the account: %d notes and %d reveals left, want none", notes, reveals)
	}
}
