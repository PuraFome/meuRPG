package play

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/types/known/timestamppb"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/live"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// The RP scene during a session (MR-015, Etapa 7, D7; questions 51 to 55 of
// the progress doc, with their defaults).
//
//   - The scene is a map point of kind SCENE with the master's actions on it
//     (package maps keeps them). The master opens one in the session
//     (game_sessions.open_scene_point_id), like showing an image: a hidden
//     point can be opened, and it stays hidden on the map.
//   - Every member reads the open scene (GetOpenScene). A player gets their
//     own character's bonus on each action, from the rules engine, and
//     never a DC, a pass or fail, or another player's roll (RN-20, question
//     52). The master gets the DCs and every roll.
//   - A player rolls each action once while the scene is open (question 55):
//     the app rolls the d20, or takes a typed one (RN-18). Closing the scene
//     and opening it again starts afresh: a roll counts for the opening that
//     is the session's latest `scene_opened` event.
//   - Opening, closing and rolling are session events (ADR-0007): ids and
//     numbers only, never a name or the master's words.

// sceneEvent is the payload of scene_opened and scene_closed.
type sceneEvent struct {
	PointID string `json:"point_id"`
	// Actions is how many actions the scene had when it opened.
	Actions int `json:"actions,omitempty"`
}

// sceneRollEvent is the payload of scene_check_rolled. Numbers and ids only:
// the action's key is a rules key, never the master's name for it, and the DC
// itself is not kept, only whether the roll reached it.
type sceneRollEvent struct {
	PointID  string `json:"point_id"`
	ActionID string `json:"action_id"`
	Key      string `json:"key"`
	D20      int32  `json:"d20"`
	Modifier int32  `json:"modifier"`
	Total    int32  `json:"total"`
	Physical bool   `json:"physical,omitempty"`
	// Passed is set only when the action had a DC.
	Passed *bool `json:"passed,omitempty"`
}

// errScene is the failed_precondition of the scene calls, with the
// SceneBlocked detail that tells the app why.
func errScene(reason playv1.SceneBlockedReason, msg string) error {
	err := connect.NewError(connect.CodeFailedPrecondition, errors.New(msg))
	if detail, detailErr := connect.NewErrorDetail(&playv1.SceneBlocked{Reason: reason}); detailErr == nil {
		err.AddDetail(detail)
	}
	return err
}

func errScenePointNotFound() error {
	return connect.NewError(connect.CodeNotFound, errors.New("scene point not found"))
}

func errSceneActionNotFound() error {
	return connect.NewError(connect.CodeNotFound, errors.New("action not found"))
}

// OpenScenePoint returns the map point of the scene open in the campaign's open
// session, "" when none is open or no session is. It implements
// maps.LiveSession.
func (s *Service) OpenScenePoint(ctx context.Context, campaignID string) (string, error) {
	point, err := s.queries.GetOpenScenePoint(ctx, campaignID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read the open scene: %w", err)
	}
	return deref(point), nil
}

// publishSceneChanged tells everyone in the session that the open scene
// changed. The hint names nothing: each app reads the scene again, filtered
// for it.
func (s *Service) publishSceneChanged(campaignID string) {
	s.Publish(campaignID, true, &playv1.WatchGameSessionResponse{Event: &playv1.WatchGameSessionResponse_SceneChanged_{
		SceneChanged: &playv1.WatchGameSessionResponse_SceneChanged{},
	}})
}

// insertSceneEvent appends a session event inside tx, and returns its id.
func insertSceneEvent(ctx context.Context, c *combatTx, kind string, actor, key *string, payload any) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode the event payload: %w", err)
	}
	seq, err := c.q.NextSessionEventSeq(ctx, c.session.ID)
	if err != nil {
		return "", fmt.Errorf("next event number: %w", err)
	}
	row, err := c.q.InsertSessionEvent(ctx, playdb.InsertSessionEventParams{
		GameSessionID: c.session.ID, Seq: seq, Kind: kind, ActorUserID: actor, CharacterID: c.characterID,
		Payload: body, IdempotencyKey: key, CreatedAt: c.now,
	})
	if err != nil {
		return "", fmt.Errorf("insert session event: %w", err)
	}
	return row.ID, nil
}

// OpenScene implements playv1connect.PlayServiceHandler.
func (s *Service) OpenScene(
	ctx context.Context,
	req *connect.Request[playv1.OpenSceneRequest],
) (*connect.Response[playv1.OpenSceneResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	pointID, err := uuid.Parse(req.Msg.GetPointId())
	if err != nil {
		return nil, errScenePointNotFound()
	}
	// The point must be a SCENE point of the campaign's maps, hidden or not.
	scene, err := s.maps.ScenePoint(ctx, m.CampaignID, pointID.String())
	if err != nil {
		return nil, s.dbError(ctx, "find the scene point", err)
	}
	if len(scene.Actions) == 0 {
		return nil, errScene(playv1.SceneBlockedReason_SCENE_BLOCKED_REASON_NO_ACTIONS, "a scene needs at least one action")
	}

	var session playdb.GameSession
	var changed bool // another scene (or none) is open now
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		var err error
		changed = false
		if session, err = q.GetOpenGameSessionForUpdate(ctx, m.CampaignID); errors.Is(err, pgx.ErrNoRows) {
			return errNoOpenSession()
		} else if err != nil {
			return fmt.Errorf("lock the open session: %w", err)
		}
		if equal(session.OpenScenePointID, ptr(pointID.String())) {
			return nil // already open: the rolls so far stay
		}
		point := pointID.String()
		if session, err = q.SetOpenScene(ctx, playdb.SetOpenSceneParams{ID: session.ID, OpenScenePointID: &point}); err != nil {
			return fmt.Errorf("open the scene: %w", err)
		}
		c := &combatTx{tx: tx, q: q, session: session, now: s.now()}
		if _, err := insertSceneEvent(ctx, c, eventSceneOpened, &m.UserID, nil, sceneEvent{PointID: point, Actions: len(scene.Actions)}); err != nil {
			return err
		}
		changed = true
		return nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "open a scene", err)
	}
	if changed {
		s.publishSceneChanged(m.CampaignID)
	}
	info, err := s.sceneInfo(ctx, m, session, scene)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.OpenSceneResponse{Scene: info}), nil
}

// CloseScene implements playv1connect.PlayServiceHandler.
func (s *Service) CloseScene(
	ctx context.Context,
	req *connect.Request[playv1.CloseSceneRequest],
) (*connect.Response[playv1.CloseSceneResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	var changed bool
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		changed = false
		session, err := q.GetOpenGameSessionForUpdate(ctx, m.CampaignID)
		if errors.Is(err, pgx.ErrNoRows) {
			return errNoOpenSession()
		}
		if err != nil {
			return fmt.Errorf("lock the open session: %w", err)
		}
		if session.OpenScenePointID == nil {
			return nil // none open: nothing to close
		}
		if _, err := q.SetOpenScene(ctx, playdb.SetOpenSceneParams{ID: session.ID}); err != nil {
			return fmt.Errorf("close the scene: %w", err)
		}
		c := &combatTx{tx: tx, q: q, session: session, now: s.now()}
		if _, err := insertSceneEvent(ctx, c, eventSceneClosed, &m.UserID, nil, sceneEvent{PointID: *session.OpenScenePointID}); err != nil {
			return err
		}
		changed = true
		return nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "close a scene", err)
	}
	if changed {
		s.publishSceneChanged(m.CampaignID)
	}
	return connect.NewResponse(&playv1.CloseSceneResponse{}), nil
}

// GetOpenScene implements playv1connect.PlayServiceHandler.
func (s *Service) GetOpenScene(
	ctx context.Context,
	req *connect.Request[playv1.GetOpenSceneRequest],
) (*connect.Response[playv1.GetOpenSceneResponse], error) {
	m, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	session, err := s.openSession(ctx, m.CampaignID)
	if err != nil {
		return nil, err
	}
	res := &playv1.GetOpenSceneResponse{}
	if session.OpenScenePointID == nil {
		return connect.NewResponse(res), nil
	}
	scene, err := s.maps.ScenePoint(ctx, m.CampaignID, *session.OpenScenePointID)
	if connect.CodeOf(err) == connect.CodeNotFound {
		// The point stopped being a scene since it was opened (the master
		// changed its kind): no scene is open.
		return connect.NewResponse(res), nil
	}
	if err != nil {
		return nil, s.dbError(ctx, "read the open scene", err)
	}
	if res.Scene, err = s.sceneInfo(ctx, m, session, scene); err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

// myCharacter is the caller's own living character, if they have one: the
// one that rolls for them.
func (s *Service) myCharacter(ctx context.Context, m authz.Membership) (link.Character, bool, error) {
	if m.Role == authz.RoleMaster {
		return link.Character{}, false, nil
	}
	party, err := s.roster.CombatParty(ctx, m.CampaignID)
	if err != nil {
		return link.Character{}, false, err
	}
	i := slices.IndexFunc(party, func(c link.Character) bool { return c.PlayerUserID != "" && c.PlayerUserID == m.UserID })
	if i < 0 {
		return link.Character{}, false, nil
	}
	return party[i], true, nil
}

// sceneInfo builds the open scene as the caller sees it (RN-20, question 52):
// the master gets the DCs and every roll with its pass or fail; a player gets
// no DC, their own character's bonus on each action, and only their own
// rolls, with no pass or fail.
func (s *Service) sceneInfo(ctx context.Context, m authz.Membership, session playdb.GameSession, scene link.Scene) (*playv1.OpenSceneInfo, error) {
	master := m.Role == authz.RoleMaster
	info := &playv1.OpenSceneInfo{PointId: scene.PointID, Name: scene.Name, Description: scene.Description}

	mine, hasMine, err := s.myCharacter(ctx, m)
	if err != nil {
		return nil, s.dbError(ctx, "find the caller's character", err)
	}
	var options []link.SceneOption
	if hasMine {
		keys := make([]string, 0, len(scene.Actions))
		for _, a := range scene.Actions {
			keys = append(keys, a.Key)
		}
		if options, err = s.roster.SceneOptions(ctx, m.CampaignID, mine.ID, keys); err != nil {
			return nil, s.dbError(ctx, "work out the scene's bonuses", err)
		}
	}
	for i, a := range scene.Actions {
		v := &playv1.SceneActionView{Id: a.ID, Key: a.Key, Name: a.Name, CheckName: s.roster.SceneCheckName(a.Key)}
		if master {
			v.Dc = clamp32(a.DC, 0, 30)
		} else if i < len(options) && options[i].Known {
			v.Bonus = ptr(clamp32(options[i].Bonus, math.MinInt32, math.MaxInt32))
			if options[i].HasPassive {
				v.Passive = ptr(clamp32(options[i].Passive, math.MinInt32, math.MaxInt32))
			}
		}
		info.Actions = append(info.Actions, v)
	}

	opened, err := s.queries.GetOpenSceneEvent(ctx, session.ID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		opened.Seq = 0 // opened before events were kept: every roll counts
	case err != nil:
		return nil, s.dbError(ctx, "read when the scene opened", err)
	default:
		info.OpenedAt = timestamppb.New(opened.CreatedAt)
	}
	rolls, err := s.sceneRolls(ctx, m, session.ID, opened.Seq, mine.ID, hasMine)
	if err != nil {
		return nil, err
	}
	info.Rolls = rolls
	return info, nil
}

// sceneRolls reads the rolls of the opening that began at event number
// after, newest first: all of them for the master, the caller's own
// character's for a player.
func (s *Service) sceneRolls(ctx context.Context, m authz.Membership, sessionID string, after int32, mineID string, hasMine bool) ([]*playv1.SceneRoll, error) {
	master := m.Role == authz.RoleMaster
	if !master && !hasMine {
		return nil, nil // a player with no living character has no rolls
	}
	rows, err := s.queries.ListSceneRollEvents(ctx, playdb.ListSceneRollEventsParams{GameSessionID: sessionID, Seq: after})
	if err != nil {
		return nil, s.dbError(ctx, "list the scene's rolls", err)
	}
	var ids []string
	for _, r := range rows {
		if r.CharacterID != nil && !slices.Contains(ids, *r.CharacterID) {
			ids = append(ids, *r.CharacterID)
		}
	}
	names := map[string]string{}
	if len(ids) > 0 {
		chars, err := s.roster.CombatCharacters(ctx, m.CampaignID, ids)
		if err != nil {
			return nil, s.dbError(ctx, "read the rollers' names", err)
		}
		for _, c := range chars {
			names[c.ID] = c.Name
		}
	}
	var out []*playv1.SceneRoll
	for _, r := range rows {
		if !master && (r.CharacterID == nil || *r.CharacterID != mineID) {
			continue
		}
		var ev sceneRollEvent
		if err := json.Unmarshal(r.Payload, &ev); err != nil {
			return nil, s.dbError(ctx, "read a scene roll", fmt.Errorf("decode the roll of event %s: %w", r.ID, err))
		}
		out = append(out, sceneRollToProto(r.ID, deref(r.CharacterID), names[deref(r.CharacterID)], r.CreatedAt, ev, master))
	}
	return out, nil
}

// sceneRollToProto builds a roll as the caller sees it: the pass or fail goes
// only to the master (RN-20).
func sceneRollToProto(id, characterID, characterName string, at time.Time, ev sceneRollEvent, master bool) *playv1.SceneRoll {
	out := &playv1.SceneRoll{
		Id: id, ActionId: ev.ActionID, CharacterId: characterID, CharacterName: characterName,
		Roll:     diceRoll(1, 20, []int32{ev.D20}, ev.Modifier, ev.Total, ev.Physical),
		RolledAt: timestamppb.New(at),
	}
	if master {
		out.Passed = ev.Passed
	}
	return out
}

// RollSceneCheck implements playv1connect.PlayServiceHandler.
func (s *Service) RollSceneCheck(
	ctx context.Context,
	req *connect.Request[playv1.RollSceneCheckRequest],
) (*connect.Response[playv1.RollSceneCheckResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RolePlayer)
	if err != nil {
		return nil, err
	}
	key, err := parseKey(req.Msg.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}
	actionID, err := uuid.Parse(req.Msg.GetActionId())
	if err != nil {
		return nil, errSceneActionNotFound()
	}
	var in rollInput
	switch roll := req.Msg.GetRoll().(type) {
	case *playv1.RollSceneCheckRequest_RollInApp:
		if !roll.RollInApp {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("roll_in_app must be true"))
		}
		in.inApp = true
	case *playv1.RollSceneCheckRequest_D20Face:
		in.typed = int(roll.D20Face)
		if in.typed < 1 || in.typed > 20 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("d20_face must be 1 to 20"))
		}
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("set roll_in_app or d20_face"))
	}

	var (
		ev       sceneRollEvent
		rollID   string
		who      link.Character
		repeated bool
		at       time.Time
		doneRow  playdb.GetSessionEventByIdempotencyKeyRow
	)
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		repeated = false
		session, err := q.GetOpenGameSessionForUpdate(ctx, m.CampaignID)
		if errors.Is(err, pgx.ErrNoRows) {
			return errNoOpenSession()
		}
		if err != nil {
			return fmt.Errorf("lock the open session: %w", err)
		}

		// A retry of a roll already made returns that roll, and changes nothing.
		doneRow, err = q.GetSessionEventByIdempotencyKey(ctx, playdb.GetSessionEventByIdempotencyKeyParams{
			GameSessionID: session.ID, IdempotencyKey: &key,
		})
		switch {
		case err == nil:
			if doneRow.Kind != eventSceneCheckRolled {
				return connect.NewError(connect.CodeInvalidArgument, errors.New("idempotency_key was already used for another change"))
			}
			repeated = true
			rollID, at = doneRow.ID, doneRow.CreatedAt
			if err := json.Unmarshal(doneRow.Payload, &ev); err != nil {
				return fmt.Errorf("decode the roll of event %s: %w", doneRow.ID, err)
			}
			return nil
		case !errors.Is(err, pgx.ErrNoRows):
			return fmt.Errorf("find the event of this idempotency key: %w", err)
		}

		if session.OpenScenePointID == nil {
			return errScene(playv1.SceneBlockedReason_SCENE_BLOCKED_REASON_NO_OPEN_SCENE, "no scene is open")
		}
		scene, err := s.maps.ScenePoint(ctx, m.CampaignID, *session.OpenScenePointID)
		if connect.CodeOf(err) == connect.CodeNotFound {
			return errScene(playv1.SceneBlockedReason_SCENE_BLOCKED_REASON_NO_OPEN_SCENE, "no scene is open") // no longer a scene point
		}
		if err != nil {
			return err
		}
		i := slices.IndexFunc(scene.Actions, func(a link.SceneAction) bool { return a.ID == actionID.String() })
		if i < 0 {
			return errSceneActionNotFound()
		}
		action := scene.Actions[i]

		// The player's own living character rolls (RN-18: how, as the campaign
		// allows).
		var has bool
		if who, has, err = s.myCharacter(ctx, m); err != nil {
			return err
		}
		if !has {
			return errScene(playv1.SceneBlockedReason_SCENE_BLOCKED_REASON_NO_CHARACTER, "you have no living character to roll for")
		}
		force, err := s.dice.ForcedDice(ctx, m.CampaignID, m.UserID)
		if err != nil {
			return err
		}
		if force.refuses(in.inApp) {
			return errScene(playv1.SceneBlockedReason_SCENE_BLOCKED_REASON_WRONG_DICE_MODE, "this is not how the campaign has you roll your dice")
		}
		options, err := s.roster.SceneOptions(ctx, m.CampaignID, who.ID, []string{action.Key})
		if err != nil {
			return err
		}
		if len(options) != 1 || !options[0].Known {
			return errScene(playv1.SceneBlockedReason_SCENE_BLOCKED_REASON_NO_CHARACTER, "your character has no numbers for this check")
		}

		// One roll per character per action while the scene is open: the rolls
		// since its opening (question 55).
		opened, err := q.GetOpenSceneEvent(ctx, session.ID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("read when the scene opened: %w", err)
		}
		rolled, err := q.ListSceneRollEvents(ctx, playdb.ListSceneRollEventsParams{GameSessionID: session.ID, Seq: opened.Seq})
		if err != nil {
			return fmt.Errorf("list the scene's rolls: %w", err)
		}
		for _, r := range rolled {
			var prev sceneRollEvent
			if err := json.Unmarshal(r.Payload, &prev); err != nil {
				return fmt.Errorf("decode the roll of event %s: %w", r.ID, err)
			}
			if r.CharacterID != nil && *r.CharacterID == who.ID && prev.ActionID == action.ID {
				return errScene(playv1.SceneBlockedReason_SCENE_BLOCKED_REASON_ALREADY_ROLLED, "this action was already rolled; the master closes and opens the scene again to allow another roll")
			}
		}

		bonus := options[0].Bonus
		face, roll, err := s.d20(in, bonus)
		if err != nil {
			return err
		}
		ev = sceneRollEvent{
			PointID: scene.PointID, ActionID: action.ID, Key: action.Key,
			D20: clamp32(face, 1, 20), Modifier: clamp32(bonus, math.MinInt32, math.MaxInt32), Total: clamp32(roll.Total, math.MinInt32, math.MaxInt32),
			Physical: roll.Physical,
		}
		if action.DC > 0 {
			ev.Passed = ptr(roll.Total >= action.DC)
		}
		c := &combatTx{tx: tx, q: q, session: session, now: s.now(), characterID: &who.ID}
		at = c.now
		rollID, err = insertSceneEvent(ctx, c, eventSceneCheckRolled, &m.UserID, &key, ev)
		return err
	})
	if err != nil {
		return nil, s.dbError(ctx, "roll a scene check", err)
	}

	characterID, name := who.ID, who.Name
	if repeated {
		// Written the first time: the character and the time are the event's.
		characterID, name = deref(doneRow.CharacterID), ""
		if chars, err := s.roster.CombatCharacters(ctx, m.CampaignID, []string{characterID}); err == nil && len(chars) == 1 {
			name = chars[0].Name
		}
	} else {
		// Only the master and the roller hear of it (RN-20).
		s.hub.Publish(m.CampaignID, live.Event{
			Audience: live.Audience{Master: true, UserID: m.UserID},
			Message: &playv1.WatchGameSessionResponse{Event: &playv1.WatchGameSessionResponse_SceneCheckRolled_{
				SceneCheckRolled: &playv1.WatchGameSessionResponse_SceneCheckRolled{ActionId: ev.ActionID},
			}},
		})
	}
	// The caller is a player: no pass or fail in the answer.
	return connect.NewResponse(&playv1.RollSceneCheckResponse{
		Roll: sceneRollToProto(rollID, characterID, name, at, ev, false),
	}), nil
}
