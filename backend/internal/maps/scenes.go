package maps

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/maps/mapsdb"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/platform/idem"
	"github.com/PuraFome/meuRPG/backend/internal/platform/names"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
)

// The actions of an RP scene (MR-015, D6, question 51): a skill check, an
// ability check or a saving throw on a SCENE point, each with an optional
// name and an optional DC. The master changes them one at a time, as the
// artboard shows, so there is no "save all": every call below is one change,
// and answers with the point's actions as they are now.
//
// Playing the scene (opening it in the session, rolling, the log) is package
// play's; this package keeps the actions, which belong to the map point, and
// gives play the scene through SessionMaps.ScenePoint. A player gets the
// actions of a point they see, with the DC only when the scene shows it
// (show_dc, RN-20, question 52); the open
// scene carries the actions of a hidden point too, but that is play's read.

const (
	// maxSceneActions is how many actions a scene point may have (question
	// 51's default; the master's list is short, and so is the player's screen).
	maxSceneActions = 20
	// maxActionName is the longest action name, in characters
	// (scene_actions_name_length).
	maxActionName = 60
	// maxDC is the highest difficulty class (scene_actions_dc_valid).
	maxDC = 30
	// defaultAttempts is how many times a player's character may roll a new
	// action; maxAttempts is the most the master may allow, and 0 means
	// unlimited (scene_actions_max_attempts_valid, question 55).
	defaultAttempts = 1
	maxAttempts     = 5
)

// SceneChecks tells which checks a scene may ask for. The rules module
// implements it (rules.Content.SceneCheckName), so the catalog of skills and
// abilities lives in one place, and nothing that belongs to the combat (an
// attack, a spell, a feature) can be a scene action.
type SceneChecks interface {
	// SceneCheckName is the Portuguese name of a scene check by its key
	// ("skill:investigation", "ability:str", "save:wis"), and false for any
	// other key.
	SceneCheckName(key string) (string, bool)
}

// AddSceneAction implements mapsv1connect.MapServiceHandler.
func (s *Service) AddSceneAction(
	ctx context.Context,
	req *connect.Request[mapsv1.AddSceneActionRequest],
) (*connect.Response[mapsv1.AddSceneActionResponse], error) {
	m, mapID, pointID, err := s.sceneCall(ctx, req.Msg.GetCampaignId(), req.Msg.GetMapId(), req.Msg.GetPointId())
	if err != nil {
		return nil, err
	}
	key, err := s.checkKey(req.Msg.GetKey())
	if err != nil {
		return nil, err
	}
	name, err := cleanActionName(req.Msg.GetName())
	if err != nil {
		return nil, err
	}
	dc, err := cleanDC(req.Msg.GetDc())
	if err != nil {
		return nil, err
	}
	attempts := int32(defaultAttempts)
	if req.Msg.MaxAttempts != nil {
		if attempts, err = cleanAttempts(req.Msg.GetMaxAttempts()); err != nil {
			return nil, err
		}
	}

	// The key is unique in the campaign, and kept with a hash of the whole request: a retry
	// returns the first action only when it is the same request.
	scopedKey, requestHash, err := keyOf(m.CampaignID, req.Msg.GetIdempotencyKey(), req.Msg)
	if err != nil {
		return nil, err
	}
	var added mapsdb.SceneAction
	var actions []mapsdb.SceneAction
	replayed := false
	ch, err := s.changeActions(ctx, m, mapID, pointID, func(q *mapsdb.Queries) error {
		// The point is locked by now, so two calls with the same key take turns.
		current, err := q.ListSceneActions(ctx, pointID)
		if err != nil {
			return fmt.Errorf("list the actions: %w", err)
		}
		added, replayed, err = idem.Create(ctx, scopedKey, requestHash, q.GetSceneActionByCreateKey,
			func(a mapsdb.SceneAction) *string { return a.CreateHash },
			func() (mapsdb.SceneAction, error) {
				if len(current) >= maxSceneActions {
					return added, connect.NewError(connect.CodeResourceExhausted, fmt.Errorf("the scene already has %d actions", maxSceneActions))
				}
				position := int32(0)
				if n := len(current); n > 0 {
					position = current[n-1].Position + 1
				}
				a, err := q.InsertSceneAction(ctx, mapsdb.InsertSceneActionParams{
					PointID: pointID, Position: position, Key: key, Name: name, Dc: dc, MaxAttempts: attempts,
					CreateKey: scopedKey, CreateHash: requestHash, Now: s.now(),
				})
				if err != nil && !errors.Is(err, pgx.ErrNoRows) {
					return a, fmt.Errorf("insert action: %w", err)
				}
				return a, err
			})
		if err != nil {
			return err
		}
		actions = current
		if !replayed {
			actions = append(current, added)
		}
		return nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "add a scene action", err)
	}
	if !replayed {
		s.actionsChanged(ctx, m.CampaignID, ch)
	}
	return connect.NewResponse(&mapsv1.AddSceneActionResponse{
		Action: s.actionToProto(added, true), Actions: s.actionsToProto(actions, true),
	}), nil
}

// UpdateSceneAction implements mapsv1connect.MapServiceHandler.
func (s *Service) UpdateSceneAction(
	ctx context.Context,
	req *connect.Request[mapsv1.UpdateSceneActionRequest],
) (*connect.Response[mapsv1.UpdateSceneActionResponse], error) {
	m, mapID, pointID, err := s.sceneCall(ctx, req.Msg.GetCampaignId(), req.Msg.GetMapId(), req.Msg.GetPointId())
	if err != nil {
		return nil, err
	}
	actionID, ok := parseID(req.Msg.GetActionId())
	if !ok {
		return nil, errActionNotFound()
	}
	msg := req.Msg
	if msg.Key == nil && msg.Name == nil && msg.Dc == nil && msg.MaxAttempts == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("nothing to change"))
	}
	var key, name *string
	var dc *int32
	if msg.Key != nil {
		k, err := s.checkKey(msg.GetKey())
		if err != nil {
			return nil, err
		}
		key = &k
	}
	if msg.Name != nil {
		n, err := cleanActionName(msg.GetName())
		if err != nil {
			return nil, err
		}
		name = &n
	}
	if msg.Dc != nil {
		d, err := cleanDC(msg.GetDc())
		if err != nil {
			return nil, err
		}
		dc = d // nil removes the DC
	}
	var attempts *int32
	if msg.MaxAttempts != nil {
		a, err := cleanAttempts(msg.GetMaxAttempts())
		if err != nil {
			return nil, err
		}
		attempts = &a
	}

	var changed mapsdb.SceneAction
	var actions []mapsdb.SceneAction
	ch, err := s.changeActions(ctx, m, mapID, pointID, func(q *mapsdb.Queries) error {
		a, err := q.GetSceneActionForUpdate(ctx, mapsdb.GetSceneActionForUpdateParams{PointID: pointID, ID: actionID})
		if errors.Is(err, pgx.ErrNoRows) {
			return errActionNotFound()
		}
		if err != nil {
			return fmt.Errorf("find action: %w", err)
		}
		params := mapsdb.UpdateSceneActionParams{PointID: pointID, ID: actionID, Key: a.Key, Name: a.Name, Dc: a.Dc, MaxAttempts: a.MaxAttempts, Now: s.now()}
		if key != nil {
			params.Key = *key
		}
		if name != nil {
			params.Name = *name
		}
		if msg.Dc != nil {
			params.Dc = dc
		}
		if attempts != nil {
			params.MaxAttempts = *attempts
		}
		if changed, err = q.UpdateSceneAction(ctx, params); err != nil {
			return fmt.Errorf("update action: %w", err)
		}
		if actions, err = q.ListSceneActions(ctx, pointID); err != nil {
			return fmt.Errorf("list the actions: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "update a scene action", err)
	}
	s.actionsChanged(ctx, m.CampaignID, ch)
	return connect.NewResponse(&mapsv1.UpdateSceneActionResponse{
		Action: s.actionToProto(changed, true), Actions: s.actionsToProto(actions, true),
	}), nil
}

// MoveSceneAction implements mapsv1connect.MapServiceHandler.
func (s *Service) MoveSceneAction(
	ctx context.Context,
	req *connect.Request[mapsv1.MoveSceneActionRequest],
) (*connect.Response[mapsv1.MoveSceneActionResponse], error) {
	m, mapID, pointID, err := s.sceneCall(ctx, req.Msg.GetCampaignId(), req.Msg.GetMapId(), req.Msg.GetPointId())
	if err != nil {
		return nil, err
	}
	actionID, ok := parseID(req.Msg.GetActionId())
	if !ok {
		return nil, errActionNotFound()
	}
	var step int
	switch req.Msg.GetDirection() {
	case mapsv1.SceneActionDirection_SCENE_ACTION_DIRECTION_UP:
		step = -1
	case mapsv1.SceneActionDirection_SCENE_ACTION_DIRECTION_DOWN:
		step = 1
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("direction is required"))
	}

	var actions []mapsdb.SceneAction
	moved := false
	ch, err := s.changeActions(ctx, m, mapID, pointID, func(q *mapsdb.Queries) error {
		var err error
		moved = false
		if actions, err = q.ListSceneActions(ctx, pointID); err != nil {
			return fmt.Errorf("list the actions: %w", err)
		}
		i := slices.IndexFunc(actions, func(a mapsdb.SceneAction) bool { return a.ID == actionID })
		if i < 0 {
			return errActionNotFound()
		}
		j := i + step
		if j < 0 || j >= len(actions) {
			return nil // already first or last: nothing changes
		}
		actions[i], actions[j] = actions[j], actions[i]
		// Renumber the whole list from 0: positions may have gaps after a
		// removal, and a swap of two values would leave equal ones.
		for p := range actions {
			want := int32(p) //nolint:gosec // G115: at most 20 actions
			if actions[p].Position == want {
				continue
			}
			actions[p].Position = want
			if err := q.SetSceneActionPosition(ctx, mapsdb.SetSceneActionPositionParams{ID: actions[p].ID, Position: want}); err != nil {
				return fmt.Errorf("set an action's position: %w", err)
			}
		}
		moved = true
		return nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "move a scene action", err)
	}
	if moved {
		s.actionsChanged(ctx, m.CampaignID, ch)
	}
	return connect.NewResponse(&mapsv1.MoveSceneActionResponse{Actions: s.actionsToProto(actions, true)}), nil
}

// RemoveSceneAction implements mapsv1connect.MapServiceHandler.
func (s *Service) RemoveSceneAction(
	ctx context.Context,
	req *connect.Request[mapsv1.RemoveSceneActionRequest],
) (*connect.Response[mapsv1.RemoveSceneActionResponse], error) {
	m, mapID, pointID, err := s.sceneCall(ctx, req.Msg.GetCampaignId(), req.Msg.GetMapId(), req.Msg.GetPointId())
	if err != nil {
		return nil, err
	}
	actionID, ok := parseID(req.Msg.GetActionId())
	if !ok {
		return nil, errActionNotFound()
	}
	var actions []mapsdb.SceneAction
	ch, err := s.changeActions(ctx, m, mapID, pointID, func(q *mapsdb.Queries) error {
		n, err := q.DeleteSceneAction(ctx, mapsdb.DeleteSceneActionParams{PointID: pointID, ID: actionID})
		if err != nil {
			return fmt.Errorf("delete action: %w", err)
		}
		if n == 0 {
			return errActionNotFound()
		}
		if actions, err = q.ListSceneActions(ctx, pointID); err != nil {
			return fmt.Errorf("list the actions: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "remove a scene action", err)
	}
	s.actionsChanged(ctx, m.CampaignID, ch)
	return connect.NewResponse(&mapsv1.RemoveSceneActionResponse{Actions: s.actionsToProto(actions, true)}), nil
}

// sceneCall is the start of every scene-action handler: the master's check,
// and the IDs of the map and the point.
func (s *Service) sceneCall(ctx context.Context, campaignID, rawMapID, rawPointID string) (m authz.Membership, mapID, pointID string, err error) {
	m, err = authz.RequireCampaignRole(ctx, campaignID, authz.RoleMaster)
	if err != nil {
		return m, "", "", err
	}
	mapID, ok := parseID(rawMapID)
	if !ok {
		return m, "", "", errMapNotFound()
	}
	pointID, ok = parseID(rawPointID)
	if !ok {
		return m, "", "", errPointNotFound()
	}
	return m, mapID, pointID, nil
}

// changedPoint is what a change to a point's actions needs to tell the
// watching members: the point, its map, and the session's current map.
type changedPoint struct {
	point      mapsdb.MapPoint
	mapRow     mapsdb.Map
	currentMap string
}

// changeActions runs one change to a scene point's actions in a transaction:
// it checks that the map is the campaign's and the point is a SCENE point of
// it, locks the point so two changes take turns (the 20-action limit holds),
// and lets change work on the rows.
func (s *Service) changeActions(ctx context.Context, m authz.Membership, mapID, pointID string, change func(q *mapsdb.Queries) error) (changedPoint, error) {
	current, err := s.currentMap(ctx, m.CampaignID)
	if err != nil {
		return changedPoint{}, err
	}
	out := changedPoint{currentMap: current}
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		var err error
		if out.mapRow, err = s.campaignMap(ctx, q, m.CampaignID, mapID); err != nil {
			return err
		}
		out.point, err = q.GetMapPointForUpdate(ctx, mapsdb.GetMapPointForUpdateParams{MapID: mapID, ID: pointID})
		if errors.Is(err, pgx.ErrNoRows) {
			return errPointNotFound()
		}
		if err != nil {
			return fmt.Errorf("find point: %w", err)
		}
		if out.point.Kind != kindToDB[mapsv1.MapPointKind_MAP_POINT_KIND_SCENE] {
			return connect.NewError(connect.CodeInvalidArgument, errors.New("only a SCENE point has actions and clues"))
		}
		return change(q)
	})
	return out, err
}

// actionsChanged tells the watching members that a scene point's actions
// changed: the master's map reads them, a player's too when the point is
// revealed on a map they see (map_changed, as any change to a point), and
// everyone in the session when the point is the open scene (scene_changed).
func (s *Service) actionsChanged(ctx context.Context, campaignID string, ch changedPoint) {
	s.publishPointsChanged(ctx, campaignID, ch.mapRow,
		playersSee(ch.mapRow.ID, ch.mapRow.RevealedAt, ch.currentMap) && ch.point.RevealedAt != nil, ch.point)
	s.publishSceneChangedIf(ctx, campaignID, ch.point.ID)
}

// publishSceneChangedIf tells everyone that the open scene changed, when the
// point is the scene open in the session. The hint names nothing: the app
// reads the scene again, filtered for it.
func (s *Service) publishSceneChangedIf(ctx context.Context, campaignID, pointID string) {
	open, err := s.live.OpenScenePoint(ctx, campaignID)
	if err != nil {
		s.logger.ErrorContext(ctx, "maps: read the open scene", "error", err)
		return
	}
	if open != "" && open == pointID {
		s.publishSceneChanged(campaignID)
	}
}

// publishSceneChanged tells everyone in the session that the open scene
// changed.
func (s *Service) publishSceneChanged(campaignID string) {
	s.live.Publish(campaignID, true, &playv1.WatchGameSessionResponse{Event: &playv1.WatchGameSessionResponse_SceneChanged_{
		SceneChanged: &playv1.WatchGameSessionResponse_SceneChanged{},
	}})
}

// checkKey checks a scene check's key against the rules' catalog.
func (s *Service) checkKey(key string) (string, error) {
	if _, ok := s.checks.SceneCheckName(key); !ok {
		return "", connect.NewError(connect.CodeInvalidArgument,
			errors.New("key must be a skill, an ability check or a saving throw (skill:investigation, ability:str, save:wis)"))
	}
	return key, nil
}

// cleanActionName checks an action's name: empty for none, else one line of
// up to 60 characters. The message says the rule, never the name.
func cleanActionName(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", nil
	}
	name, err := names.Clean(raw, maxActionName)
	if err != nil {
		return "", connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("name %w", err))
	}
	return name, nil
}

// cleanDC checks a difficulty class: 0 for none, else 1 to 30.
func cleanDC(dc int32) (*int32, error) {
	switch {
	case dc == 0:
		return nil, nil
	case dc < 1 || dc > maxDC:
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("dc must be 1 to %d, or 0 for none", maxDC))
	}
	return &dc, nil
}

// cleanAttempts checks an action's attempts per player: 1 to 5, or 0 for
// unlimited.
func cleanAttempts(n int32) (int32, error) {
	if n < 0 || n > maxAttempts {
		return 0, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("max_attempts must be 1 to %d, or 0 for unlimited", maxAttempts))
	}
	return n, nil
}

func errActionNotFound() error {
	return connect.NewError(connect.CodeNotFound, errors.New("action not found"))
}

// actionToProto builds a SceneAction. The DC goes to the master, and to a
// player only when the scene shows it (RN-20): withDC says whether the viewer
// gets it.
func (s *Service) actionToProto(a mapsdb.SceneAction, withDC bool) *mapsv1.SceneAction {
	checkName, _ := s.checks.SceneCheckName(a.Key)
	out := &mapsv1.SceneAction{Id: a.ID, Key: a.Key, Name: a.Name, CheckName: checkName, MaxAttempts: a.MaxAttempts}
	if withDC && a.Dc != nil {
		out.Dc = *a.Dc
	}
	return out
}

func (s *Service) actionsToProto(rows []mapsdb.SceneAction, withDC bool) []*mapsv1.SceneAction {
	out := make([]*mapsv1.SceneAction, 0, len(rows))
	for _, a := range rows {
		out = append(out, s.actionToProto(a, withDC))
	}
	return out
}

// attachActions fills the scene_actions of the SCENE points of a map, as the
// viewer sees them: a player gets a DC only from a scene that shows it
// (show_dc).
func (s *Service) attachActions(ctx context.Context, mapID string, points []*mapsv1.MapPoint, master bool) error {
	if !slices.ContainsFunc(points, func(p *mapsv1.MapPoint) bool { return p.GetKind() == mapsv1.MapPointKind_MAP_POINT_KIND_SCENE }) {
		return nil
	}
	rows, err := s.queries.ListSceneActionsOfMap(ctx, mapID)
	if err != nil {
		return err
	}
	byPoint := map[string][]mapsdb.SceneAction{}
	for _, r := range rows {
		byPoint[r.PointID] = append(byPoint[r.PointID], r)
	}
	for _, p := range points {
		p.SceneActions = s.actionsToProto(byPoint[p.GetId()], master || p.GetShowDc())
	}
	return nil
}

// ScenePoint implements play.MapKeeper (through SessionMaps): the scene of a
// SCENE point of the campaign, hidden or not, with its actions and their DCs, its hooks and its clues (the master's: package play gives them to the master only),
// or a `not_found` Connect error for any other point. Package play decides
// what each member sees of it.
func (sm *SessionMaps) ScenePoint(ctx context.Context, tx pgx.Tx, campaignID, pointID string) (link.Scene, error) {
	q := queriesIn(sm.queries, tx)
	p, err := q.GetScenePoint(ctx, mapsdb.GetScenePointParams{CampaignID: campaignID, ID: pointID})
	if errors.Is(err, pgx.ErrNoRows) {
		return link.Scene{}, errPointNotFound()
	}
	if err != nil {
		return link.Scene{}, fmt.Errorf("find the scene point: %w", err)
	}
	rows, err := q.ListSceneActions(ctx, p.ID)
	if err != nil {
		return link.Scene{}, fmt.Errorf("list the scene's actions: %w", err)
	}
	out := link.Scene{PointID: p.ID, Name: p.Name, Description: p.Description, Hooks: p.Hooks, ShowDC: p.ShowDc}
	if out.Clues, err = sm.sceneClues(ctx, q, p.ID); err != nil {
		return link.Scene{}, err
	}
	for _, a := range rows {
		act := link.SceneAction{ID: a.ID, Key: a.Key, Name: a.Name, MaxAttempts: int(a.MaxAttempts)}
		if a.Dc != nil {
			act.DC = int(*a.Dc)
		}
		out.Actions = append(out.Actions, act)
	}
	return out, nil
}
