package leaktest

import (
	"bytes"
	"strings"
	"testing"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1/mapsv1connect"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1/playv1connect"
	"google.golang.org/protobuf/proto"
)

// MR-031, MR-015, RN-10: the master hid an NPC on the stage and an action of the open scene. The
// fixture's marker names, their ids, the NPC's portrait and the action's DC are the needles (unique
// markers and an exact id, not words); the master's answer to GetOpenScene holds all of them (the
// positive control), and no player's holds any. TestLeakMatrix already asks every read and the stream;
// these tests add what only they can: the refusal of a roll, the reveal and the hide again.

// openSceneAs asks for the open scene as p.
func (w *world) openSceneAs(p *person) reply {
	return p.call(playv1connect.PlayServiceGetOpenSceneProcedure, &playv1.GetOpenSceneRequest{CampaignId: w.campaign})
}

// needlesOf lists the needles of the canaries of the given kinds.
func (w *world) needlesOf(kinds ...string) []string {
	var out []string
	for _, c := range w.secrets.list {
		for _, k := range kinds {
			if c.kind == k {
				out = append(out, c.needle)
			}
		}
	}
	return out
}

func holdsAny(body []byte, needles []string) (string, bool) {
	for _, n := range needles {
		if bytes.Contains(body, []byte(n)) {
			return n, true
		}
	}
	return "", false
}

func TestRN10_AHiddenStageNPCIsTheMastersAlone(t *testing.T) {
	w := newWorld(t)
	ctx := t.Context()
	needles := append(w.needlesOf("hidden-stage-name"), w.hiddenStageNPC.GetId())
	if len(needles) != 2 {
		t.Fatalf("the needles of the hidden stage NPC = %v, want its name and its id", needles)
	}

	// The positive control: the master reads the NPC, by name and id, marked hidden.
	master := w.openSceneAs(w.master)
	for _, n := range needles {
		if !bytes.Contains(master.body, []byte(n)) {
			t.Fatalf("the master's open scene lacks %q, so the check proves nothing: %s", n, master.body)
		}
	}
	// No player, pending member, stranger or signed-out caller reads it.
	players := append(w.players(), w.anonymous())
	for _, p := range players {
		if n, found := holdsAny(w.openSceneAs(p).body, needles); found {
			t.Errorf("%s reads the hidden stage NPC (%s) in the open scene", p.name, n)
		}
	}
	// Its portrait is 404 for them and 200 for the master.
	if status, _ := w.master.get("/images/" + w.imgHiddenStage); status != 200 {
		t.Errorf("the master gets HTTP %d for the hidden NPC's portrait, want 200", status)
	}
	for _, p := range w.players() {
		if status, _ := p.get("/images/" + w.imgHiddenStage); status != 404 {
			t.Errorf("%s gets HTTP %d for the hidden NPC's portrait, want 404", p.name, status)
		}
	}

	// "Dar a fala" shows it: Ana and Caio read it where they may, and Bia (pending) and Eva (a stranger) still do not.
	must(w.master.play.SetSpeaker(ctx, rq(&playv1.SetSpeakerRequest{CampaignId: w.campaign, CharacterId: w.hiddenStageNPC.GetId()})))
	for _, p := range []*person{w.ana, w.caio} {
		if !bytes.Contains(w.openSceneAs(p).body, []byte(needles[0])) {
			t.Errorf("%s does not read the NPC the master gave the floor to", p.name)
		}
		if status, _ := p.get("/images/" + w.imgHiddenStage); status != 200 {
			t.Errorf("%s gets HTTP %d for the portrait of the NPC that speaks, want 200", p.name, status)
		}
	}
	for _, p := range []*person{w.pending, w.stranger, w.anonymous()} {
		if n, found := holdsAny(w.openSceneAs(p).body, needles); found {
			t.Errorf("%s reads the NPC the master gave the floor to (%s)", p.name, n)
		}
	}

	// Hidden again, it is gone from them at once.
	must(w.master.play.SetStageNpcHidden(ctx, rq(&playv1.SetStageNpcHiddenRequest{CampaignId: w.campaign, CharacterId: w.hiddenStageNPC.GetId(), Hidden: true})))
	for _, p := range players {
		if n, found := holdsAny(w.openSceneAs(p).body, needles); found {
			t.Errorf("%s reads the NPC hidden again (%s)", p.name, n)
		}
	}
}

func TestRN10_AHiddenSceneActionIsTheMastersAlone(t *testing.T) {
	w := newWorld(t)
	ctx := t.Context()
	scene := w.pts["scene-open"]
	needles := append(w.needlesOf("scene-hidden-action"), w.hiddenAction)
	if len(needles) != 2 {
		t.Fatalf("the needles of the hidden action = %v, want its name and its id", needles)
	}
	getMap := func(p *person) reply {
		return p.call(mapsv1connect.MapServiceGetMapProcedure, &mapsv1.GetMapRequest{CampaignId: w.campaign, MapId: w.fogMap})
	}

	// The positive control: the master reads the action, by name, id and DC, in the scene and on the map.
	for what, r := range map[string]reply{"the open scene": w.openSceneAs(w.master), "the map": getMap(w.master)} {
		for _, n := range needles {
			if !bytes.Contains(r.body, []byte(n)) {
				t.Fatalf("the master's %s lacks %q, so the check proves nothing: %s", what, n, r.body)
			}
		}
		if !strings.Contains(string(r.body), `"dc":30`) && !strings.Contains(string(r.body), `"dc": 30`) {
			t.Fatalf("the master's %s lacks the hidden action's DC: %s", what, r.body)
		}
	}
	// No player reads it, and the DC (30, in a field named dc) is not in what they read.
	players := append(w.players(), w.anonymous())
	for _, p := range players {
		for what, r := range map[string]reply{"the open scene": w.openSceneAs(p), "the map": getMap(p)} {
			if n, found := holdsAny(r.body, needles); found {
				t.Errorf("%s reads the hidden action (%s) in %s", p.name, n, what)
			}
			if strings.Contains(string(r.body), `"dc":30`) || strings.Contains(string(r.body), `"dc": 30`) {
				t.Errorf("%s reads the hidden action's DC in %s", p.name, what)
			}
		}
	}

	// A roll on it is refused like a roll on an action that does not exist: the same status, code and message.
	roll := func(p *person, action string) reply {
		return p.call(playv1connect.PlayServiceRollSceneCheckProcedure, &playv1.RollSceneCheckRequest{
			CampaignId: w.campaign, ActionId: action, IdempotencyKey: newKey(), Roll: &playv1.RollSceneCheckRequest_D20Face{D20Face: 12},
		})
	}
	unknown := roll(w.ana, "00000000-0000-4000-8000-000000000000")
	hidden := roll(w.ana, w.hiddenAction)
	if unknown.ok() || hidden.status != unknown.status || !bytes.Equal(hidden.body, unknown.body) {
		t.Errorf("a roll on the hidden action = HTTP %d %s, on an unknown one = HTTP %d %s, want the same refusal", hidden.status, hidden.body, unknown.status, unknown.body)
	}

	// The master shows it: the scene's players read it where they may, and can roll it.
	setHidden := func(hide bool) {
		must(w.master.maps.UpdateSceneAction(ctx, rq(&mapsv1.UpdateSceneActionRequest{
			CampaignId: w.campaign, MapId: scene.GetMapId(), PointId: scene.GetId(), ActionId: w.hiddenAction, Hidden: proto.Bool(hide),
		})))
	}
	setHidden(false)
	for _, p := range []*person{w.ana, w.caio} {
		if !bytes.Contains(w.openSceneAs(p).body, []byte(needles[0])) {
			t.Errorf("%s does not read the action the master showed", p.name)
		}
	}
	if r := roll(w.ana, w.hiddenAction); !r.ok() {
		t.Errorf("Ana's roll on the action once shown = HTTP %d %s, want it accepted", r.status, r.body)
	}
	for _, p := range []*person{w.pending, w.stranger, w.anonymous()} {
		if n, found := holdsAny(w.openSceneAs(p).body, needles); found {
			t.Errorf("%s reads the action the master showed (%s)", p.name, n)
		}
	}

	// Hidden again, it is gone from them at once, with the roll Ana made on it.
	setHidden(true)
	for _, p := range players {
		if n, found := holdsAny(w.openSceneAs(p).body, needles); found {
			t.Errorf("%s reads the action hidden again (%s)", p.name, n)
		}
	}
	if r := w.openSceneAs(w.ana); strings.Contains(string(r.body), w.hiddenAction) {
		t.Errorf("Ana still reads a roll on the action hidden again: %s", r.body)
	}
	if r := w.openSceneAs(w.master); !bytes.Contains(r.body, []byte(w.hiddenAction)) {
		t.Errorf("the master no longer reads the action hidden again: %s", r.body)
	}
}
