package maps

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The hidden scene actions (MR-015, RN-10): the master hides an action while preparing it or live,
// and a hidden action never reaches a player.

// A made-up name that is neither a word nor hexadecimal (docs/testing.md, Needles), and a DC that
// no other action of these tests has.
const (
	hiddenActionName = "Qzvwlx-Dorn-9942"
	hiddenActionDC   = 27
)

func (u *user) addHiddenAction(campaignID, mapID, pointID, key, name string, dc int32, hidden bool) *mapsv1.SceneAction {
	u.h.t.Helper()
	res, err := u.maps.AddSceneAction(u.h.t.Context(), connect.NewRequest(&mapsv1.AddSceneActionRequest{
		CampaignId: campaignID, MapId: mapID, PointId: pointID, Key: key, Name: name, Dc: dc, Hidden: hidden,
	}))
	if err != nil {
		u.h.t.Fatalf("AddSceneAction(%s, hidden = %v) error = %v", key, hidden, err)
	}
	return res.Msg.GetAction()
}

// setActionHidden changes only the visibility of an action.
func (s *scenes) setActionHidden(a *mapsv1.SceneAction, hidden bool) *mapsv1.SceneAction {
	s.h.t.Helper()
	res, err := s.update(s.master, a, func(r *mapsv1.UpdateSceneActionRequest) { r.Hidden = proto.Bool(hidden) })
	if err != nil {
		s.h.t.Fatalf("UpdateSceneAction(hidden = %v) error = %v", hidden, err)
	}
	return res.GetAction()
}

// actionIDsOf lists the IDs of the actions of an open scene.
func actionIDsOf(scene *playv1.OpenSceneInfo) []string {
	var out []string
	for _, a := range scene.GetActions() {
		out = append(out, a.GetId())
	}
	return out
}

// hiddenActions is a revealed scene point, open, with the master's three actions and a hidden one;
// the master's and Ana's streams are watched.
type hiddenActions struct {
	*scenes
	hidden                *mapsv1.SceneAction
	masterWatch, anaWatch *watcher
	probeMap              string
}

func newHiddenActions(t *testing.T) *hiddenActions {
	t.Helper()
	s := newScenes(t, true)
	s.setup()
	s.master.setPointRevealed(s.campaign, s.point, true)
	h := &hiddenActions{scenes: s}
	h.probeMap = s.master.createMap(s.campaign, "Sonda", s.master.newImage(s.campaign)).GetId()
	s.master.mustOpenScene(s.campaign, s.point.GetId())
	h.masterWatch, h.anaWatch = s.master.watch(s.campaign), s.ana.watch(s.campaign)
	return h
}

// anaGot is everything the stream sent Ana up to a probe change that reaches every player; the master's
// stream is read up to the probe too.
func (h *hiddenActions) anaGot() []*playv1.WatchGameSessionResponse {
	h.h.t.Helper()
	h.probe(h.probeMap)
	h.masterWatch.drain(h.probeMap)
	return h.anaWatch.drain(h.probeMap)
}

func hasSceneOrMapChanged(events []*playv1.WatchGameSessionResponse) bool {
	return slices.ContainsFunc(events, func(ev *playv1.WatchGameSessionResponse) bool {
		return ev.GetSceneChanged() != nil || ev.GetMapChanged() != nil
	})
}

// MR-015: a hidden action added while preparing is the master's alone: Ana's map and open scene do
// not list it, her stream hears nothing, and a roll of hers on it is refused exactly like a roll on an
// action that does not exist.
func TestMR015_AHiddenActionIsTheMastersAlone(t *testing.T) {
	t.Parallel()
	h := newHiddenActions(t)
	s := h.scenes
	hidden := s.master.addHiddenAction(s.campaign, s.mapID, s.point.GetId(), "skill:stealth", hiddenActionName, hiddenActionDC, true)
	if !hidden.GetHidden() {
		t.Fatalf("the added action = %v, want it hidden", hidden)
	}
	h.masterWatch.sceneChanged() // the master's screen reads the scene again
	if got := h.anaGot(); hasSceneOrMapChanged(got) {
		t.Errorf("Ana's stream got %v for a hidden action added, want nothing", got)
	}

	// The master reads it, marked, on the map and in the open scene.
	var masterKeys []string
	for _, a := range s.masterPointOf(s.point.GetId()).GetSceneActions() {
		masterKeys = append(masterKeys, a.GetKey())
		if a.GetId() == hidden.GetId() && (!a.GetHidden() || a.GetDc() != hiddenActionDC) {
			t.Errorf("the master's map action = %v, want it hidden, DC %d", a, hiddenActionDC)
		}
	}
	if !slices.Contains(masterKeys, "skill:stealth") {
		t.Errorf("the master's point has %v, want the hidden action among them", masterKeys)
	}
	if a := actionByID(s.master.getScene(s.campaign), hidden.GetId()); a == nil || !a.GetHidden() || a.GetName() != hiddenActionName {
		t.Errorf("the master's open scene action = %v, want the hidden action", a)
	}

	// Ana reads three actions on the map and in the scene, none of them hidden.
	for _, p := range s.ana.mustGetMap(s.campaign, s.mapID).GetPoints() {
		if len(p.GetSceneActions()) != 3 {
			t.Errorf("Ana's point has %d actions, want the 3 the players see", len(p.GetSceneActions()))
		}
	}
	scene := s.ana.getScene(s.campaign)
	if len(scene.GetActions()) != 3 || slices.Contains(actionIDsOf(scene), hidden.GetId()) {
		t.Errorf("Ana's open scene actions = %v, want 3 without the hidden one", scene.GetActions())
	}

	// A roll on it is refused as an action that does not exist: the same code and message.
	_, errHidden := s.ana.rollWith(s.campaign, hidden.GetId(), 10, newKey())
	_, errUnknown := s.ana.rollWith(s.campaign, "00000000-0000-4000-8000-000000000000", 10, newKey())
	wantRefusal(t, "a roll on a hidden action", errHidden, connect.CodeNotFound, "action not found")
	wantRefusal(t, "a roll on an unknown action", errUnknown, connect.CodeNotFound, "action not found")
	if n := s.count("scene_check_rolled"); n != 0 {
		t.Errorf("%d rolls were written, want none", n)
	}
	// The master still grants attempts on it, and reads the table's rolls.
	if _, err := s.master.grantAttempt(s.campaign, hidden.GetId(), s.pens.GetId(), newKey()); err != nil {
		t.Errorf("GrantSceneAttempt(a hidden action) error = %v, want it granted", err)
	}
}

// MR-015: the master turns an action live. Showing it offers it to the players at once; hiding it takes
// it from them at once, with the rolls they made on it; nothing is erased, so showing it again gives
// them back their rolls and attempts. Changing an action that stays hidden tells the master alone.
func TestMR015_TheMasterShowsAndHidesAnActionLive(t *testing.T) {
	t.Parallel()
	h := newHiddenActions(t)
	s := h.scenes
	action := s.master.addHiddenAction(s.campaign, s.mapID, s.point.GetId(), "skill:stealth", hiddenActionName, hiddenActionDC, true)
	h.masterWatch.sceneChanged()
	if got := h.anaGot(); hasSceneOrMapChanged(got) {
		t.Fatalf("Ana's stream got %v for a hidden action added, want nothing", got)
	}

	// Edits of a hidden action stay with the master, including a new name and a new DC.
	if _, err := s.update(s.master, action, func(r *mapsv1.UpdateSceneActionRequest) {
		r.Name = proto.String(hiddenActionName + "b")
		r.Dc = proto.Int32(hiddenActionDC)
	}); err != nil {
		t.Fatal(err)
	}
	h.masterWatch.sceneChanged()
	if got := h.anaGot(); hasSceneOrMapChanged(got) {
		t.Errorf("Ana's stream got %v for an edit of a hidden action, want nothing", got)
	}

	// Showing it: the players hear of it at once, and may roll it.
	shown := s.setActionHidden(action, false)
	if shown.GetHidden() {
		t.Errorf("the shown action = %v, want it not hidden", shown)
	}
	h.anaWatch.nextSceneChanged()
	h.masterWatch.sceneChanged()
	if scene := s.ana.getScene(s.campaign); len(scene.GetActions()) != 4 || !slices.Contains(actionIDsOf(scene), action.GetId()) {
		t.Fatalf("Ana's actions after showing = %v, want 4 with the new one", scene.GetActions())
	}
	if got := s.ana.mustGetMap(s.campaign, s.mapID).GetPoints()[0]; len(got.GetSceneActions()) != 4 {
		t.Errorf("Ana's map point has %d actions after showing, want 4", len(got.GetSceneActions()))
	}
	roll := s.ana.mustRoll(s.campaign, action.GetId(), 12)
	if len(s.ana.getScene(s.campaign).GetRolls()) != 1 {
		t.Fatalf("Ana's rolls after the roll = %v, want hers", s.ana.getScene(s.campaign).GetRolls())
	}

	// Hiding it: gone at once for Ana, with her roll; the master keeps everything.
	if !s.setActionHidden(action, true).GetHidden() {
		t.Errorf("the hidden action does not say hidden")
	}
	h.anaWatch.nextSceneChanged()
	scene := s.ana.getScene(s.campaign)
	if len(scene.GetActions()) != 3 || len(scene.GetRolls()) != 0 {
		t.Errorf("Ana's scene after hiding = %d actions, %d rolls, want 3 and 0", len(scene.GetActions()), len(scene.GetRolls()))
	}
	_, err := s.ana.rollWith(s.campaign, action.GetId(), 10, newKey())
	wantRefusal(t, "a roll on the action hidden again", err, connect.CodeNotFound, "action not found")
	masterScene := s.master.getScene(s.campaign)
	if len(masterScene.GetActions()) != 4 || len(masterScene.GetRolls()) != 1 || masterScene.GetRolls()[0].GetId() != roll.GetId() {
		t.Errorf("the master's scene = %d actions, %d rolls, want 4 and Ana's roll", len(masterScene.GetActions()), len(masterScene.GetRolls()))
	}

	// Showing it again gives Ana back her roll: nothing was erased.
	s.setActionHidden(action, false)
	h.anaWatch.nextSceneChanged()
	if got := s.ana.getScene(s.campaign).GetRolls(); len(got) != 1 || got[0].GetId() != roll.GetId() {
		t.Errorf("Ana's rolls after showing again = %v, want the roll she made", got)
	}
}

// MR-015: a hidden action never reaches a player in the preparing scene either: it is not on a
// revealed point's map, and the point's other changes still do.
func TestMR015_AHiddenActionIsNotOnAPlayersMap(t *testing.T) {
	t.Parallel()
	s := newScenes(t, false)
	s.setup()
	s.master.addHiddenAction(s.campaign, s.mapID, s.point.GetId(), "skill:stealth", hiddenActionName, hiddenActionDC, true)
	s.master.setPointRevealed(s.campaign, s.point, true)
	if got := s.ana.mustGetMap(s.campaign, s.mapID).GetPoints()[0].GetSceneActions(); len(got) != 3 {
		t.Errorf("Ana's map actions = %v, want the 3 the players see", got)
	}
	if got := s.masterPointOf(s.point.GetId()).GetSceneActions(); len(got) != 4 {
		t.Errorf("the master's map actions = %d, want 4", len(got))
	}
	// An action can be added hidden and edited with the visibility alone; nothing to change is still refused.
	_, err := s.update(s.master, s.masterPointOf(s.point.GetId()).GetSceneActions()[0], func(*mapsv1.UpdateSceneActionRequest) {})
	wantCode(t, "UpdateSceneAction(nothing)", err, connect.CodeInvalidArgument)
	// The idempotency key covers the visibility: the same key with another one is another request.
	add := func(hidden bool) error {
		_, err := s.master.maps.AddSceneAction(t.Context(), connect.NewRequest(&mapsv1.AddSceneActionRequest{
			CampaignId: s.campaign, MapId: s.mapID, PointId: s.point.GetId(), Key: "skill:history", IdempotencyKey: "uma-chave-de-acao", Hidden: hidden,
		}))
		return err
	}
	if err := add(true); err != nil {
		t.Fatalf("AddSceneAction(hidden) error = %v", err)
	}
	wantCode(t, "the same key, shown", add(false), connect.CodeInvalidArgument)
}

// RN-10: whatever a player is sent, as JSON, never holds a hidden action: not its name, its DC (which
// the scene shows for the others), its ID, or a roll on it. The master, as the positive control, gets
// the name, the DC and the ID.
func TestRN10_AHiddenSceneActionNeverReachesAPlayer(t *testing.T) {
	t.Parallel()
	h := newHiddenActions(t)
	s := h.scenes
	s.mustSetShowDC(true) // the DCs of the others reach the players: the hidden one's must not
	h.masterWatch.sceneChanged()
	h.anaWatch.nextSceneChanged()
	hidden := s.master.addHiddenAction(s.campaign, s.mapID, s.point.GetId(), "skill:stealth", hiddenActionName, hiddenActionDC, true)
	s.ana.mustRoll(s.campaign, s.master.addAction(s.campaign, s.mapID, s.point.GetId(), "skill:history", "Visível", 14).GetId(), 9)
	var seen []string
	see := func(m proto.Message) { seen = append(seen, protojson.Format(m)) }
	for _, ev := range h.anaGot() {
		see(ev)
	}
	see(s.ana.getScene(s.campaign))
	see(s.ana.mustGetMap(s.campaign, s.mapID))
	_, err := s.ana.rollWith(s.campaign, hidden.GetId(), 10, newKey())
	if err == nil {
		t.Fatal("a roll on a hidden action was accepted")
	}
	seen = append(seen, err.Error())

	masterText := protojson.Format(s.master.getScene(s.campaign)) + protojson.Format(s.master.mustGetMap(s.campaign, s.mapID))
	dc := regexp.MustCompile(`"dc":\s*27\b`)
	if !strings.Contains(masterText, hiddenActionName) || !strings.Contains(masterText, hidden.GetId()) || !dc.MatchString(masterText) {
		t.Fatalf("the master was not given the hidden action (name, id, DC), so the check proves nothing:\n%s", masterText)
	}
	if !regexp.MustCompile(`"dc":\s*14\b`).MatchString(strings.Join(seen, "\n")) {
		t.Errorf("Ana was not given the DC of the visible action: the scene shows it, so the check proves nothing")
	}
	for _, text := range seen {
		for _, needle := range []string{hiddenActionName, hidden.GetId()} {
			if strings.Contains(text, needle) {
				t.Errorf("Ana was sent %q:\n%s", needle, text)
			}
		}
		if dc.MatchString(text) {
			t.Errorf("Ana was sent the hidden action's DC:\n%s", text)
		}
	}
}
