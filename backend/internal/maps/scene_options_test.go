package maps

import (
	"errors"
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

// The scene options (MR-015, Etapa 8; Samuel's answers to questions 52 and
// 55) and the session summary's checks (MR-032, question 64): "Mostrar a CD aos
// jogadores", the attempts per action, "Dar mais uma tentativa". Pensantus's
// numbers are the golden file's: Investigação +6; Toren's, a gnome wizard
// with Intelligence 16 and the gnome bonus: Investigação +4 (no proficiency).

// nextSceneChanged reads events until a scene_changed, skipping the
// scene_check_rolled of a roll the watcher's user made.
func (w *watcher) nextSceneChanged() *playv1.WatchGameSessionResponse {
	w.t.Helper()
	for {
		ev := w.next()
		if ev.GetSceneChanged() != nil {
			return ev
		}
		if ev.GetSceneCheckRolled() == nil && ev.GetMapChanged() == nil {
			w.t.Fatalf("event = %v, want scene_changed", ev)
		}
	}
}

// setShowDC turns the scene's "Mostrar a CD aos jogadores" on or off.
func (s *scenes) setShowDC(on bool) (*mapsv1.MapPoint, error) {
	return s.master.updatePoint(&mapsv1.UpdateMapPointRequest{CampaignId: s.campaign, MapId: s.mapID, PointId: s.point.GetId(), ShowDc: &on})
}

func (s *scenes) mustSetShowDC(on bool) {
	s.h.t.Helper()
	if _, err := s.setShowDC(on); err != nil {
		s.h.t.Fatalf("UpdateMapPoint(show_dc = %v) error = %v", on, err)
	}
}

// addActionWith adds an action with a limit of attempts (0 is unlimited).
func (s *scenes) addActionWith(key string, dc, attempts int32) *mapsv1.SceneAction {
	s.h.t.Helper()
	res, err := s.master.maps.AddSceneAction(s.h.t.Context(), connect.NewRequest(&mapsv1.AddSceneActionRequest{
		CampaignId: s.campaign, MapId: s.mapID, PointId: s.point.GetId(), Key: key, Dc: dc, MaxAttempts: proto.Int32(attempts),
	}))
	if err != nil {
		s.h.t.Fatalf("AddSceneAction(%s, attempts %d) error = %v", key, attempts, err)
	}
	return res.Msg.GetAction()
}

func (s *scenes) setAttempts(a *mapsv1.SceneAction, attempts int32) error {
	_, err := s.update(s.master, a, func(r *mapsv1.UpdateSceneActionRequest) { r.MaxAttempts = proto.Int32(attempts) })
	return err
}

func (u *user) grantAttempt(campaignID, actionID, characterID, key string) (*playv1.OpenSceneInfo, error) {
	res, err := u.play.GrantSceneAttempt(u.h.t.Context(), connect.NewRequest(&playv1.GrantSceneAttemptRequest{
		CampaignId: campaignID, ActionId: actionID, CharacterId: characterID, IdempotencyKey: key,
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg.GetScene(), nil
}

func (u *user) mustGrantAttempt(campaignID, actionID, characterID string) *playv1.OpenSceneInfo {
	u.h.t.Helper()
	scene, err := u.grantAttempt(campaignID, actionID, characterID, newKey())
	if err != nil {
		u.h.t.Fatalf("GrantSceneAttempt() error = %v", err)
	}
	return scene
}

// left is the attempts a player's scene says they have at an action: -1 when
// the count is unset (unlimited).
func left(scene *playv1.OpenSceneInfo, actionID string) int32 {
	a := actionByID(scene, actionID)
	if a == nil {
		return -2
	}
	if a.AttemptsLeft == nil {
		return -1
	}
	return a.GetAttemptsLeft()
}

// TestMR015_ShowTheDCToPlayers: the switch is per scene and off by default.
// Off, a player gets no DC and no pass or fail (RN-20, as before). On, they
// get the DC of each action that has one, and the pass or fail of their own
// rolls; an action with no DC shows nothing extra either way; the master
// always sees the DC.
func TestMR015_ShowTheDCToPlayers(t *testing.T) {
	t.Parallel()
	s := newScenes(t, true)
	invest, force, wis := s.setup() // DC 15, none, 12
	s.master.setPointRevealed(s.campaign, s.point, true)
	anaWatch, caioWatch := s.ana.watch(s.campaign), s.caio.watch(s.campaign)

	if s.point.GetShowDc() {
		t.Error("a new scene shows its DC: it must be off by default")
	}
	scene := s.master.mustOpenScene(s.campaign, s.point.GetId())
	if scene.GetShowDc() || actionByID(scene, invest.GetId()).GetDc() != 15 {
		t.Errorf("the master's scene = show_dc %v, DC %d; want off, and the DC 15 anyway", scene.GetShowDc(), actionByID(scene, invest.GetId()).GetDc())
	}
	for _, w := range []*watcher{anaWatch, caioWatch} {
		w.nextSceneChanged()
	}

	// Off: the player gets neither the DC nor the pass or fail.
	roll := s.ana.mustRoll(s.campaign, invest.GetId(), 11) // 17 reaches 15
	if roll.Passed != nil {
		t.Errorf("a roll in a scene that hides its DC carries passed = %v", *roll.Passed)
	}
	view := s.ana.getScene(s.campaign)
	if view.GetShowDc() || actionByID(view, invest.GetId()).GetDc() != 0 || actionByID(view, wis.GetId()).GetDc() != 0 {
		t.Errorf("Ana's scene with the switch off = %v, want no DC", view)
	}
	if view.GetRolls()[0].Passed != nil {
		t.Errorf("Ana's roll log carries passed with the switch off")
	}
	if p := s.ana.mustGetMap(s.campaign, s.mapID).GetPoints()[0]; p.GetShowDc() || p.GetSceneActions()[0].GetDc() != 0 {
		t.Errorf("Ana's map with the switch off = %v, want no DC", p)
	}

	// On: the app is told to read the scene again, and a player gets the DCs
	// and the pass or fail of their own rolls.
	s.mustSetShowDC(true)
	anaWatch.nextSceneChanged()
	caioWatch.nextSceneChanged()
	view = s.ana.getScene(s.campaign)
	if !view.GetShowDc() {
		t.Error("Ana's scene with the switch on says show_dc = false")
	}
	for id, dc := range map[string]int32{invest.GetId(): 15, force.GetId(): 0, wis.GetId(): 12} {
		if got := actionByID(view, id).GetDc(); got != dc {
			t.Errorf("Ana's DC of %s = %d, want %d (0 is an action with none)", id, got, dc)
		}
	}
	// The roll made while the DC was hidden stays without a pass or fail for her,
	// whatever the switch says now; the master always reads it.
	if r := view.GetRolls()[0]; r.Passed != nil {
		t.Errorf("Ana's roll made with the DC hidden = %v, want no passed after the switch went on", r)
	}
	for _, r := range s.master.getScene(s.campaign).GetRolls() {
		if r.GetActionId() == invest.GetId() && (r.Passed == nil || !r.GetPassed()) {
			t.Errorf("the master's roll = %v, want passed always", r)
		}
	}
	if p := s.ana.mustGetMap(s.campaign, s.mapID).GetPoints()[0]; !p.GetShowDc() || p.GetSceneActions()[0].GetDc() != 15 {
		t.Errorf("Ana's map with the switch on = %v, want the DC", p)
	}
	miss := s.ana.mustRoll(s.campaign, wis.GetId(), 2) // 2 + 4 = 6 misses 12
	if miss.Passed == nil || miss.GetPassed() {
		t.Errorf("the roll's answer = %v, want passed = false", miss)
	}
	noDC := s.ana.mustRoll(s.campaign, force.GetId(), 20)
	if noDC.Passed != nil {
		t.Errorf("a roll of an action with no DC carries passed = %v", noDC)
	}
	// Caio's tries are his alone, DC shown or not.
	s.caio.mustRoll(s.campaign, invest.GetId(), 3)
	for _, r := range s.ana.getScene(s.campaign).GetRolls() {
		if r.GetCharacterId() != s.pens.GetId() {
			t.Errorf("Ana was sent %v: another player's roll", r)
		}
	}
	// The master always sees the DC, and every pass or fail.
	mview := s.master.getScene(s.campaign)
	if actionByID(mview, invest.GetId()).GetDc() != 15 || len(mview.GetRolls()) != 4 {
		t.Errorf("the master's scene = %v", mview)
	}

	// Each roll remembers whether the scene showed its DC then: the first
	// was made with it hidden, the others with it shown (the summary counts
	// only these).
	var shown []bool
	for _, e := range s.h.sceneEvents() {
		if strings.HasPrefix(e, "scene_check_rolled ") {
			shown = append(shown, strings.Contains(e, `"dc_shown": true`))
		}
	}
	if !slices.Equal(shown, []bool{false, true, true, true}) {
		t.Errorf("rolls made with the DC shown = %v, want [false true true true]", shown)
	}

	// Back off: everything is as before.
	s.mustSetShowDC(false)
	if view = s.ana.getScene(s.campaign); view.GetShowDc() || actionByID(view, invest.GetId()).GetDc() != 0 {
		t.Errorf("Ana's scene after the switch went off = %v, want no DC", view)
	}
	// The pass or fail follows each roll's own moment: the rolls made with the
	// DC shown keep theirs, the first (hidden) never had one.
	for _, r := range view.GetRolls() {
		wantPassed := r.GetActionId() == wis.GetId() // the only one of hers shown with a DC
		if (r.Passed != nil) != wantPassed {
			t.Errorf("Ana's roll of %s has passed = %v after the switch went off", r.GetActionId(), r.Passed)
		}
	}

	// Only a scene has the switch.
	s.mustSetShowDC(true)
	battle := s.master.createPoint(&mapsv1.CreateMapPointRequest{CampaignId: s.campaign, MapId: s.mapID, Kind: mapsv1.MapPointKind_MAP_POINT_KIND_BATTLE, Name: "Emboscada", XBp: 1000, YBp: 1000})
	on := true
	_, err := s.master.updatePoint(&mapsv1.UpdateMapPointRequest{CampaignId: s.campaign, MapId: s.mapID, PointId: battle.GetId(), ShowDc: &on})
	wantCode(t, "show_dc on a battle point", err, connect.CodeInvalidArgument)
	_, err = s.master.maps.CreateMapPoint(t.Context(), connect.NewRequest(&mapsv1.CreateMapPointRequest{
		CampaignId: s.campaign, MapId: s.mapID, Kind: mapsv1.MapPointKind_MAP_POINT_KIND_SUBMAP, Name: "Vale", ShowDc: true,
	}))
	wantCode(t, "creating a submap point with show_dc", err, connect.CodeInvalidArgument)
	created := s.master.createPoint(&mapsv1.CreateMapPointRequest{CampaignId: s.campaign, MapId: s.mapID, Kind: mapsv1.MapPointKind_MAP_POINT_KIND_SCENE, Name: "A ponte", ShowDc: true})
	if !created.GetShowDc() {
		t.Error("a scene point created with show_dc is off")
	}
	// A point that stops being a scene loses the switch.
	kind := mapsv1.MapPointKind_MAP_POINT_KIND_BATTLE
	changed, err := s.master.updatePoint(&mapsv1.UpdateMapPointRequest{CampaignId: s.campaign, MapId: s.mapID, PointId: s.point.GetId(), Kind: &kind})
	if err != nil || changed.GetShowDc() {
		t.Errorf("a scene changed to a battle point = %v, %v; want show_dc cleared", changed, err)
	}
	back := mapsv1.MapPointKind_MAP_POINT_KIND_SCENE
	if changed, err = s.master.updatePoint(&mapsv1.UpdateMapPointRequest{CampaignId: s.campaign, MapId: s.mapID, PointId: s.point.GetId(), Kind: &back}); err != nil || changed.GetShowDc() {
		t.Errorf("the point back to a scene = %v, %v; want the switch still off", changed, err)
	}
}

// TestMR015_AttemptsPerAction: each action has a limit of attempts per
// player: 1 by default, 1 to 5, or unlimited (0). A character that used them
// all is refused with ALREADY_ROLLED; every count is per character; closing and
// opening the scene resets them; lowering the limit below what someone used
// leaves them with none and erases nothing.
func TestMR015_AttemptsPerAction(t *testing.T) {
	t.Parallel()
	s := newScenes(t, true)
	one := s.master.addAction(s.campaign, s.mapID, s.point.GetId(), "skill:investigation", "", 0)
	three := s.addActionWith("skill:perception", 0, 3)
	free := s.addActionWith("ability:str", 0, 0)
	if one.GetMaxAttempts() != 1 || three.GetMaxAttempts() != 3 || free.GetMaxAttempts() != 0 {
		t.Fatalf("limits = %d, %d, %d; want 1 (the default), 3, 0 (unlimited)", one.GetMaxAttempts(), three.GetMaxAttempts(), free.GetMaxAttempts())
	}
	for _, bad := range []int32{-1, 6} {
		_, err := s.master.maps.AddSceneAction(t.Context(), connect.NewRequest(&mapsv1.AddSceneActionRequest{
			CampaignId: s.campaign, MapId: s.mapID, PointId: s.point.GetId(), Key: "ability:dex", MaxAttempts: proto.Int32(bad),
		}))
		wantCode(t, "AddSceneAction with max_attempts out of range", err, connect.CodeInvalidArgument)
		wantCode(t, "UpdateSceneAction with max_attempts out of range", s.setAttempts(one, bad), connect.CodeInvalidArgument)
	}

	s.master.mustOpenScene(s.campaign, s.point.GetId())
	view := s.ana.getScene(s.campaign)
	for _, c := range []struct {
		a         *mapsv1.SceneAction
		max, left int32
	}{{one, 1, 1}, {three, 3, 3}, {free, 0, -1}} {
		if got := actionByID(view, c.a.GetId()); got.GetMaxAttempts() != c.max || left(view, c.a.GetId()) != c.left {
			t.Errorf("Ana's %s = limit %d, left %d; want %d, %d", c.a.GetKey(), got.GetMaxAttempts(), left(view, c.a.GetId()), c.max, c.left)
		}
	}
	if mv := s.master.getScene(s.campaign); actionByID(mv, three.GetId()).GetMaxAttempts() != 3 || actionByID(mv, three.GetId()).AttemptsLeft != nil {
		t.Errorf("the master's action = %v, want the limit and no count of his own", actionByID(mv, three.GetId()))
	}

	// 1: one roll, then none.
	s.ana.mustRoll(s.campaign, one.GetId(), 5)
	_, err := s.ana.rollWith(s.campaign, one.GetId(), 6, newKey())
	wantSceneBlocked(t, "the second roll of a 1-attempt action", err, playv1.SceneBlockedReason_SCENE_BLOCKED_REASON_ALREADY_ROLLED)
	// 3: three rolls, counting down; Caio's count is his own.
	for i, wantLeft := range []int32{2, 1, 0} {
		s.ana.mustRoll(s.campaign, three.GetId(), int32(i)+5)
		if got := left(s.ana.getScene(s.campaign), three.GetId()); got != wantLeft {
			t.Errorf("Ana's attempts left after roll %d = %d, want %d", i+1, got, wantLeft)
		}
	}
	_, err = s.ana.rollWith(s.campaign, three.GetId(), 9, newKey())
	wantSceneBlocked(t, "the fourth roll of a 3-attempt action", err, playv1.SceneBlockedReason_SCENE_BLOCKED_REASON_ALREADY_ROLLED)
	if got := left(s.caio.getScene(s.campaign), three.GetId()); got != 3 {
		t.Errorf("Caio's attempts left = %d, want 3: counts are per character", got)
	}
	// Unlimited: as many as the player wants, and never a count.
	for range 6 {
		s.ana.mustRoll(s.campaign, free.GetId(), 7)
	}
	if got := left(s.ana.getScene(s.campaign), free.GetId()); got != -1 {
		t.Errorf("an unlimited action's count = %d, want unset", got)
	}
	// The master reads who is out of attempts on each roll card.
	for _, r := range s.master.getScene(s.campaign).GetRolls() {
		want := map[string]int32{one.GetId(): 0, three.GetId(): 0, free.GetId(): -1}[r.GetActionId()]
		got := int32(-1)
		if r.AttemptsLeft != nil {
			got = r.GetAttemptsLeft()
		}
		if got != want {
			t.Errorf("the master's roll of %s says %d attempts left, want %d", r.GetActionId(), got, want)
		}
	}
	for _, r := range s.ana.getScene(s.campaign).GetRolls() {
		if r.AttemptsLeft != nil {
			t.Errorf("Ana's roll carries attempts_left = %d: only the master gets it", r.GetAttemptsLeft())
		}
	}

	// Lowering the limit below what she used: nothing is erased, she is out;
	// raising it gives attempts back.
	rollsBefore := len(s.master.getScene(s.campaign).GetRolls())
	if err := s.setAttempts(three, 2); err != nil {
		t.Fatal(err)
	}
	view = s.ana.getScene(s.campaign)
	if got := left(view, three.GetId()); got != 0 {
		t.Errorf("attempts left after lowering the limit below the used = %d, want 0", got)
	}
	if got := len(s.master.getScene(s.campaign).GetRolls()); got != rollsBefore {
		t.Errorf("the log has %d rolls after lowering the limit, want %d: nothing is erased", got, rollsBefore)
	}
	_, err = s.ana.rollWith(s.campaign, three.GetId(), 9, newKey())
	wantSceneBlocked(t, "rolling after the limit was lowered", err, playv1.SceneBlockedReason_SCENE_BLOCKED_REASON_ALREADY_ROLLED)
	if err := s.setAttempts(three, 5); err != nil {
		t.Fatal(err)
	}
	if got := left(s.ana.getScene(s.campaign), three.GetId()); got != 2 {
		t.Errorf("attempts left after raising the limit to 5 = %d, want 2 (5 - 3 used)", got)
	}
	if err := s.setAttempts(free, 1); err != nil {
		t.Fatal(err)
	}
	if got := left(s.ana.getScene(s.campaign), free.GetId()); got != 0 {
		t.Errorf("an unlimited action limited to 1 after 6 rolls has %d left, want 0", got)
	}

	// Closing the scene and opening it again resets every count.
	s.master.closeScene(s.campaign)
	s.master.mustOpenScene(s.campaign, s.point.GetId())
	view = s.ana.getScene(s.campaign)
	for _, c := range []struct {
		a    *mapsv1.SceneAction
		left int32
	}{{one, 1}, {three, 5}, {free, 1}} {
		if got := left(view, c.a.GetId()); got != c.left {
			t.Errorf("after reopening, attempts left of %s = %d, want %d", c.a.GetKey(), got, c.left)
		}
	}
	if _, err := s.ana.rollWith(s.campaign, one.GetId(), 4, newKey()); err != nil {
		t.Errorf("rolling after reopening error = %v", err)
	}

	// A player cannot set a limit.
	_, err = s.update(s.ana, one, func(r *mapsv1.UpdateSceneActionRequest) { r.MaxAttempts = proto.Int32(5) })
	wantCode(t, "a player setting max_attempts", err, connect.CodePermissionDenied)
}

// TestMR015_OneMoreAttempt: the master gives one character one more attempt at
// one action of the open scene ("Dar mais uma tentativa"). It lasts for this
// opening, is idempotent by key, reaches the players only as the content-free
// scene hint, and is a history event with ids only.
func TestMR015_OneMoreAttempt(t *testing.T) {
	t.Parallel()
	s := newScenes(t, true)
	invest := s.master.addAction(s.campaign, s.mapID, s.point.GetId(), "skill:investigation", "Procurar pistas", 12)
	free := s.addActionWith("ability:str", 0, 0)
	other := s.master.addAction(s.campaign, s.mapID, s.point.GetId(), "skill:perception", "", 0)
	_, err := s.master.grantAttempt(s.campaign, invest.GetId(), s.pens.GetId(), newKey())
	wantSceneBlocked(t, "granting with no scene open", err, playv1.SceneBlockedReason_SCENE_BLOCKED_REASON_NO_OPEN_SCENE)
	s.master.mustOpenScene(s.campaign, s.point.GetId())
	anaWatch, caioWatch := s.ana.watch(s.campaign), s.caio.watch(s.campaign)

	s.ana.mustRoll(s.campaign, invest.GetId(), 2) // 2 + 6 = 8: fails
	_, err = s.ana.rollWith(s.campaign, invest.GetId(), 20, newKey())
	wantSceneBlocked(t, "rolling again with none left", err, playv1.SceneBlockedReason_SCENE_BLOCKED_REASON_ALREADY_ROLLED)

	key := newKey()
	scene, err := s.master.grantAttempt(s.campaign, invest.GetId(), s.pens.GetId(), key)
	if err != nil {
		t.Fatalf("GrantSceneAttempt() error = %v", err)
	}
	if actionByID(scene, invest.GetId()).GetDc() != 12 {
		t.Errorf("the answer is not the open scene as the master sees it: %v", scene)
	}
	// Ana is told something changed, and nothing else; Caio too.
	for name, w := range map[string]*watcher{"Ana": anaWatch, "Caio": caioWatch} {
		ev := w.nextSceneChanged()
		if strings.Contains(asJSON(t, ev), s.pens.GetId()) {
			t.Errorf("%s's event = %v, want the content-free scene_changed", name, ev)
		}
	}
	if got := left(s.ana.getScene(s.campaign), invest.GetId()); got != 1 {
		t.Errorf("Ana's attempts left after the grant = %d, want 1", got)
	}
	if got := left(s.caio.getScene(s.campaign), invest.GetId()); got != 1 {
		t.Errorf("Caio's attempts left = %d, want 1: the grant was Pensantus's", got)
	}
	// A retry with the same key adds nothing.
	if _, err := s.master.grantAttempt(s.campaign, invest.GetId(), s.pens.GetId(), key); err != nil {
		t.Fatalf("the retry error = %v", err)
	}
	if n := s.count("scene_attempt_granted"); n != 1 {
		t.Errorf("%d scene_attempt_granted events after the retry, want 1", n)
	}
	if got := left(s.ana.getScene(s.campaign), invest.GetId()); got != 1 {
		t.Errorf("Ana's attempts left after the retry = %d, want 1", got)
	}
	// She rolls it, and is out again; a second grant is another attempt.
	if _, err := s.ana.rollWith(s.campaign, invest.GetId(), 10, newKey()); err != nil {
		t.Fatalf("the granted roll error = %v", err)
	}
	if got := left(s.ana.getScene(s.campaign), invest.GetId()); got != 0 {
		t.Errorf("Ana's attempts left after using it = %d, want 0", got)
	}
	s.master.mustGrantAttempt(s.campaign, invest.GetId(), s.pens.GetId())
	s.master.mustGrantAttempt(s.campaign, invest.GetId(), s.pens.GetId())
	if got := left(s.ana.getScene(s.campaign), invest.GetId()); got != 2 {
		t.Errorf("Ana's attempts left after two more grants = %d, want 2", got)
	}
	if got := left(s.ana.getScene(s.campaign), other.GetId()); got != 1 {
		t.Errorf("Ana's attempts left at another action = %d, want 1: a grant is for one action", got)
	}

	// An unlimited action has nothing to add: no event.
	before := s.count("scene_attempt_granted")
	s.master.mustGrantAttempt(s.campaign, free.GetId(), s.pens.GetId())
	if n := s.count("scene_attempt_granted"); n != before {
		t.Errorf("granting at an unlimited action wrote an event (%d, was %d)", n, before)
	}

	// The history holds ids only.
	for _, e := range s.h.sceneEvents() {
		if !strings.HasPrefix(e, "scene_attempt_granted ") {
			continue
		}
		for _, banned := range []string{"Pensantus", "Procurar pistas", "carroça", "\"dc\""} {
			if strings.Contains(e, banned) {
				t.Errorf("the history %q holds %q", e, banned)
			}
		}
	}

	// A grant lasts for this opening: reopening the scene resets it.
	s.master.closeScene(s.campaign)
	s.master.mustOpenScene(s.campaign, s.point.GetId())
	if got := left(s.ana.getScene(s.campaign), invest.GetId()); got != 1 {
		t.Errorf("Ana's attempts left after reopening = %d, want 1 (the grants went with the old opening)", got)
	}

	// What is refused.
	npc := s.master.createNPC(s.campaign, "Guarda", "")
	for _, c := range []struct {
		name         string
		action, char string
		key          string
		code         connect.Code
	}{
		{"an action that is not in the scene", newKey(), s.pens.GetId(), newKey(), connect.CodeNotFound},
		{"an action that is not a UUID", "x", s.pens.GetId(), newKey(), connect.CodeNotFound},
		{"an NPC", invest.GetId(), npc.GetId(), newKey(), connect.CodeNotFound},
		{"a character that is not a UUID", invest.GetId(), "x", newKey(), connect.CodeNotFound},
		{"a key that is not a UUID", invest.GetId(), s.pens.GetId(), "x", connect.CodeInvalidArgument},
	} {
		_, err := s.master.grantAttempt(s.campaign, c.action, c.char, c.key)
		wantCode(t, "granting for "+c.name, err, c.code)
	}
	rollKey := newKey()
	if _, err := s.caio.rollWith(s.campaign, other.GetId(), 5, rollKey); err != nil {
		t.Fatal(err)
	}
	_, err = s.master.grantAttempt(s.campaign, invest.GetId(), s.pens.GetId(), rollKey)
	wantCode(t, "granting with the key of a roll", err, connect.CodeInvalidArgument)
	_, err = s.ana.grantAttempt(s.campaign, invest.GetId(), s.pens.GetId(), newKey())
	wantCode(t, "a player granting", err, connect.CodePermissionDenied)
}

// TestRN20_SessionSummaryHidesWhatTheSceneHid: the session summary counts the
// checks passed outside combat only from rolls made while their scene showed
// its DC, and only for actions with one. A scene that hid its DC gives the
// summary nothing, for anyone; a player never reads another player's tries.
func TestRN20_SessionSummaryHidesWhatTheSceneHid(t *testing.T) {
	t.Parallel()
	s := newScenes(t, true)
	invest, force, _ := s.setup() // DC 15, none, 12
	session := s.currentSession()

	// Scene one hides its DC: both players pass an Investigação at DC 15 and
	// it counts for nobody.
	s.master.mustOpenScene(s.campaign, s.point.GetId())
	s.ana.mustRoll(s.campaign, invest.GetId(), 14)  // 14 + 6 = 20 passes
	s.caio.mustRoll(s.campaign, invest.GetId(), 20) // 20 + 4 = 24 passes
	s.master.closeScene(s.campaign)

	// Scene two shows it: Ana passes, Toren misses, and a check with no DC is
	// no test at all.
	s.mustSetShowDC(true)
	s.master.mustOpenScene(s.campaign, s.point.GetId())
	s.ana.mustRoll(s.campaign, invest.GetId(), 14)
	s.caio.mustRoll(s.campaign, invest.GetId(), 2) // 2 + 4 = 6 misses
	s.ana.mustRoll(s.campaign, force.GetId(), 20)
	s.master.closeScene(s.campaign)
	// The master hides it again: the rolls made while it was shown still
	// count, the ones made after never do.
	s.mustSetShowDC(false)
	s.master.mustOpenScene(s.campaign, s.point.GetId())
	s.caio.mustRoll(s.campaign, invest.GetId(), 20)
	s.master.closeScene(s.campaign)

	_, err := s.master.summary(s.campaign, session.GetId())
	wantSessionNotEnded(t, "the summary of an open session", err)
	s.master.end(s.campaign, session)

	var seen []string // everything the players were sent, as JSON
	for name, u := range map[string]*user{"Ana": s.ana, "Caio": s.caio} {
		sum, err := u.summary(s.campaign, session.GetId())
		if err != nil {
			t.Fatalf("%s's summary error = %v", name, err)
		}
		seen = append(seen, protojson.Format(sum))
		mine := sum.GetMine()
		wantPassed, wantTried := int32(1), int32(1) // Ana: 1 of 1; Caio: 0 of 1
		if name == "Caio" {
			wantPassed = 0
		}
		if mine.GetChecksPassed() != wantPassed || mine.GetChecksTried() != wantTried {
			t.Errorf("%s's own result = %d of %d, want %d of %d", name, mine.GetChecksPassed(), mine.GetChecksTried(), wantPassed, wantTried)
		}
		if len(sum.GetPlayers()) != 0 || sum.GetChecksTried() != 0 || sum.GetChecksPassed() != 0 || sum.GetScenesOpened() != 0 || sum.GetCombats() != 0 {
			t.Errorf("%s got the master's numbers: %v", name, sum)
		}
	}
	// The winner of "Mais testes passados": Ana, with 1; Pensantus's name is
	// public, Toren's number is not in her summary.
	anaSum, _ := s.ana.summary(s.campaign, session.GetId())
	if cats := anaSum.GetCategories(); len(cats) != 1 || cats[0].GetKind() != playv1.HighlightKind_HIGHLIGHT_KIND_CHECKS_PASSED ||
		cats[0].GetValue() != 1 || len(cats[0].GetWinners()) != 1 || cats[0].GetWinners()[0].GetCharacterId() != s.pens.GetId() {
		t.Errorf("the categories = %v, want only Pensantus with 1 passed", cats)
	}
	// Neither player reads the table, a total or the other's tries: Ana's
	// summary never names Toren, who won nothing.
	for _, text := range seen {
		if strings.Contains(text, "\"players\"") || strings.Contains(text, "\"checksTried\": 2") {
			t.Errorf("a player's summary has more than the winners and their own result: %s", text)
		}
	}
	if text := asJSON(t, anaSum); strings.Contains(text, "Toren") || strings.Contains(text, s.other.GetId()) {
		t.Errorf("Ana's summary names Toren, who won nothing: %s", text)
	}

	// The master: every number, and the table. Scenes: 3 opened; tests: Ana 1
	// of 1, Toren 0 of 1 (the rolls of the two scenes that hid the DC are in
	// neither number).
	sum, err := s.master.summary(s.campaign, session.GetId())
	if err != nil {
		t.Fatal(err)
	}
	if sum.GetScenesOpened() != 3 || sum.GetChecksPassed() != 1 || sum.GetChecksTried() != 2 || sum.GetMine() != nil {
		t.Errorf("the master's summary = %v, want 3 scenes and 1 of 2 tests", sum)
	}
	got := map[string][2]int32{}
	for _, p := range sum.GetPlayers() {
		got[p.GetHighlights().GetName()] = [2]int32{p.GetChecksPassed(), p.GetChecksTried()}
	}
	if len(got) != 2 || got["Pensantus"] != [2]int32{1, 1} || got["Toren"] != [2]int32{0, 1} {
		t.Errorf("the master's table = %v, want Pensantus 1 of 1, Toren 0 of 1", got)
	}
}

func (u *user) summary(campaignID, sessionID string) (*playv1.SessionSummary, error) {
	res, err := u.play.GetSessionSummary(u.h.t.Context(), connect.NewRequest(&playv1.GetSessionSummaryRequest{CampaignId: campaignID, GameSessionId: sessionID}))
	if err != nil {
		return nil, err
	}
	return res.Msg.GetSummary(), nil
}

// end ends the session as u.
func (u *user) end(campaignID string, session *playv1.GameSession) {
	u.h.t.Helper()
	if _, err := u.play.EndGameSession(u.h.t.Context(), connect.NewRequest(&playv1.EndGameSessionRequest{CampaignId: campaignID, GameSessionId: session.GetId()})); err != nil {
		u.h.t.Fatalf("EndGameSession() error = %v", err)
	}
}

// currentSession is the campaign's open session.
func (s *scenes) currentSession() *playv1.GameSession {
	s.h.t.Helper()
	res, err := s.master.play.ListGameSessions(s.h.t.Context(), connect.NewRequest(&playv1.ListGameSessionsRequest{CampaignId: s.campaign}))
	if err != nil || len(res.Msg.GetGameSessions()) == 0 {
		s.h.t.Fatalf("ListGameSessions() = %v, %v", res, err)
	}
	return res.Msg.GetGameSessions()[0]
}

// wantSessionNotEnded checks the refusal of a summary of an open session.
func wantSessionNotEnded(t *testing.T, call string, err error) {
	t.Helper()
	wantCode(t, call, err, connect.CodeFailedPrecondition)
	ce, _ := errors.AsType[*connect.Error](err)
	for _, d := range ce.Details() {
		if msg, derr := d.Value(); derr == nil {
			if b, ok := msg.(*playv1.GameSessionBlocked); ok && b.GetReason() == playv1.GameSessionBlockedReason_GAME_SESSION_BLOCKED_REASON_SESSION_NOT_ENDED {
				return
			}
		}
	}
	t.Errorf("%s error = %v, want a GameSessionBlocked detail with SESSION_NOT_ENDED", call, err)
}

// A retry of a roll with the same key after the limit is spent returns the
// stored roll, not ALREADY_ROLLED; and two rolls of the same action at once
// with different keys, with one attempt, let exactly one through.
func TestMR015_ReplayAndRaceOnTheLastAttempt(t *testing.T) {
	t.Parallel()
	s := newScenes(t, true)
	one := s.master.addAction(s.campaign, s.mapID, s.point.GetId(), "skill:investigation", "", 0)
	s.master.mustOpenScene(s.campaign, s.point.GetId())

	key := newKey()
	first, err := s.ana.rollWith(s.campaign, one.GetId(), 9, key)
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.ana.rollWith(s.campaign, one.GetId(), 9, key)
	if err != nil || !proto.Equal(first, again) {
		t.Errorf("the replay after the limit = %v, %v; want the stored roll", again, err)
	}

	// Caio, one attempt, two calls at once.
	errs := make(chan error, 2)
	for range 2 {
		go func() {
			_, err := s.caio.rollWith(s.campaign, one.GetId(), 8, newKey())
			errs <- err
		}()
	}
	ok := 0
	for range 2 {
		if err := <-errs; err == nil {
			ok++
		} else if connect.CodeOf(err) != connect.CodeFailedPrecondition {
			t.Errorf("the losing roll error = %v, want failed_precondition", err)
		}
	}
	if ok != 1 {
		t.Errorf("%d of two simultaneous rolls with one attempt succeeded, want 1", ok)
	}
}

// The key of a grant belongs to that grant: reusing it for another character
// or action is the key of another change.
func TestMR015_GrantKeyBelongsToItsGrant(t *testing.T) {
	t.Parallel()
	s := newScenes(t, true)
	a := s.master.addAction(s.campaign, s.mapID, s.point.GetId(), "skill:investigation", "", 0)
	b := s.master.addAction(s.campaign, s.mapID, s.point.GetId(), "skill:perception", "", 0)
	s.master.mustOpenScene(s.campaign, s.point.GetId())
	key := newKey()
	if _, err := s.master.grantAttempt(s.campaign, a.GetId(), s.pens.GetId(), key); err != nil {
		t.Fatal(err)
	}
	_, err := s.master.grantAttempt(s.campaign, a.GetId(), s.other.GetId(), key)
	wantCode(t, "the key with another character", err, connect.CodeInvalidArgument)
	_, err = s.master.grantAttempt(s.campaign, b.GetId(), s.pens.GetId(), key)
	wantCode(t, "the key with another action", err, connect.CodeInvalidArgument)
	if _, err := s.master.grantAttempt(s.campaign, a.GetId(), s.pens.GetId(), key); err != nil {
		t.Errorf("the same grant again error = %v", err)
	}
	// A grant may push the count past the limit, on purpose.
	s.master.mustGrantAttempt(s.campaign, a.GetId(), s.pens.GetId())
	if got := left(s.ana.getScene(s.campaign), a.GetId()); got != 3 {
		t.Errorf("attempts left = %d, want 3 (1 + 2 grants, above the limit of 1)", got)
	}
}

// A character whose death the master recorded during the session stays in its
// summary, named, with its checks, and its player still reads their own result,
// summed with the character that replaced it.
func TestMR032_SummaryKeepsTheDeadAndSumsReplacements(t *testing.T) {
	t.Parallel()
	s := newScenes(t, true)
	invest, _, _ := s.setup()
	s.mustSetShowDC(true)
	session := s.currentSession()
	s.master.mustOpenScene(s.campaign, s.point.GetId())
	s.caio.mustRoll(s.campaign, invest.GetId(), 20) // Toren passes
	if _, err := s.master.characters.MarkCharacterDead(t.Context(), connect.NewRequest(&charactersv1.MarkCharacterDeadRequest{CampaignId: s.campaign, CharacterId: s.other.GetId()})); err != nil {
		t.Fatalf("MarkCharacterDead() error = %v", err)
	}
	second := s.caio.createCharacter(s.campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Toren Segundo")
	s.caio.mustRoll(s.campaign, invest.GetId(), 2) // misses
	s.master.closeScene(s.campaign)
	s.master.end(s.campaign, session)

	mine := mustSummary(t, s.caio, s.campaign, session.GetId()).GetMine()
	if mine.GetChecksPassed() != 1 || mine.GetChecksTried() != 2 {
		t.Errorf("Caio's own result = %d of %d, want 1 of 2 across both characters", mine.GetChecksPassed(), mine.GetChecksTried())
	}
	got := map[string][2]int32{}
	for _, p := range mustSummary(t, s.master, s.campaign, session.GetId()).GetPlayers() {
		if p.GetHighlights().GetName() == "" {
			t.Errorf("the master's table has a nameless row: %v", p)
		}
		got[p.GetHighlights().GetName()] = [2]int32{p.GetChecksPassed(), p.GetChecksTried()}
	}
	if got["Toren"] != [2]int32{1, 1} || got[second.GetName()] != [2]int32{0, 1} {
		t.Errorf("the master's table = %v, want the dead Toren 1 of 1 and his replacement 0 of 1", got)
	}
	for _, c := range mustSummary(t, s.ana, s.campaign, session.GetId()).GetCategories() {
		for _, w := range c.GetWinners() {
			if w.GetName() == "" {
				t.Errorf("a winner has no name: %v", c)
			}
		}
	}
}

func mustSummary(t *testing.T, u *user, campaignID, sessionID string) *playv1.SessionSummary {
	t.Helper()
	sum, err := u.summary(campaignID, sessionID)
	if err != nil {
		t.Fatalf("GetSessionSummary() error = %v", err)
	}
	return sum
}
