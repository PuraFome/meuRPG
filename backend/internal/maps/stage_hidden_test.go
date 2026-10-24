package maps

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The stage's hidden NPCs (MR-031, RN-10): the master puts an NPC on the stage hidden, or turns
// each one between "Os jogadores veem" and "Escondida dos jogadores"; a hidden NPC never reaches
// a player, and "Dar a fala" shows it.

// A made-up name that is neither a word nor hexadecimal, so finding it in an answer proves the NPC
// leaked (docs/testing.md, Needles).
const hiddenNPCName = "Zqvxorth-Kwjl-7731"

func (u *user) putHiddenOnStage(campaignID, characterID string, hidden bool) ([]*playv1.StageNpc, error) {
	res, err := u.play.PutOnStage(u.h.t.Context(), connect.NewRequest(&playv1.PutOnStageRequest{CampaignId: campaignID, CharacterId: characterID, Hidden: hidden}))
	if err != nil {
		return nil, err
	}
	return res.Msg.GetStage(), nil
}

func (u *user) setStageNpcHidden(campaignID, characterID string, hidden bool) ([]*playv1.StageNpc, error) {
	res, err := u.play.SetStageNpcHidden(u.h.t.Context(), connect.NewRequest(&playv1.SetStageNpcHiddenRequest{CampaignId: campaignID, CharacterId: characterID, Hidden: hidden}))
	if err != nil {
		return nil, err
	}
	return res.Msg.GetStage(), nil
}

func (u *user) mustSetStageNpcHidden(campaignID, characterID string, hidden bool) []*playv1.StageNpc {
	u.h.t.Helper()
	stage, err := u.setStageNpcHidden(campaignID, characterID, hidden)
	if err != nil {
		u.h.t.Fatalf("SetStageNpcHidden(hidden = %v) error = %v", hidden, err)
	}
	return stage
}

// stageMarks lists the names on a master's stage, with "#" after the hidden ones and "*" after the speaker.
func stageMarks(stage []*playv1.StageNpc) []string {
	out := []string{}
	for _, n := range stage {
		name := n.GetName()
		if n.GetHidden() {
			name += "#"
		}
		if n.GetSpeaking() {
			name += "*"
		}
		out = append(out, name)
	}
	return out
}

func wantMarks(t *testing.T, when string, got []*playv1.StageNpc, want ...string) {
	t.Helper()
	if g := stageMarks(got); !slices.Equal(g, want) {
		t.Errorf("%s: the stage is %v, want %v", when, g, want)
	}
}

// hiddenStage is a scene open with Aldo in plain sight and Zqvxorth hidden: the master's, Ana's streams.
type hiddenStage struct {
	*scenes
	aldo, secret          *charactersv1.Character
	masterWatch, anaWatch *watcher
	probeMap              string
}

func newHiddenStage(t *testing.T) *hiddenStage {
	t.Helper()
	s := newScenes(t, true)
	s.setup()
	h := &hiddenStage{scenes: s}
	h.aldo = s.master.createNPC(s.campaign, "Aldo", "")
	h.secret = s.master.createNPC(s.campaign, hiddenNPCName, "")
	h.probeMap = s.master.createMap(s.campaign, "Sonda", s.master.newImage(s.campaign)).GetId()
	s.master.mustOpenScene(s.campaign, s.point.GetId())
	h.masterWatch, h.anaWatch = s.master.watch(s.campaign), s.ana.watch(s.campaign)
	return h
}

// anaGot is everything the stream sent Ana up to a probe change that reaches every player. The master's
// stream is read up to the probe too, so the master's events that follow are the next ones.
func (h *hiddenStage) anaGot() []*playv1.WatchGameSessionResponse {
	h.h.t.Helper()
	h.probe(h.probeMap)
	h.masterWatch.drain(h.probeMap)
	return h.anaWatch.drain(h.probeMap)
}

func hasStageChanged(events []*playv1.WatchGameSessionResponse) bool {
	return slices.ContainsFunc(events, func(ev *playv1.WatchGameSessionResponse) bool { return ev.GetStageChanged() != nil })
}

// MR-031: the master puts an NPC on the stage hidden; it counts for the limit of 4, the master reads it
// marked, no player gets it or a hint of it, and putting it again changes nothing.
func TestMR031_TheMasterPutsAnNPCOnStageHidden(t *testing.T) {
	t.Parallel()
	h := newHiddenStage(t)
	s := h.scenes

	wantMarks(t, "Aldo in plain sight", s.master.mustPutOnStage(s.campaign, h.aldo.GetId()), "Aldo")
	h.masterWatch.stageChanged()
	h.anaWatch.stageChanged()

	stage, err := s.master.putHiddenOnStage(s.campaign, h.secret.GetId(), true)
	if err != nil {
		t.Fatalf("PutOnStage(hidden) error = %v", err)
	}
	wantMarks(t, "the master after the hidden put", stage, "Aldo", hiddenNPCName+"#")
	h.masterWatch.stageChanged() // the master hears of it
	wantMarks(t, "the master reads the scene", s.master.getScene(s.campaign).GetStage(), "Aldo", hiddenNPCName+"#")
	if got := h.anaGot(); hasStageChanged(got) || len(got) != 0 {
		t.Errorf("Ana's stream got %v for a hidden NPC put on the stage, want nothing", got)
	}
	wantMarks(t, "Ana reads the scene", s.ana.getScene(s.campaign).GetStage(), "Aldo")

	// Putting it again, hidden or not, changes nothing: SetStageNpcHidden is how it is shown.
	wantMarks(t, "put again, in plain sight", s.master.mustPutOnStage(s.campaign, h.secret.GetId()), "Aldo", hiddenNPCName+"#")
	if n := len(s.h.stageEvents()); n != 2 {
		t.Errorf("%d stage events, want 2: the repeated put wrote none", n)
	}

	// The hidden NPC counts for the limit of 4.
	for _, n := range []string{"Capitã", "Duda"} {
		s.master.mustPutOnStage(s.campaign, s.master.createNPC(s.campaign, n, "").GetId())
	}
	_, err = s.master.putOnStage(s.campaign, s.master.createNPC(s.campaign, "Eira", "").GetId())
	wantSceneBlocked(t, "PutOnStage(a fifth, one of the four hidden)", err, playv1.SceneBlockedReason_SCENE_BLOCKED_REASON_STAGE_FULL)

	// Taking the hidden NPC off tells only the master: the players never knew it.
	h.masterWatch.stageChanged() // Capitã
	h.masterWatch.stageChanged() // Duda
	h.anaWatch.stageChanged()
	h.anaWatch.stageChanged()
	wantMarks(t, "the hidden NPC taken off", s.master.takeOffStage(s.campaign, h.secret.GetId()), "Aldo", "Capitã", "Duda")
	h.masterWatch.stageChanged()
	if got := h.anaGot(); hasStageChanged(got) {
		t.Errorf("Ana's stream got %v for a hidden NPC taken off the stage, want nothing", got)
	}
}

// MR-031: turning an NPC between "Os jogadores veem" and "Escondida dos jogadores" tells everyone's
// stream, so the players read the scene again and the NPC appears or is gone; setting what it
// already is changes nothing; hiding the speaker takes the floor from it.
func TestMR031_TheMasterShowsAndHidesAnNPCOnStage(t *testing.T) {
	t.Parallel()
	h := newHiddenStage(t)
	s := h.scenes
	s.master.mustPutOnStage(s.campaign, h.aldo.GetId())
	s.master.mustPutOnStage(s.campaign, h.secret.GetId())
	h.masterWatch.stageChanged()
	h.masterWatch.stageChanged()
	h.anaWatch.stageChanged()
	h.anaWatch.stageChanged()
	wantMarks(t, "both in plain sight", s.ana.getScene(s.campaign).GetStage(), "Aldo", hiddenNPCName)

	wantMarks(t, "the secret hidden", s.master.mustSetStageNpcHidden(s.campaign, h.secret.GetId(), true), "Aldo", hiddenNPCName+"#")
	h.masterWatch.stageChanged()
	h.anaWatch.stageChanged() // Ana's app reads the scene again, and the NPC is gone
	wantMarks(t, "Ana after the hide", s.ana.getScene(s.campaign).GetStage(), "Aldo")

	s.master.mustSetStageNpcHidden(s.campaign, h.secret.GetId(), true) // already hidden: nothing
	if got := h.anaGot(); hasStageChanged(got) {
		t.Errorf("Ana's stream got %v for hiding an NPC already hidden, want nothing", got)
	}
	if n := len(s.h.stageEvents()); n != 3 {
		t.Errorf("%d stage events, want 3 (two puts and one hide): hiding it again wrote none", n)
	}

	wantMarks(t, "the secret shown", s.master.mustSetStageNpcHidden(s.campaign, h.secret.GetId(), false), "Aldo", hiddenNPCName)
	h.masterWatch.stageChanged()
	h.anaWatch.stageChanged()
	wantMarks(t, "Ana after the reveal", s.ana.getScene(s.campaign).GetStage(), "Aldo", hiddenNPCName)

	// The speaker, hidden, stops speaking; the shown NPC does not speak again by itself.
	s.master.mustSetSpeaker(s.campaign, h.secret.GetId())
	wantMarks(t, "the secret hidden while speaking", s.master.mustSetStageNpcHidden(s.campaign, h.secret.GetId(), true), "Aldo", hiddenNPCName+"#")
	wantMarks(t, "shown again", s.master.mustSetStageNpcHidden(s.campaign, h.secret.GetId(), false), "Aldo", hiddenNPCName)

	// Only an NPC on the stage can be turned; the call is the master's.
	_, err := s.master.setStageNpcHidden(s.campaign, s.master.createNPC(s.campaign, "Fora", "").GetId(), true)
	wantCode(t, "SetStageNpcHidden(an NPC off the stage)", err, connect.CodeNotFound)
	_, err = s.master.setStageNpcHidden(s.campaign, "não é um ID", true)
	wantCode(t, "SetStageNpcHidden(not an ID)", err, connect.CodeNotFound)
	_, err = s.ana.setStageNpcHidden(s.campaign, h.secret.GetId(), false)
	wantCode(t, "SetStageNpcHidden(a player)", err, connect.CodePermissionDenied)
	s.master.closeScene(s.campaign)
	_, err = s.master.setStageNpcHidden(s.campaign, h.secret.GetId(), true)
	wantCode(t, "SetStageNpcHidden(no scene, so no stage)", err, connect.CodeNotFound)

	// The history keeps ids only, and says what changed.
	events := strings.Join(s.h.stageEvents(), "\n")
	for _, want := range []string{`"change":"hidden"`, `"change":"shown"`, h.secret.GetId()} {
		if !strings.Contains(events, want) {
			t.Errorf("the stage events %q lack %s", events, want)
		}
	}
	if strings.Contains(events, hiddenNPCName) {
		t.Errorf("the history holds the hidden NPC's name: %s", events)
	}
}

// MR-031: "Dar a fala" on a hidden NPC shows it in the same call (the master's tap means that it
// speaks now), and the scene's players hear of it at once.
func TestMR031_GivingTheFloorShowsAHiddenNPC(t *testing.T) {
	t.Parallel()
	h := newHiddenStage(t)
	s := h.scenes
	if _, err := s.master.putHiddenOnStage(s.campaign, h.secret.GetId(), true); err != nil {
		t.Fatal(err)
	}
	h.masterWatch.stageChanged()

	stage := s.master.mustSetSpeaker(s.campaign, h.secret.GetId())
	wantMarks(t, "the hidden NPC given the floor", stage, hiddenNPCName+"*")
	h.masterWatch.stageChanged()
	h.anaWatch.stageChanged()
	got := s.ana.getScene(s.campaign).GetStage()
	if len(got) != 1 || got[0].GetName() != hiddenNPCName || !got[0].GetSpeaking() || got[0].GetHidden() {
		t.Errorf("Ana's stage = %v, want the NPC, speaking", got)
	}
	if ev := s.h.stageEvents(); !strings.Contains(ev[len(ev)-1], `"change":"speaker"`) || !strings.Contains(ev[len(ev)-1], `"revealed":true`) {
		t.Errorf("the last stage event = %s, want the speaker change that revealed the NPC", ev[len(ev)-1])
	}

	// An NPC in plain sight given the floor reveals nothing.
	s.master.mustSetSpeaker(s.campaign, "")
	s.master.mustPutOnStage(s.campaign, h.aldo.GetId())
	s.master.mustSetSpeaker(s.campaign, h.aldo.GetId())
	if ev := s.h.stageEvents(); strings.Contains(ev[len(ev)-1], "revealed") {
		t.Errorf("the last stage event = %s, want no reveal", ev[len(ev)-1])
	}
}

// MR-031 and RN-10: a player downloads a portrait only while its NPC is on the stage and shown.
func TestMR031_AHiddenNPCsPortraitIs404ForPlayers(t *testing.T) {
	t.Parallel()
	s := newScenes(t, true)
	s.setup()
	img := s.master.mustUpload(s.campaign, "zqvx.png", pngImage(t, 40, 30))
	npc := s.master.createNPC(s.campaign, hiddenNPCName, img.GetId())
	s.master.mustOpenScene(s.campaign, s.point.GetId())
	status := func(who *user) int { return who.get(img.GetUrl()).status }
	thumb := func(who *user) int { return who.get(img.GetThumbnailUrl()).status }

	if _, err := s.master.putHiddenOnStage(s.campaign, npc.GetId(), true); err != nil {
		t.Fatal(err)
	}
	for _, who := range []*user{s.ana, s.caio} {
		if got, tg := status(who), thumb(who); got != http.StatusNotFound || tg != http.StatusNotFound {
			t.Errorf("hidden on stage: a player gets %d and %d for the portrait, want 404", got, tg)
		}
	}
	etag := `"` + img.GetId() + `"`
	if res := s.ana.get(img.GetUrl(), "If-None-Match", etag); res.status != http.StatusNotFound {
		t.Errorf("revalidating a hidden NPC's portrait: status %d, want 404, not a 304", res.status)
	}
	if got := status(s.master); got != http.StatusOK {
		t.Errorf("the master gets %d for the portrait, want 200", got)
	}
	s.master.mustSetStageNpcHidden(s.campaign, npc.GetId(), false)
	if got := status(s.ana); got != http.StatusOK {
		t.Errorf("shown on stage: Ana gets %d, want 200", got)
	}
	s.master.mustSetStageNpcHidden(s.campaign, npc.GetId(), true)
	if got := status(s.ana); got != http.StatusNotFound {
		t.Errorf("hidden again: Ana gets %d, want 404", got)
	}
}

// RN-10: whatever a player is sent, as JSON, never holds a hidden stage NPC: not its name, its
// character ID, its place on the stage, its portrait. The master, as the positive control, gets
// the name and the ID; Ana's reads and her stream get neither.
func TestRN10_AHiddenStageNPCNeverReachesAPlayer(t *testing.T) {
	t.Parallel()
	s := newScenes(t, true)
	s.setup()
	portrait := s.master.newImage(s.campaign)
	secret := s.master.createNPC(s.campaign, hiddenNPCName, portrait)
	aldo := s.master.createNPC(s.campaign, "Aldo", "")
	anaWatch := s.ana.watch(s.campaign)
	var seen []string
	see := func(m proto.Message) { seen = append(seen, protojson.Format(m)) }
	drain := func(n int) {
		t.Helper()
		for range n {
			see(anaWatch.next())
		}
	}

	s.master.mustOpenScene(s.campaign, s.point.GetId())
	s.master.mustPutOnStage(s.campaign, aldo.GetId())
	hiddenStage, err := s.master.putHiddenOnStage(s.campaign, secret.GetId(), true)
	if err != nil {
		t.Fatal(err)
	}
	s.master.mustSetSpeaker(s.campaign, aldo.GetId())
	s.master.mustSetStageNpcHidden(s.campaign, secret.GetId(), false)
	s.master.mustSetStageNpcHidden(s.campaign, secret.GetId(), true)
	s.master.takeOffStage(s.campaign, secret.GetId())
	drain(5) // scene_changed, stage_changed (Aldo), speaker, shown, hidden: the hidden put and the take-off are not told
	see(s.ana.getScene(s.campaign))
	see(s.ana.mustGetMap(s.campaign, s.mapID))
	live, err := s.ana.play.GetLiveSession(t.Context(), connect.NewRequest(&playv1.GetLiveSessionRequest{CampaignId: s.campaign}))
	if err != nil {
		t.Fatalf("GetLiveSession() error = %v", err)
	}
	see(live.Msg)

	// The positive control: the master reads the NPC, by name and ID.
	masterText := protojson.Format(&playv1.GetOpenSceneResponse{Scene: s.master.getScene(s.campaign)}) + protojson.Format(&playv1.PutOnStageResponse{Stage: hiddenStage})
	if !strings.Contains(masterText, hiddenNPCName) || !strings.Contains(masterText, secret.GetId()) {
		t.Fatalf("the master was not given the hidden NPC (name and id), so the check proves nothing:\n%s", masterText)
	}
	for _, text := range seen {
		for _, needle := range []string{hiddenNPCName, secret.GetId(), portrait} {
			if strings.Contains(text, needle) {
				t.Errorf("Ana was sent %q:\n%s", needle, text)
			}
		}
	}
}
