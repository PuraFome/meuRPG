package maps

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
)

// Characterization of the calls that edit a scene point's clues and actions
// (AddSceneClue, MoveSceneClue, RevealSceneClue, AddSceneAction,
// UpdateSceneAction, MoveSceneAction): each refusal with its exact code and
// message and the order the checks run in, what each success answers, and what
// the master's, a chosen player's and another player's streams hear. They were
// written on the code before it was split into phases, and must keep passing
// unchanged.

// sceneRefusal renders a refusal as "code: message", or "ok" for no error.
func sceneRefusal(err error) string {
	if err == nil {
		return "ok"
	}
	var ce *connect.Error
	if !errors.As(err, &ce) {
		return "not a connect error: " + err.Error()
	}
	return ce.Code().String() + ": " + ce.Message()
}

// sceneEditTable is the scene of these tests plus the calls under test, each
// as a function of the raw request fields so a case changes one field only.
type sceneEditTable struct {
	*scenes
	pid, clueID, actionID string
}

func newSceneEditTable(t *testing.T) *sceneEditTable {
	t.Helper()
	s := newScenes(t, true)
	pid := s.point.GetId()
	return &sceneEditTable{
		scenes: s, pid: pid,
		clueID:   s.master.addClue(s.campaign, s.mapID, pid, "Uma pista").GetId(),
		actionID: s.master.addAction(s.campaign, s.mapID, pid, "skill:arcana", "Ler a runa", 10).GetId(),
	}
}

func (e *sceneEditTable) addClue(u *user, mapID, pointID, text, key string) error {
	_, err := u.maps.AddSceneClue(e.h.t.Context(), connect.NewRequest(&mapsv1.AddSceneClueRequest{
		CampaignId: e.campaign, MapId: mapID, PointId: pointID, Text: text, IdempotencyKey: key,
	}))
	return err
}

func (e *sceneEditTable) moveClue(u *user, clueID string, dir mapsv1.SceneActionDirection) error {
	_, err := u.maps.MoveSceneClue(e.h.t.Context(), connect.NewRequest(&mapsv1.MoveSceneClueRequest{
		CampaignId: e.campaign, MapId: e.mapID, PointId: e.pid, ClueId: clueID, Direction: dir,
	}))
	return err
}

func (e *sceneEditTable) moveClueIn(mapID, pointID string) error {
	_, err := e.master.maps.MoveSceneClue(e.h.t.Context(), connect.NewRequest(&mapsv1.MoveSceneClueRequest{
		CampaignId: e.campaign, MapId: mapID, PointId: pointID, ClueId: e.clueID, Direction: mapsv1.SceneActionDirection_SCENE_ACTION_DIRECTION_UP,
	}))
	return err
}

func (e *sceneEditTable) reveal(u *user, clueID string, ids ...string) error {
	_, err := u.revealClue(e.campaign, clueID, ids...)
	return err
}

func (e *sceneEditTable) addAction(u *user, mapID, pointID string, edit func(*mapsv1.AddSceneActionRequest)) error {
	req := &mapsv1.AddSceneActionRequest{CampaignId: e.campaign, MapId: mapID, PointId: pointID, Key: "skill:stealth"}
	edit(req)
	_, err := u.maps.AddSceneAction(e.h.t.Context(), connect.NewRequest(req))
	return err
}

func (e *sceneEditTable) updateAction(u *user, actionID string, edit func(*mapsv1.UpdateSceneActionRequest)) error {
	req := &mapsv1.UpdateSceneActionRequest{CampaignId: e.campaign, MapId: e.mapID, PointId: e.pid, ActionId: actionID}
	edit(req)
	_, err := u.maps.UpdateSceneAction(e.h.t.Context(), connect.NewRequest(req))
	return err
}

func (e *sceneEditTable) moveAction(actionID string, dir mapsv1.SceneActionDirection) error {
	_, err := e.master.maps.MoveSceneAction(e.h.t.Context(), connect.NewRequest(&mapsv1.MoveSceneActionRequest{
		CampaignId: e.campaign, MapId: e.mapID, PointId: e.pid, ActionId: actionID, Direction: dir,
	}))
	return err
}

func TestSceneEditRefusalsAreFrozen(t *testing.T) {
	t.Parallel()
	e := newSceneEditTable(t)
	up := mapsv1.SceneActionDirection_SCENE_ACTION_DIRECTION_UP
	unset := mapsv1.SceneActionDirection_SCENE_ACTION_DIRECTION_UNSPECIFIED
	battle := e.master.createPoint(&mapsv1.CreateMapPointRequest{CampaignId: e.campaign, MapId: e.mapID, Kind: mapsv1.MapPointKind_MAP_POINT_KIND_BATTLE, Name: "Emboscada"})
	otherMap := e.master.createMap(e.campaign, "Outro mapa", e.master.newImage(e.campaign))
	long := strings.Repeat("a", maxClueText+1)

	cases := []struct {
		name string
		call func() error
		want string
	}{
		// AddSceneClue: the call's own checks, in order.
		{"add clue: a player", func() error { return e.addClue(e.ana, e.mapID, e.pid, "x", "") }, "permission_denied: only the campaign's master can do this"},
		{"add clue: map not a UUID", func() error { return e.addClue(e.master, "x", e.pid, "x", "") }, "not_found: map not found"},
		{"add clue: point not a UUID", func() error { return e.addClue(e.master, e.mapID, "x", "x", "") }, "not_found: point not found"},
		{"add clue: bad point wins over bad text", func() error { return e.addClue(e.master, e.mapID, "x", "", "") }, "not_found: point not found"},
		{"add clue: empty text", func() error { return e.addClue(e.master, e.mapID, e.pid, "  ", "") }, "invalid_argument: text must not be empty"},
		{"add clue: text too long", func() error { return e.addClue(e.master, e.mapID, e.pid, long, "") }, "invalid_argument: text must be at most 500 characters, got 501"},
		{"add clue: bad text wins over bad key", func() error { return e.addClue(e.master, e.mapID, e.pid, "", strings.Repeat("k", 65)) }, "invalid_argument: text must not be empty"},
		{"add clue: bad key", func() error { return e.addClue(e.master, e.mapID, e.pid, "x", strings.Repeat("k", 65)) }, "invalid_argument: idempotency_key must be 1 to 64 characters"},
		{"add clue: unknown map", func() error { return e.addClue(e.master, newKey(), e.pid, "x", "") }, "not_found: map not found"},
		{"add clue: unknown point", func() error { return e.addClue(e.master, e.mapID, newKey(), "x", "") }, "not_found: point not found"},
		{"add clue: point of another map", func() error { return e.addClue(e.master, otherMap.GetId(), e.pid, "x", "") }, "not_found: point not found"},
		{"add clue: not a scene point", func() error { return e.addClue(e.master, e.mapID, battle.GetId(), "x", "") }, "invalid_argument: only a SCENE point has actions and clues"},

		// UpdateSceneClue, RemoveSceneClue (untouched by the refactor, kept as the baseline of the shared parts).
		{"move clue: a player", func() error { return e.moveClue(e.ana, e.clueID, up) }, "permission_denied: only the campaign's master can do this"},
		{"move clue: bad point", func() error { return e.moveClueIn(e.mapID, "x") }, "not_found: point not found"},
		{"move clue: id not a UUID", func() error { return e.moveClue(e.master, "x", up) }, "not_found: clue not found"},
		{"move clue: no direction", func() error { return e.moveClue(e.master, e.clueID, unset) }, "invalid_argument: direction is required"},
		{"move clue: bad id wins over no direction", func() error { return e.moveClue(e.master, "x", unset) }, "not_found: clue not found"},
		{"move clue: direction wins over not a scene point", func() error {
			_, err := e.master.maps.MoveSceneClue(t.Context(), connect.NewRequest(&mapsv1.MoveSceneClueRequest{
				CampaignId: e.campaign, MapId: e.mapID, PointId: battle.GetId(), ClueId: e.clueID,
			}))
			return err
		}, "invalid_argument: direction is required"},
		{"move clue: not a scene point", func() error {
			_, err := e.master.maps.MoveSceneClue(t.Context(), connect.NewRequest(&mapsv1.MoveSceneClueRequest{
				CampaignId: e.campaign, MapId: e.mapID, PointId: battle.GetId(), ClueId: e.clueID, Direction: up,
			}))
			return err
		}, "invalid_argument: only a SCENE point has actions and clues"},
		{"move clue: unknown clue", func() error { return e.moveClue(e.master, newKey(), up) }, "not_found: clue not found"},

		// RevealSceneClue: master, clue id, count, each id, the characters, the clue.
		{"reveal: a player", func() error { return e.reveal(e.ana, e.clueID, e.pens.GetId()) }, "permission_denied: only the campaign's master can do this"},
		{"reveal: clue id not a UUID", func() error { return e.reveal(e.master, "x", e.pens.GetId()) }, "not_found: clue not found"},
		{"reveal: bad clue id wins over no characters", func() error { return e.reveal(e.master, "x") }, "not_found: clue not found"},
		{"reveal: no characters", func() error { return e.reveal(e.master, e.clueID) }, "invalid_argument: character_ids must have 1 to 50 characters"},
		{"reveal: 51 characters", func() error { return e.reveal(e.master, e.clueID, make([]string, 51)...) }, "invalid_argument: character_ids must have 1 to 50 characters"},
		{"reveal: a bad id after a good one", func() error { return e.reveal(e.master, e.clueID, e.pens.GetId(), "x") }, "not_found: character not found"},
		{"reveal: bad character wins over unknown clue", func() error { return e.reveal(e.master, newKey(), "x") }, "not_found: character not found"},
		{"reveal: a stranger", func() error { return e.reveal(e.master, e.clueID, newKey()) }, "not_found: character not found"},
		{"reveal: a good one and a stranger", func() error { return e.reveal(e.master, e.clueID, e.pens.GetId(), newKey()) }, "not_found: character not found"},
		{"reveal: unknown clue", func() error { return e.reveal(e.master, newKey(), e.pens.GetId()) }, "not_found: clue not found"},

		// AddSceneAction: master and point, key, name, dc, attempts, idempotency key.
		{"add action: a player", func() error { return e.addAction(e.ana, e.mapID, e.pid, func(*mapsv1.AddSceneActionRequest) {}) }, "permission_denied: only the campaign's master can do this"},
		{"add action: bad point", func() error {
			return e.addAction(e.master, e.mapID, "x", func(r *mapsv1.AddSceneActionRequest) { r.Key = "nope" })
		}, "not_found: point not found"},
		{"add action: bad key", func() error {
			return e.addAction(e.master, e.mapID, e.pid, func(r *mapsv1.AddSceneActionRequest) { r.Key = "attack:sword" })
		}, "invalid_argument: key must be a skill, an ability check or a saving throw (skill:investigation, ability:str, save:wis)"},
		{"add action: bad key wins over bad name", func() error {
			return e.addAction(e.master, e.mapID, e.pid, func(r *mapsv1.AddSceneActionRequest) { r.Key = "x"; r.Name = strings.Repeat("a", 61) })
		}, "invalid_argument: key must be a skill, an ability check or a saving throw (skill:investigation, ability:str, save:wis)"},
		{"add action: name too long", func() error {
			return e.addAction(e.master, e.mapID, e.pid, func(r *mapsv1.AddSceneActionRequest) { r.Name = strings.Repeat("a", 61); r.Dc = 99 })
		}, "invalid_argument: name must be at most 60 characters, got 61"},
		{"add action: dc 31", func() error {
			return e.addAction(e.master, e.mapID, e.pid, func(r *mapsv1.AddSceneActionRequest) { r.Dc = 31; r.MaxAttempts = proto.Int32(9) })
		}, "invalid_argument: dc must be 1 to 30, or 0 for none"},
		{"add action: attempts 6", func() error {
			return e.addAction(e.master, e.mapID, e.pid, func(r *mapsv1.AddSceneActionRequest) {
				r.MaxAttempts = proto.Int32(6)
				r.IdempotencyKey = strings.Repeat("k", 65)
			})
		}, "invalid_argument: max_attempts must be 1 to 5, or 0 for unlimited"},
		{"add action: negative attempts", func() error {
			return e.addAction(e.master, e.mapID, e.pid, func(r *mapsv1.AddSceneActionRequest) { r.MaxAttempts = proto.Int32(-1) })
		}, "invalid_argument: max_attempts must be 1 to 5, or 0 for unlimited"},
		{"add action: bad idempotency key", func() error {
			return e.addAction(e.master, e.mapID, e.pid, func(r *mapsv1.AddSceneActionRequest) { r.IdempotencyKey = strings.Repeat("k", 65) })
		}, "invalid_argument: idempotency_key must be 1 to 64 characters"},
		{"add action: not a scene point", func() error {
			return e.addAction(e.master, e.mapID, battle.GetId(), func(*mapsv1.AddSceneActionRequest) {})
		}, "invalid_argument: only a SCENE point has actions and clues"},

		// UpdateSceneAction: master and point, action id, nothing to change, then key, name, dc, attempts.
		{"update action: a player", func() error {
			return e.updateAction(e.ana, e.actionID, func(r *mapsv1.UpdateSceneActionRequest) { r.Dc = proto.Int32(5) })
		}, "permission_denied: only the campaign's master can do this"},
		{"update action: id not a UUID", func() error { return e.updateAction(e.master, "x", func(*mapsv1.UpdateSceneActionRequest) {}) }, "not_found: action not found"},
		{"update action: nothing to change", func() error { return e.updateAction(e.master, e.actionID, func(*mapsv1.UpdateSceneActionRequest) {}) }, "invalid_argument: nothing to change"},
		{"update action: bad key wins over the rest", func() error {
			return e.updateAction(e.master, e.actionID, func(r *mapsv1.UpdateSceneActionRequest) {
				r.Key = new("x")
				r.Name = new(strings.Repeat("a", 61))
				r.Dc = proto.Int32(99)
				r.MaxAttempts = proto.Int32(9)
			})
		}, "invalid_argument: key must be a skill, an ability check or a saving throw (skill:investigation, ability:str, save:wis)"},
		{"update action: bad name wins over dc", func() error {
			return e.updateAction(e.master, e.actionID, func(r *mapsv1.UpdateSceneActionRequest) {
				r.Name = new(strings.Repeat("a", 61))
				r.Dc = proto.Int32(99)
				r.MaxAttempts = proto.Int32(9)
			})
		}, "invalid_argument: name must be at most 60 characters, got 61"},
		{"update action: bad dc wins over attempts", func() error {
			return e.updateAction(e.master, e.actionID, func(r *mapsv1.UpdateSceneActionRequest) { r.Dc = proto.Int32(-3); r.MaxAttempts = proto.Int32(9) })
		}, "invalid_argument: dc must be 1 to 30, or 0 for none"},
		{"update action: bad attempts", func() error {
			return e.updateAction(e.master, e.actionID, func(r *mapsv1.UpdateSceneActionRequest) { r.MaxAttempts = proto.Int32(6) })
		}, "invalid_argument: max_attempts must be 1 to 5, or 0 for unlimited"},
		{"update action: unknown action", func() error {
			return e.updateAction(e.master, newKey(), func(r *mapsv1.UpdateSceneActionRequest) { r.Dc = proto.Int32(5) })
		}, "not_found: action not found"},

		// MoveSceneAction.
		{"move action: id not a UUID", func() error { return e.moveAction("x", up) }, "not_found: action not found"},
		{"move action: no direction", func() error { return e.moveAction(e.actionID, unset) }, "invalid_argument: direction is required"},
		{"move action: bad id wins over no direction", func() error { return e.moveAction("x", unset) }, "not_found: action not found"},
		{"move action: unknown action", func() error { return e.moveAction(newKey(), up) }, "not_found: action not found"},
		{"move action: a player", func() error {
			_, err := e.ana.maps.MoveSceneAction(t.Context(), connect.NewRequest(&mapsv1.MoveSceneActionRequest{
				CampaignId: e.campaign, MapId: e.mapID, PointId: e.pid, ActionId: e.actionID, Direction: up,
			}))
			return err
		}, "permission_denied: only the campaign's master can do this"},
	}
	for _, c := range cases {
		if got := sceneRefusal(c.call()); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

// TestSceneEditLimitsAndReplays freezes the limit of clues and actions per
// point, and the replay of a create with the same idempotency key.
func TestSceneEditLimitsAndReplays(t *testing.T) {
	t.Parallel()
	e := newSceneEditTable(t)
	pid := e.pid

	// Replays: the same key and request returns the first row, a different
	// request with the key is refused, and nothing is added twice.
	first, err := e.master.maps.AddSceneClue(t.Context(), connect.NewRequest(&mapsv1.AddSceneClueRequest{CampaignId: e.campaign, MapId: e.mapID, PointId: pid, Text: "Repetida", IdempotencyKey: "k-clue"}))
	if err != nil {
		t.Fatalf("AddSceneClue() error = %v", err)
	}
	again, err := e.master.maps.AddSceneClue(t.Context(), connect.NewRequest(&mapsv1.AddSceneClueRequest{CampaignId: e.campaign, MapId: e.mapID, PointId: pid, Text: "Repetida", IdempotencyKey: "k-clue"}))
	if err != nil || again.Msg.GetClue().GetId() != first.Msg.GetClue().GetId() || len(again.Msg.GetClues()) != 2 {
		t.Errorf("a replayed AddSceneClue = %v, %v; want the first clue and 2 clues", again, err)
	}
	if got := sceneRefusal(e.addClue(e.master, e.mapID, pid, "Outra", "k-clue")); got != "invalid_argument: idempotency_key was used for another change" {
		t.Errorf("AddSceneClue reusing a key = %q", got)
	}
	firstAction := func() (*connect.Response[mapsv1.AddSceneActionResponse], error) {
		return e.master.maps.AddSceneAction(t.Context(), connect.NewRequest(&mapsv1.AddSceneActionRequest{
			CampaignId: e.campaign, MapId: e.mapID, PointId: pid, Key: "skill:nature", Name: "Rastrear", Dc: 12, IdempotencyKey: "k-action",
		}))
	}
	a1, err := firstAction()
	if err != nil {
		t.Fatalf("AddSceneAction() error = %v", err)
	}
	a2, err := firstAction()
	if err != nil || a2.Msg.GetAction().GetId() != a1.Msg.GetAction().GetId() || len(a2.Msg.GetActions()) != 2 {
		t.Errorf("a replayed AddSceneAction = %v, %v; want the first action and 2 actions", a2, err)
	}
	if got := sceneRefusal(e.addAction(e.master, e.mapID, pid, func(r *mapsv1.AddSceneActionRequest) {
		r.Key = "skill:nature"
		r.IdempotencyKey = "k-action"
	})); got != "invalid_argument: idempotency_key was used for another change" {
		t.Errorf("AddSceneAction reusing a key = %q", got)
	}

	// The limits, on a point of their own: 30 clues and 20 actions.
	full := e.master.createPoint(&mapsv1.CreateMapPointRequest{CampaignId: e.campaign, MapId: e.mapID, Kind: mapsv1.MapPointKind_MAP_POINT_KIND_SCENE, Name: "Cheia"})
	for i := range maxSceneClues {
		if err := e.addClue(e.master, e.mapID, full.GetId(), fmt.Sprintf("pista %d", i), ""); err != nil {
			t.Fatalf("clue %d error = %v", i, err)
		}
	}
	if got := sceneRefusal(e.addClue(e.master, e.mapID, full.GetId(), "a 31ª", "")); got != "resource_exhausted: the scene already has 30 clues" {
		t.Errorf("the 31st clue = %q", got)
	}
	for i := range maxSceneActions {
		if err := e.addAction(e.master, e.mapID, full.GetId(), func(*mapsv1.AddSceneActionRequest) {}); err != nil {
			t.Fatalf("action %d error = %v", i, err)
		}
	}
	if got := sceneRefusal(e.addAction(e.master, e.mapID, full.GetId(), func(*mapsv1.AddSceneActionRequest) {})); got != "resource_exhausted: the scene already has 20 actions" {
		t.Errorf("the 21st action = %q", got)
	}
}

// sceneEditView renders the clues and the actions of the master's point as
// one line each: position order, text or key, and who has each clue.
func sceneEditView(e *sceneEditTable) string {
	var out []string
	p := e.masterPointOf(e.pid)
	for _, c := range p.GetClues() {
		var to []string
		for _, r := range c.GetRevealedTo() {
			to = append(to, r.GetCharacterName())
		}
		out = append(out, fmt.Sprintf("clue %q to [%s]", c.GetText(), strings.Join(to, ",")))
	}
	for _, a := range p.GetSceneActions() {
		out = append(out, fmt.Sprintf("action %s %q dc=%d attempts=%d (%s)", a.GetKey(), a.GetName(), a.GetDc(), a.GetMaxAttempts(), a.GetCheckName()))
	}
	return strings.Join(out, "\n")
}

// TestSceneEditResultsAreFrozen freezes what each call answers (the clue or
// action and the whole list, in order) and what the master reads afterwards.
func TestSceneEditResultsAreFrozen(t *testing.T) {
	t.Parallel()
	s := newScenes(t, false)
	e := &sceneEditTable{scenes: s, pid: s.point.GetId()}
	ctx := t.Context()
	up := mapsv1.SceneActionDirection_SCENE_ACTION_DIRECTION_UP
	down := mapsv1.SceneActionDirection_SCENE_ACTION_DIRECTION_DOWN
	texts := func(cs []*mapsv1.SceneClue) string {
		var out []string
		for _, c := range cs {
			out = append(out, c.GetText())
		}
		return strings.Join(out, "|")
	}
	keys := func(as []*mapsv1.SceneAction) string {
		var out []string
		for _, a := range as {
			out = append(out, fmt.Sprintf("%s/%s/%d/%d", a.GetKey(), a.GetName(), a.GetDc(), a.GetMaxAttempts()))
		}
		return strings.Join(out, "|")
	}

	// Clues: add three, move (down, up, the ends), reveal.
	var ids []string
	for _, text := range []string{"  primeira  ", "segunda", "terceira\ncom quebra"} {
		res, err := s.master.maps.AddSceneClue(ctx, connect.NewRequest(&mapsv1.AddSceneClueRequest{CampaignId: s.campaign, MapId: s.mapID, PointId: e.pid, Text: text}))
		if err != nil {
			t.Fatalf("AddSceneClue(%q) error = %v", text, err)
		}
		ids = append(ids, res.Msg.GetClue().GetId())
		if res.Msg.GetClue().GetText() != strings.TrimSpace(text) {
			t.Errorf("the added clue = %q", res.Msg.GetClue().GetText())
		}
	}
	step := func(id string, dir mapsv1.SceneActionDirection) string {
		res, err := s.master.maps.MoveSceneClue(ctx, connect.NewRequest(&mapsv1.MoveSceneClueRequest{CampaignId: s.campaign, MapId: s.mapID, PointId: e.pid, ClueId: id, Direction: dir}))
		if err != nil {
			t.Fatalf("MoveSceneClue() error = %v", err)
		}
		return texts(res.Msg.GetClues())
	}
	for _, c := range []struct {
		id   string
		dir  mapsv1.SceneActionDirection
		want string
	}{
		{ids[0], up, "primeira|segunda|terceira\ncom quebra"},
		{ids[0], down, "segunda|primeira|terceira\ncom quebra"},
		{ids[2], up, "segunda|terceira\ncom quebra|primeira"},
		{ids[2], down, "segunda|primeira|terceira\ncom quebra"},
		{ids[2], down, "segunda|primeira|terceira\ncom quebra"},
		{ids[1], up, "segunda|primeira|terceira\ncom quebra"},
	} {
		if got := step(c.id, c.dir); got != c.want {
			t.Errorf("moving %s %v = %q, want %q", c.id, c.dir, got, c.want)
		}
	}
	// After the moves above ids[1] is first; ids[2] last. Reveal to Pensantus.
	rev, err := s.master.revealClue(s.campaign, ids[1], s.pens.GetId(), s.pens.GetId())
	if err != nil || rev.GetText() != "segunda" || len(rev.GetRevealedTo()) != 1 || rev.GetRevealedTo()[0].GetCharacterName() != "Pensantus" {
		t.Errorf("RevealSceneClue = %v, %v", rev, err)
	}
	again, err := s.master.revealClue(s.campaign, ids[1], s.pens.GetId())
	if err != nil || len(again.GetRevealedTo()) != 1 {
		t.Errorf("revealing again = %v, %v; want it unchanged", again, err)
	}
	both, err := s.master.revealClue(s.campaign, ids[1], s.pens.GetId(), s.other.GetId())
	if err != nil || len(both.GetRevealedTo()) != 2 || both.GetRevealedTo()[0].GetCharacterName() != "Pensantus" || both.GetRevealedTo()[1].GetCharacterName() != "Toren" {
		t.Errorf("revealing to a second player = %v, %v", both, err)
	}

	// Actions: add (with the defaults and each field), update each field,
	// move, and the whole list in each answer.
	add := func(key, name string, dc int32, attempts *int32) *mapsv1.AddSceneActionResponse {
		res, err := s.master.maps.AddSceneAction(ctx, connect.NewRequest(&mapsv1.AddSceneActionRequest{
			CampaignId: s.campaign, MapId: s.mapID, PointId: e.pid, Key: key, Name: name, Dc: dc, MaxAttempts: attempts,
		}))
		if err != nil {
			t.Fatalf("AddSceneAction(%s) error = %v", key, err)
		}
		return res.Msg
	}
	a := add("skill:stealth", " Furtivo ", 0, nil)
	if keys([]*mapsv1.SceneAction{a.GetAction()}) != "skill:stealth/Furtivo/0/1" {
		t.Errorf("the default action = %s", keys([]*mapsv1.SceneAction{a.GetAction()}))
	}
	b := add("ability:str", "", 18, proto.Int32(0))
	c := add("save:wis", "Resistir", 12, proto.Int32(5))
	if got := keys(c.GetActions()); got != "skill:stealth/Furtivo/0/1|ability:str//18/0|save:wis/Resistir/12/5" {
		t.Errorf("the list after three adds = %s", got)
	}
	upd := func(id string, edit func(*mapsv1.UpdateSceneActionRequest)) *mapsv1.UpdateSceneActionResponse {
		res, err := s.update(s.master, &mapsv1.SceneAction{Id: id}, edit)
		if err != nil {
			t.Fatalf("UpdateSceneAction() error = %v", err)
		}
		return res
	}
	if got := upd(b.GetAction().GetId(), func(r *mapsv1.UpdateSceneActionRequest) { r.Name = new("Arrombar") }); keys([]*mapsv1.SceneAction{got.GetAction()}) != "ability:str/Arrombar/18/0" {
		t.Errorf("updating the name = %s", keys([]*mapsv1.SceneAction{got.GetAction()}))
	}
	if got := upd(b.GetAction().GetId(), func(r *mapsv1.UpdateSceneActionRequest) { r.Dc = proto.Int32(0) }); keys([]*mapsv1.SceneAction{got.GetAction()}) != "ability:str/Arrombar/0/0" {
		t.Errorf("clearing the dc = %s", keys([]*mapsv1.SceneAction{got.GetAction()}))
	}
	if got := upd(b.GetAction().GetId(), func(r *mapsv1.UpdateSceneActionRequest) {
		r.MaxAttempts = proto.Int32(3)
		r.Key = new("skill:athletics")
	}); keys(got.GetActions()) != "skill:stealth/Furtivo/0/1|skill:athletics/Arrombar/0/3|save:wis/Resistir/12/5" {
		t.Errorf("updating attempts and key = %s", keys(got.GetActions()))
	}
	mv := func(id string, dir mapsv1.SceneActionDirection) string {
		res, err := s.master.maps.MoveSceneAction(ctx, connect.NewRequest(&mapsv1.MoveSceneActionRequest{CampaignId: s.campaign, MapId: s.mapID, PointId: e.pid, ActionId: id, Direction: dir}))
		if err != nil {
			t.Fatalf("MoveSceneAction() error = %v", err)
		}
		var out []string
		for _, x := range res.Msg.GetActions() {
			out = append(out, x.GetKey())
		}
		return strings.Join(out, "|")
	}
	if got := mv(c.GetAction().GetId(), up); got != "skill:stealth|save:wis|skill:athletics" {
		t.Errorf("moving the last up = %s", got)
	}
	if got := mv(c.GetAction().GetId(), up); got != "save:wis|skill:stealth|skill:athletics" {
		t.Errorf("moving it up again = %s", got)
	}
	if got := mv(c.GetAction().GetId(), up); got != "save:wis|skill:stealth|skill:athletics" {
		t.Errorf("moving the first up = %s", got)
	}
	if got := mv(c.GetAction().GetId(), down); got != "skill:stealth|save:wis|skill:athletics" {
		t.Errorf("moving it down = %s", got)
	}

	const wantView = `clue "segunda" to [Pensantus,Toren]
clue "primeira" to []
clue "terceira\ncom quebra" to []
action skill:stealth "Furtivo" dc=0 attempts=1 (Furtividade)
action save:wis "Resistir" dc=12 attempts=5 (Teste de resistência de Sabedoria)
action skill:athletics "Arrombar" dc=0 attempts=3 (Atletismo)`
	if got := sceneEditView(e); got != wantView {
		t.Errorf("the master's point =\n%s\nwant\n%s", got, wantView)
	}
}

// sceneEvents renders the events a watcher got before the probe's map_changed.
func sceneEvents(w *watcher, probeMapID string) string {
	var out []string
	for _, ev := range w.drain(probeMapID) {
		switch {
		case ev.GetMapChanged() != nil:
			out = append(out, "map_changed")
		case ev.GetSceneChanged() != nil:
			out = append(out, "scene_changed")
		case ev.GetNotesChanged() != nil:
			out = append(out, "notes_changed")
		default:
			out = append(out, fmt.Sprintf("%T", ev.GetEvent()))
		}
	}
	return strings.Join(out, ",")
}

// TestSceneEditStreamsAreFrozen freezes who hears each change: with the point
// revealed on a map the players see and the scene open, and again with the
// scene closed.
func TestSceneEditStreamsAreFrozen(t *testing.T) {
	t.Parallel()
	s := newScenes(t, true)
	e := &sceneEditTable{scenes: s, pid: s.point.GetId()}
	probe := s.master.createMap(s.campaign, "Sonda", s.master.newImage(s.campaign)).GetId()
	s.master.setPointRevealed(s.campaign, s.point, true)
	up := mapsv1.SceneActionDirection_SCENE_ACTION_DIRECTION_UP
	down := mapsv1.SceneActionDirection_SCENE_ACTION_DIRECTION_DOWN

	run := func(label string, steps []struct {
		name string
		do   func()
		want [3]string // master, Ana, Caio
	},
	) {
		for _, st := range steps {
			mw, aw, cw := s.master.watch(s.campaign), s.ana.watch(s.campaign), s.caio.watch(s.campaign)
			st.do()
			s.probe(probe)
			got := [3]string{sceneEvents(mw, probe), sceneEvents(aw, probe), sceneEvents(cw, probe)}
			if got != st.want {
				t.Errorf("%s / %s: master, Ana, Caio heard %q, want %q", label, st.name, got, st.want)
			}
			s.master.setMapRevealed(s.campaign, probe, false)
		}
	}
	type step = struct {
		name string
		do   func()
		want [3]string
	}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("call error = %v", err)
		}
	}
	var second string
	s.master.mustOpenScene(s.campaign, e.pid)
	run("scene open", []step{
		{"add clue", func() { must(e.addClue(s.master, s.mapID, e.pid, "um", "")) }, [3]string{"map_changed,scene_changed", "", ""}},
		{"add second clue", func() {
			res, err := s.master.tryAddClue(s.campaign, s.mapID, e.pid, "dois")
			must(err)
			second = res.GetClue().GetId()
		}, [3]string{"map_changed,scene_changed", "", ""}},
		{"move clue up", func() { must(e.moveClue(s.master, second, up)) }, [3]string{"map_changed,scene_changed", "", ""}},
		{"move clue to the end (no change)", func() { must(e.moveClue(s.master, second, up)) }, [3]string{"", "", ""}},
		{"reveal to Ana", func() { must(e.reveal(s.master, second, s.pens.GetId())) }, [3]string{"scene_changed", "notes_changed", ""}},
		{"reveal again (nothing new)", func() { must(e.reveal(s.master, second, s.pens.GetId())) }, [3]string{"", "", ""}},
		{"add action", func() { must(e.addAction(s.master, s.mapID, e.pid, func(*mapsv1.AddSceneActionRequest) {})) }, [3]string{"map_changed,scene_changed", "map_changed,scene_changed", "map_changed,scene_changed"}},
		{"update action", func() {
			must(e.updateAction(s.master, s.master.mustGetMap(s.campaign, s.mapID).GetPoints()[0].GetSceneActions()[0].GetId(), func(r *mapsv1.UpdateSceneActionRequest) { r.Dc = proto.Int32(7) }))
		}, [3]string{"map_changed,scene_changed", "map_changed,scene_changed", "map_changed,scene_changed"}},
		{"move action at the end (no change)", func() {
			must(e.moveAction(s.master.mustGetMap(s.campaign, s.mapID).GetPoints()[0].GetSceneActions()[0].GetId(), down))
		}, [3]string{"", "", ""}},
	})
}
