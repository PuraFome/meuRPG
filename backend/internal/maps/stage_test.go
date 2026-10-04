package maps

import (
	"encoding/json"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The stage, "NPCs em cena" (MR-031, D7, RN-20; question 62 with its default),
// and the portraits that go with it. The tests live here because this harness
// has the maps (the image route), the characters and play together.

// What an NPC holds that no player may ever read.
const npcNote = "SEGREDO-DO-MESTRE-SOBRE-O-NPC"

// numbers finds the NPC's hit points and XP (createNPC) used as a JSON value.
var numbers = regexp.MustCompile(`:\s+(7777|43210)\b`)

// createNPC creates a minion with numbers and a description that stay the
// master's (RN-20), with the gallery image as its portrait when there is one.
func (u *user) createNPC(campaignID, name, portraitImageID string) *charactersv1.Character {
	u.h.t.Helper()
	sheet := &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Basic{Basic: &charactersv1.BasicSheet{
		HitPointsMax: 7777, ArmorClass: 29, SpeedFt: 30, Description: npcNote, XpValue: 43210, ChallengeRating: "2",
		PortraitImageId: portraitImageID,
	}}}
	res, err := u.characters.CreateCharacter(u.h.t.Context(), connect.NewRequest(&charactersv1.CreateCharacterRequest{
		CampaignId: campaignID, Kind: charactersv1.CharacterKind_CHARACTER_KIND_MINION, Name: name, Sheet: sheet,
	}))
	if err != nil {
		u.h.t.Fatalf("CreateCharacter(%s) error = %v", name, err)
	}
	return res.Msg.GetCharacter()
}

func (u *user) putOnStage(campaignID, characterID string) ([]*playv1.StageNpc, error) {
	res, err := u.play.PutOnStage(u.h.t.Context(), connect.NewRequest(&playv1.PutOnStageRequest{CampaignId: campaignID, CharacterId: characterID}))
	if err != nil {
		return nil, err
	}
	return res.Msg.GetStage(), nil
}

func (u *user) mustPutOnStage(campaignID, characterID string) []*playv1.StageNpc {
	u.h.t.Helper()
	stage, err := u.putOnStage(campaignID, characterID)
	if err != nil {
		u.h.t.Fatalf("PutOnStage() error = %v", err)
	}
	return stage
}

func (u *user) takeOffStage(campaignID, characterID string) []*playv1.StageNpc {
	u.h.t.Helper()
	res, err := u.play.TakeOffStage(u.h.t.Context(), connect.NewRequest(&playv1.TakeOffStageRequest{CampaignId: campaignID, CharacterId: characterID}))
	if err != nil {
		u.h.t.Fatalf("TakeOffStage() error = %v", err)
	}
	return res.Msg.GetStage()
}

func (u *user) setSpeaker(campaignID, characterID string) ([]*playv1.StageNpc, error) {
	res, err := u.play.SetSpeaker(u.h.t.Context(), connect.NewRequest(&playv1.SetSpeakerRequest{CampaignId: campaignID, CharacterId: characterID}))
	if err != nil {
		return nil, err
	}
	return res.Msg.GetStage(), nil
}

func (u *user) mustSetSpeaker(campaignID, characterID string) []*playv1.StageNpc {
	u.h.t.Helper()
	stage, err := u.setSpeaker(campaignID, characterID)
	if err != nil {
		u.h.t.Fatalf("SetSpeaker() error = %v", err)
	}
	return stage
}

// stageNames lists the names on a stage, in order, with a "*" after the one speaking.
func stageNames(stage []*playv1.StageNpc) []string {
	out := []string{}
	for _, n := range stage {
		name := n.GetName()
		if n.GetSpeaking() {
			name += "*"
		}
		out = append(out, name)
	}
	return out
}

func wantStage(t *testing.T, when string, got []*playv1.StageNpc, want ...string) {
	t.Helper()
	if want == nil {
		want = []string{}
	}
	if g := stageNames(got); !slices.Equal(g, want) {
		t.Errorf("%s: the stage is %v, want %v", when, g, want)
	}
}

// stageEvents reads the stage_changed events, oldest first, as payload JSON.
func (h *harness) stageEvents() []string {
	h.t.Helper()
	rows, err := h.pool.Query(h.t.Context(), `SELECT payload::TEXT FROM session_events WHERE kind = 'stage_changed' ORDER BY seq`)
	if err != nil {
		h.t.Fatalf("read the stage events: %v", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			h.t.Fatal(err)
		}
		out = append(out, payload)
	}
	return out
}

// stageChanged reads the next event, which must be stage_changed.
func (w *watcher) stageChanged() {
	w.t.Helper()
	if ev := w.next(); ev.GetStageChanged() == nil {
		w.t.Fatalf("event = %v, want stage_changed", ev)
	}
}

// secondScene adds another scene point, with an action, so a test can switch.
func (s *scenes) secondScene() *mapsv1.MapPoint {
	s.h.t.Helper()
	p := s.master.createPoint(&mapsv1.CreateMapPointRequest{
		CampaignId: s.campaign, MapId: s.mapID, Kind: mapsv1.MapPointKind_MAP_POINT_KIND_SCENE,
		Name: "O vau do riacho", Description: "A água corre rasa.", XBp: 6000, YBp: 2000,
	})
	s.master.addAction(s.campaign, s.mapID, p.GetId(), "skill:perception", "", 0)
	return p
}

// MR-031: the master puts NPCs on the stage and takes them out, in order and
// at most four; the speaker is one at a time; closing the scene and switching
// to another empty the stage; each change reaches everyone's stream and the
// history, which keeps ids only.
func TestMR031_TheMasterPutsNPCsOnStage(t *testing.T) {
	t.Parallel()
	s := newScenes(t, true)
	s.setup()
	npc := map[string]*charactersv1.Character{}
	for _, n := range []string{"Aldo", "Barão Ivo", "Capitã", "Duda", "Eira"} {
		npc[n] = s.master.createNPC(s.campaign, n, "")
	}
	put := func(n string) []*playv1.StageNpc { return s.master.mustPutOnStage(s.campaign, npc[n].GetId()) }
	masterWatch, anaWatch := s.master.watch(s.campaign), s.ana.watch(s.campaign)

	// Without a scene there is no stage.
	_, err := s.master.putOnStage(s.campaign, npc["Aldo"].GetId())
	wantSceneBlocked(t, "PutOnStage(no scene)", err, playv1.SceneBlockedReason_SCENE_BLOCKED_REASON_NO_OPEN_SCENE)

	s.master.mustOpenScene(s.campaign, s.point.GetId())
	masterWatch.sceneChanged()
	anaWatch.sceneChanged()
	wantStage(t, "a scene just opened", s.master.getScene(s.campaign).GetStage())

	// In order, four at most.
	wantStage(t, "Aldo", put("Aldo"), "Aldo")
	wantStage(t, "Barão Ivo", put("Barão Ivo"), "Aldo", "Barão Ivo")
	wantStage(t, "Capitã", put("Capitã"), "Aldo", "Barão Ivo", "Capitã")
	wantStage(t, "Duda", put("Duda"), "Aldo", "Barão Ivo", "Capitã", "Duda")
	for range 4 {
		masterWatch.stageChanged()
		anaWatch.stageChanged() // everyone hears of it
	}
	_, err = s.master.putOnStage(s.campaign, npc["Eira"].GetId())
	wantSceneBlocked(t, "PutOnStage(a fifth)", err, playv1.SceneBlockedReason_SCENE_BLOCKED_REASON_STAGE_FULL)
	wantStage(t, "an NPC already there", put("Aldo"), "Aldo", "Barão Ivo", "Capitã", "Duda") // changes nothing: no event either
	if n := len(s.h.stageEvents()); n != 4 {
		t.Errorf("%d stage events, want 4: the refused and the repeated puts wrote none", n)
	}

	// Only a living NPC of the campaign goes on the stage.
	_, err = s.master.putOnStage(s.campaign, s.pens.GetId())
	wantCode(t, "PutOnStage(a player's character)", err, connect.CodeNotFound)
	_, err = s.master.putOnStage(s.campaign, "00000000-0000-4000-8000-000000000000")
	wantCode(t, "PutOnStage(an unknown ID)", err, connect.CodeNotFound)
	_, err = s.master.putOnStage(s.campaign, "não é um ID")
	wantCode(t, "PutOnStage(not an ID)", err, connect.CodeNotFound)

	// One speaks at a time; the same again changes nothing; nobody is allowed.
	wantStage(t, "Barão speaks", s.master.mustSetSpeaker(s.campaign, npc["Barão Ivo"].GetId()), "Aldo", "Barão Ivo*", "Capitã", "Duda")
	wantStage(t, "Capitã speaks", s.master.mustSetSpeaker(s.campaign, npc["Capitã"].GetId()), "Aldo", "Barão Ivo", "Capitã*", "Duda")
	s.master.mustSetSpeaker(s.campaign, npc["Capitã"].GetId())
	wantStage(t, "nobody speaks", s.master.mustSetSpeaker(s.campaign, ""), "Aldo", "Barão Ivo", "Capitã", "Duda")
	s.master.mustSetSpeaker(s.campaign, "")
	_, err = s.master.setSpeaker(s.campaign, npc["Eira"].GetId())
	wantCode(t, "SetSpeaker(an NPC that is not on the stage)", err, connect.CodeNotFound)
	s.master.mustSetSpeaker(s.campaign, npc["Barão Ivo"].GetId())
	for range 3 { // Barão, Capitã, nobody, Barão: the repeats wrote nothing
		masterWatch.stageChanged()
		anaWatch.stageChanged()
	}
	masterWatch.stageChanged()
	anaWatch.stageChanged()

	// Taking one off keeps the others' order, and makes room.
	wantStage(t, "Capitã off", s.master.takeOffStage(s.campaign, npc["Capitã"].GetId()), "Aldo", "Barão Ivo*", "Duda")
	wantStage(t, "taking off an NPC that is not there", s.master.takeOffStage(s.campaign, npc["Eira"].GetId()), "Aldo", "Barão Ivo*", "Duda")
	wantStage(t, "Eira in", put("Eira"), "Aldo", "Barão Ivo*", "Duda", "Eira")
	wantStage(t, "the speaker leaves", s.master.takeOffStage(s.campaign, npc["Barão Ivo"].GetId()), "Aldo", "Duda", "Eira")
	masterWatch.stageChanged() // Capitã off
	masterWatch.stageChanged() // Eira in
	masterWatch.stageChanged() // Barão off
	for range 3 {
		anaWatch.stageChanged()
	}

	// Closing the scene empties the stage, and the next one starts empty.
	s.master.closeScene(s.campaign)
	if masterWatch.next().GetSceneChanged() == nil {
		t.Fatal("closing the scene sent no scene_changed")
	}
	masterWatch.stageChanged()
	wantStage(t, "the scene closed", s.master.getScene(s.campaign).GetStage())
	s.master.mustOpenScene(s.campaign, s.point.GetId())
	wantStage(t, "the scene opened again", s.master.getScene(s.campaign).GetStage())

	// Switching to another scene empties it too.
	put("Aldo")
	put("Duda")
	second := s.secondScene()
	info := s.master.mustOpenScene(s.campaign, second.GetId())
	wantStage(t, "another scene", info.GetStage())
	wantStage(t, "another scene, read again", s.master.getScene(s.campaign).GetStage())
	// Opening the scene that is already open keeps what is on it.
	put("Eira")
	s.master.mustOpenScene(s.campaign, second.GetId())
	wantStage(t, "the same scene opened again", s.master.getScene(s.campaign).GetStage(), "Eira")

	// The history keeps ids only: no name, no portrait.
	for _, e := range s.h.stageEvents() {
		for n := range npc {
			if strings.Contains(e, n) {
				t.Errorf("the history %q holds the name %q", e, n)
			}
		}
	}
	if events := s.h.stageEvents(); !strings.Contains(events[0], `"put"`) || !strings.Contains(events[0], npc["Aldo"].GetId()) {
		t.Errorf("the first stage event = %s, want Aldo put on stage, by id", events[0])
	}
}

// MR-031 and RN-20: a player gets an NPC's name and portrait on the stage,
// and whether it speaks, and nothing else about it: not its kind, hit points,
// armor class, XP, description or sheet, not even its character ID. Every
// answer and stream event the player gets is read as JSON.
func TestMR031_APlayerSeesOnlyNameAndPortrait(t *testing.T) {
	t.Parallel()
	s := newScenes(t, true)
	s.setup()
	portrait := s.master.newImage(s.campaign)
	aldo := s.master.createNPC(s.campaign, "Aldo", portrait)
	duda := s.master.createNPC(s.campaign, "Duda", "") // no portrait: the app draws the initials
	anaWatch := s.ana.watch(s.campaign)

	var seen []string // everything Ana was sent, as JSON
	see := func(m proto.Message) { seen = append(seen, protojson.Format(m)) }
	drain := func(n int) {
		t.Helper()
		for range n {
			see(anaWatch.next())
		}
	}

	s.master.mustOpenScene(s.campaign, s.point.GetId())
	s.master.mustPutOnStage(s.campaign, aldo.GetId())
	s.master.mustPutOnStage(s.campaign, duda.GetId())
	s.master.mustSetSpeaker(s.campaign, aldo.GetId())
	drain(4) // scene_changed, stage_changed x3
	see(s.ana.getScene(s.campaign))

	// What the master gets has the character IDs; Ana's has none.
	for _, n := range s.master.getScene(s.campaign).GetStage() {
		if n.GetCharacterId() == "" {
			t.Errorf("the master's stage entry %v has no character_id", n)
		}
	}
	stage := s.ana.getScene(s.campaign).GetStage()
	if len(stage) != 2 {
		t.Fatalf("Ana's stage = %v, want Aldo and Duda", stage)
	}
	if got := stage[0]; got.GetName() != "Aldo" || got.GetPortraitUrl() != "/images/"+portrait || !got.GetSpeaking() || got.GetId() == "" || got.GetCharacterId() != "" {
		t.Errorf("Ana's first entry = %v, want Aldo, his portrait, speaking, no character_id", got)
	}
	if got := stage[1]; got.GetName() != "Duda" || got.GetPortraitUrl() != "" || got.GetSpeaking() {
		t.Errorf("Ana's second entry = %v, want Duda with no portrait, not speaking", got)
	}
	// An entry is a place on the stage, not the character.
	for _, n := range stage {
		if n.GetId() == aldo.GetId() || n.GetId() == duda.GetId() {
			t.Errorf("the entry's id %s is a character's", n.GetId())
		}
	}
	// Each entry carries only these fields: a new one would have to be thought
	// through for players.
	for _, n := range stage {
		var fields map[string]any
		if err := json.Unmarshal([]byte(protojson.Format(n)), &fields); err != nil {
			t.Fatal(err)
		}
		for key := range fields {
			if !slices.Contains([]string{"id", "name", "portraitUrl", "speaking"}, key) {
				t.Errorf("Ana's stage entry has the field %q", key)
			}
		}
	}

	s.master.takeOffStage(s.campaign, duda.GetId())
	s.master.mustSetSpeaker(s.campaign, "")
	s.master.closeScene(s.campaign)
	drain(4) // stage_changed x2, scene_changed, stage_changed
	see(s.ana.getScene(s.campaign))
	live, err := s.ana.play.GetLiveSession(t.Context(), connect.NewRequest(&playv1.GetLiveSessionRequest{CampaignId: s.campaign}))
	if err != nil {
		t.Fatalf("GetLiveSession() error = %v", err)
	}
	see(live.Msg)
	see(s.ana.mustGetMap(s.campaign, s.mapID))

	for _, text := range seen {
		for _, banned := range []string{aldo.GetId(), duda.GetId(), npcNote, "armorClass", "xpValue", "challengeRating", "CHARACTER_KIND", "MINION"} {
			if strings.Contains(text, banned) {
				t.Errorf("Ana was sent %q:\n%s", banned, text)
			}
		}
		// The NPC's numbers, as a JSON value (a hex digit string of a UUID
		// could hold the same digits).
		if numbers.MatchString(text) {
			t.Errorf("Ana was sent the NPC's hit points or XP:\n%s", text)
		}
	}
}

// MR-031 and RN-20, image access: a player fetches a portrait (and its
// thumbnail) only while its NPC is on the stage of the open scene. Any other
// time it is 404, exactly like an image they may not see; the master always
// gets it.
func TestMR031_PortraitsAreVisibleOnlyOnStage(t *testing.T) {
	t.Parallel()
	s := newScenes(t, true)
	s.setup()
	img := s.master.mustUpload(s.campaign, "aldo.png", pngImage(t, 40, 30))
	aldo := s.master.createNPC(s.campaign, "Aldo", img.GetId())
	stranger := s.h.newUser("De fora")

	status := func(who *user) (int, int) {
		t.Helper()
		return who.get(img.GetUrl()).status, who.get(img.GetThumbnailUrl()).status
	}
	check := func(when string, wantPlayer int) {
		t.Helper()
		if full, thumb := status(s.ana); full != wantPlayer || thumb != wantPlayer {
			t.Errorf("%s: Ana gets %d for the portrait and %d for its thumbnail, want %d", when, full, thumb, wantPlayer)
		}
		if full, thumb := status(s.master); full != http.StatusOK || thumb != http.StatusOK {
			t.Errorf("%s: the master gets %d and %d, want 200", when, full, thumb)
		}
		if full, thumb := status(stranger); full != http.StatusNotFound || thumb != http.StatusNotFound {
			t.Errorf("%s: someone outside the campaign gets %d and %d, want 404", when, full, thumb)
		}
	}

	check("the NPC has a portrait and nothing is open", http.StatusNotFound)
	s.master.mustOpenScene(s.campaign, s.point.GetId())
	check("a scene open and the NPC off stage", http.StatusNotFound)

	s.master.mustPutOnStage(s.campaign, aldo.GetId())
	check("on stage", http.StatusOK)
	etag := `"` + img.GetId() + `"`
	if res := s.ana.get(img.GetUrl(), "If-None-Match", etag); res.status != http.StatusNotModified {
		t.Errorf("revalidating a portrait on stage: status %d, want 304", res.status)
	}
	if got := s.ana.get(img.GetUrl()).header.Get("Cache-Control"); got != "private, no-cache" {
		t.Errorf("a player's portrait has Cache-Control %q, want private, no-cache: it may be gone any moment", got)
	}

	s.master.takeOffStage(s.campaign, aldo.GetId())
	check("taken off", http.StatusNotFound)
	if res := s.ana.get(img.GetUrl(), "If-None-Match", etag); res.status != http.StatusNotFound {
		t.Errorf("revalidating a portrait taken off: status %d, want 404, not a 304", res.status)
	}

	s.master.mustPutOnStage(s.campaign, aldo.GetId())
	check("back on stage", http.StatusOK)
	s.master.closeScene(s.campaign)
	check("the scene closed", http.StatusNotFound)

	// Another scene is another stage.
	second := s.secondScene()
	s.master.mustOpenScene(s.campaign, s.point.GetId())
	s.master.mustPutOnStage(s.campaign, aldo.GetId())
	check("on stage again", http.StatusOK)
	s.master.mustOpenScene(s.campaign, second.GetId())
	check("switched to another scene", http.StatusNotFound)

	// Ending the session ends it all.
	s.master.mustPutOnStage(s.campaign, aldo.GetId())
	check("on stage in the second scene", http.StatusOK)
	live, err := s.master.play.GetLiveSession(t.Context(), connect.NewRequest(&playv1.GetLiveSessionRequest{CampaignId: s.campaign}))
	if err != nil {
		t.Fatalf("GetLiveSession() error = %v", err)
	}
	if _, err := s.master.play.EndGameSession(t.Context(), connect.NewRequest(&playv1.EndGameSessionRequest{CampaignId: s.campaign, GameSessionId: live.Msg.GetGameSession().GetId()})); err != nil {
		t.Fatalf("EndGameSession() error = %v", err)
	}
	check("the session ended", http.StatusNotFound)
}

// MR-031: deleting a gallery image that is an NPC's portrait clears the portrait
// (a map's image refuses the delete; a portrait falls back to the initials), the
// NPC stays on the stage with its name, and everyone hears of it.
func TestMR031_DeletingAPortraitClearsIt(t *testing.T) {
	t.Parallel()
	s := newScenes(t, true)
	s.setup()
	img := s.master.mustUpload(s.campaign, "aldo.png", pngImage(t, 40, 30))
	aldo := s.master.createNPC(s.campaign, "Aldo", img.GetId())
	s.master.mustOpenScene(s.campaign, s.point.GetId())
	s.master.mustPutOnStage(s.campaign, aldo.GetId())
	anaWatch := s.ana.watch(s.campaign)

	if _, err := s.master.gallery.DeleteGalleryImage(t.Context(), connect.NewRequest(&mapsv1.DeleteGalleryImageRequest{CampaignId: s.campaign, ImageId: img.GetId()})); err != nil {
		t.Fatalf("DeleteGalleryImage(a portrait) error = %v, want it deleted", err)
	}
	anaWatch.stageChanged()
	stage := s.ana.getScene(s.campaign).GetStage()
	if len(stage) != 1 || stage[0].GetName() != "Aldo" || stage[0].GetPortraitUrl() != "" {
		t.Errorf("the stage after the delete = %v, want Aldo with no portrait", stage)
	}
	got, err := s.master.characters.GetCharacter(t.Context(), connect.NewRequest(&charactersv1.GetCharacterRequest{CampaignId: s.campaign, CharacterId: aldo.GetId()}))
	if err != nil {
		t.Fatalf("GetCharacter() error = %v", err)
	}
	if got.Msg.GetCharacter().GetSheet().GetBasic().GetPortraitImageId() != "" {
		t.Errorf("the sheet still has the portrait %q", got.Msg.GetCharacter().GetSheet().GetBasic().GetPortraitImageId())
	}
	if got.Msg.GetCharacter().GetRevision() <= aldo.GetRevision() {
		t.Errorf("revision = %d, want it above %d: an editor with the old sheet must be told", got.Msg.GetCharacter().GetRevision(), aldo.GetRevision())
	}
}

// The stage's calls are the master's; a player, an outsider, a pending
// member and nobody signed in are refused as everywhere else.
func TestStageAuthorizationMatrix(t *testing.T) {
	t.Parallel()
	s := newScenes(t, true)
	s.setup()
	pending := s.h.newUser("Pendente")
	s.h.join(s.master, s.campaign, true, pending)
	aldo := s.master.createNPC(s.campaign, "Aldo", "")
	s.master.mustOpenScene(s.campaign, s.point.GetId())

	type row struct {
		name string
		call func(u *user) error
		// master, player, non-member, anonymous, pending
		want [5]connect.Code
	}
	masterOnly := [5]connect.Code{allowed, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound}
	rows := []row{
		{"PutOnStage", func(u *user) error { _, err := u.putOnStage(s.campaign, aldo.GetId()); return err }, masterOnly},
		{"TakeOffStage", func(u *user) error {
			_, err := u.play.TakeOffStage(t.Context(), connect.NewRequest(&playv1.TakeOffStageRequest{CampaignId: s.campaign, CharacterId: aldo.GetId()}))
			return err
		}, masterOnly},
		{"SetSpeaker", func(u *user) error { _, err := u.setSpeaker(s.campaign, ""); return err }, masterOnly},
	}
	covered := map[string]bool{}
	for _, r := range rows {
		covered[r.name] = true
	}
	methods := playv1.File_meurpg_play_v1_play_proto.Services().ByName("PlayService").Methods()
	for i := range methods.Len() {
		name := string(methods.Get(i).Name())
		if strings.HasSuffix(name, "Stage") || name == "SetSpeaker" {
			if !covered[name] {
				t.Errorf("PlayService.%s is missing from the stage authorization matrix", name)
			}
		}
	}
	callers := []*user{s.master, s.ana, s.h.newUser("De fora"), s.h.anonymous(), pending}
	who := []string{"master", "player", "non-member", "anonymous", "pending"}
	for _, r := range rows {
		for i, u := range callers {
			wantCode(t, r.name+" as "+who[i], r.call(u), r.want[i])
		}
	}
	// A session that is not open has no stage to change.
	s2 := newScenes(t, false)
	npc := s2.master.createNPC(s2.campaign, "Aldo", "")
	_, err := s2.master.putOnStage(s2.campaign, npc.GetId())
	wantCode(t, "PutOnStage(no session)", err, connect.CodeFailedPrecondition)
}
