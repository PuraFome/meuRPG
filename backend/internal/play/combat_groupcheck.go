package play

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/types/known/timestamppb"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/platform/idem"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/live"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// The group check (SRD 5.1, "Working Together", Group Checks): the master asks the whole
// party for one check; everyone makes it, and if at least half of the group succeeds, the
// whole group succeeds. The count is over the characters asked (the living player
// characters when the master asks), and a character that did not answer when the master
// closes the check counts as failed.
//
// The same request also asks only some characters, and for a saving throw ("save:con") as
// well as a skill or an ability check, at any moment out of combat. When it asks named
// characters, each roll is judged alone (is_group false): there is no group verdict.
// Only the characters asked read the request or may roll it (RN-10): a player who was not
// asked reads no group check at all.
//
// What each side reads (RN-20): a player reads their own roll, and passed or failed only
// when the master shows the DC (the scene checks' rule); the group's verdict, once the
// check is closed, on the same condition. The DC is never a player's. The master reads
// everything, who has not answered, and may roll for a character or close the check.
//
// The check lives in the session, not in a combat: the master asks it in a scene, on the
// trail. One group check is open at a time.

// groupCheckEvent is the payload of the group check's events: IDs only.
type groupCheckEvent struct {
	GroupCheckID string `json:"group_check_id"`
	CharacterID  string `json:"character_id,omitempty"`
}

// groupWrite runs one change to the open session's group check, as the scene checks do: one
// transaction locks the open session, replays a retry of the key, lets do change the rows
// and appends the event. It returns whether the call was a retry.
func (s *Service) groupWrite(ctx context.Context, m authz.Membership, key string, hash *string, kind string, do func(q *playdb.Queries, tx pgx.Tx, session playdb.GameSession) (payload groupCheckEvent, characterID string, err error)) (groupCheckEvent, bool, error) {
	var ev groupCheckEvent
	var repeated bool
	err := db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		ev, repeated = groupCheckEvent{}, false
		q := s.queries.WithTx(tx)
		session, err := q.GetOpenGameSessionForUpdate(ctx, m.CampaignID)
		if errors.Is(err, pgx.ErrNoRows) {
			return errNoOpenSession()
		}
		if err != nil {
			return fmt.Errorf("lock the open session: %w", err)
		}
		done, err := q.GetSessionEventByIdempotencyKey(ctx, playdb.GetSessionEventByIdempotencyKeyParams{GameSessionID: session.ID, IdempotencyKey: &key})
		switch {
		case err == nil:
			if done.Kind != kind || done.ActorUserID == nil || *done.ActorUserID != m.UserID || hashDiffers(done.IdempotencyHash, hash) {
				return connect.NewError(connect.CodeInvalidArgument, errors.New("idempotency_key was already used for another change"))
			}
			repeated = true
			// A retry answers what the first call made: the check it names is in the event.
			if err := json.Unmarshal(done.Payload, &ev); err != nil {
				return fmt.Errorf("read the event of this idempotency key: %w", err)
			}
			return nil
		case !errors.Is(err, pgx.ErrNoRows):
			return fmt.Errorf("find the event of this idempotency key: %w", err)
		}
		c, err := s.openTx(ctx, combatTx{tx: tx, q: q, session: session, now: s.now(), hash: hash, svc: s})
		if err != nil {
			return err
		}
		payload, characterID, err := do(q, tx, session)
		if errors.Is(err, errRollHeld) {
			return nil // the roll waits for the player's answer about a die: nothing is written yet
		}
		if err != nil {
			return err
		}
		if characterID != "" {
			c.characterID = &characterID
		}
		ev = payload
		_, err = insertSceneEvent(ctx, c, kind, &m.UserID, &key, payload)
		return err
	})
	return ev, repeated, err
}

// errRollHeld is what a write's closure returns when its roll was kept for the player's answer
// about a Bardic Inspiration die: the change commits with what it kept, and writes no event.
var errRollHeld = errors.New("the roll is held")

// groupHold carries what a group check roll needs to be held for the answer about a die.
type groupHold struct {
	m     authz.Membership
	key   string
	raw   *playv1.RollGroupCheckRequest
	offer *playv1.OutsideInspirationOffer
}

type groupHoldKey struct{}

func groupHoldOf(ctx context.Context) *groupHold {
	g, _ := ctx.Value(groupHoldKey{}).(*groupHold)
	return g
}

// publishGroupCheckChanged tells the table that the group check changed: a hint with no
// content, read again by each as their own role.
func (s *Service) publishGroupCheckChanged(campaignID string) {
	s.hub.Publish(campaignID, live.Event{
		Audience: live.Audience{Everyone: true},
		Message: &playv1.WatchGameSessionResponse{Event: &playv1.WatchGameSessionResponse_GroupCheckChanged_{
			GroupCheckChanged: &playv1.WatchGameSessionResponse_GroupCheckChanged{},
		}},
	})
}

// RequestGroupCheck implements playv1connect.ContestServiceHandler.
func (s *Service) RequestGroupCheck(
	ctx context.Context,
	req *connect.Request[playv1.RequestGroupCheckRequest],
) (*connect.Response[playv1.RequestGroupCheckResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	key, err := parseKey(req.Msg.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}
	skill := req.Msg.GetSkillKey()
	if !isCheckKey(skill) || s.roster.SceneCheckName(skill) == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("skill_key must be a skill, an ability check or a saving throw"))
	}
	askedIDs := req.Msg.GetCharacterIds()
	for i, id := range askedIDs {
		if _, err := parseCombatID(id, "character"); err != nil {
			return nil, err
		}
		if slices.Contains(askedIDs[:i], id) {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("character_ids names a character twice"))
		}
	}
	dc := req.Msg.GetDc()
	if dc < 0 || dc > 40 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("dc must be 1 to 40, or 0 for none"))
	}
	var created playdb.GroupCheck
	ev, repeated, err := s.groupWrite(ctx, m, key, idem.Hash(req.Msg), eventGroupCheckRequested, func(q *playdb.Queries, tx pgx.Tx, session playdb.GameSession) (groupCheckEvent, string, error) {
		if _, err := q.GetOpenGroupCheck(ctx, session.ID); err == nil {
			return groupCheckEvent{}, "", errContest(playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_GROUP_CHECK_OPEN, "a group check is open already")
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return groupCheckEvent{}, "", fmt.Errorf("find the open group check: %w", err)
		}
		party, err := s.roster.CombatParty(ctx, tx, m.CampaignID)
		if err != nil {
			return groupCheckEvent{}, "", err
		}
		if len(party) == 0 {
			return groupCheckEvent{}, "", connect.NewError(connect.CodeInvalidArgument, errors.New("the campaign has no living player character to ask"))
		}
		// The characters asked: all of the party, or the ones named (each must be a living
		// player character of this campaign: never an NPC, a dead or an unknown one).
		asked := party
		if len(askedIDs) > 0 {
			asked = make([]link.Character, 0, len(askedIDs))
			for _, id := range askedIDs {
				i := slices.IndexFunc(party, func(c link.Character) bool { return c.ID == id })
				if i < 0 {
					return groupCheckEvent{}, "", connect.NewError(connect.CodeInvalidArgument, errors.New("character_ids must name living player characters of this campaign"))
				}
				asked = append(asked, party[i])
			}
		}
		// A group check is the default of asking everyone; asking some is judged one by one.
		isGroup := len(askedIDs) == 0
		if req.Msg.Group != nil {
			isGroup = req.Msg.GetGroup()
		}
		if isGroup && len(asked) < 2 && req.Msg.Group != nil {
			return groupCheckEvent{}, "", connect.NewError(connect.CodeInvalidArgument, errors.New("a group check needs at least two characters; ask one without group"))
		}
		var dcValue *int32
		if dc > 0 {
			dcValue = &dc
		}
		if created, err = q.InsertGroupCheck(ctx, playdb.InsertGroupCheckParams{
			GameSessionID: session.ID, SkillKey: skill, Dc: dcValue, ShowDc: req.Msg.GetShowDc(), IsGroup: isGroup, CreatedAt: s.now(),
		}); err != nil {
			return groupCheckEvent{}, "", fmt.Errorf("ask the group check: %w", err)
		}
		for _, ch := range asked {
			if err := q.InsertGroupCheckMember(ctx, playdb.InsertGroupCheckMemberParams{GroupCheckID: created.ID, CharacterID: ch.ID}); err != nil {
				return groupCheckEvent{}, "", fmt.Errorf("ask a character: %w", err)
			}
		}
		return groupCheckEvent{GroupCheckID: created.ID}, "", nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "ask a group check", err)
	}
	if !repeated {
		s.publishGroupCheckChanged(m.CampaignID)
	}
	view, err := s.groupCheckView(ctx, m, ev.GroupCheckID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.RequestGroupCheckResponse{GroupCheck: view}), nil
}

// isCheckKey says whether a key can be asked: a skill ("skill:stealth"), an ability check
// ("ability:str") or a saving throw ("save:con"). The roster checks the rest of the key.
func isCheckKey(key string) bool {
	return strings.HasPrefix(key, "skill:") || strings.HasPrefix(key, "ability:") || strings.HasPrefix(key, "save:")
}

// RollGroupCheck implements playv1connect.ContestServiceHandler.
func (s *Service) RollGroupCheck(
	ctx context.Context,
	req *connect.Request[playv1.RollGroupCheckRequest],
) (*connect.Response[playv1.RollGroupCheckResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RolePlayer)
	if err != nil {
		return nil, err
	}
	key, err := parseKey(req.Msg.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}
	checkID, err := parseCombatID(req.Msg.GetGroupCheckId(), "group check")
	if err != nil {
		return nil, err
	}
	in, err := parseCheckInput(req.Msg.GetRoll())
	if err != nil {
		return nil, err
	}
	hold := &groupHold{m: m, key: key, raw: req.Msg}
	ctx = context.WithValue(ctx, groupHoldKey{}, hold)
	_, repeated, err := s.groupWrite(ctx, m, key, idem.Hash(req.Msg), eventGroupCheckRolled, func(q *playdb.Queries, tx pgx.Tx, session playdb.GameSession) (groupCheckEvent, string, error) {
		hold.offer = nil
		who, has, err := s.myCharacter(ctx, tx, m)
		if err != nil {
			return groupCheckEvent{}, "", err
		}
		if !has {
			return groupCheckEvent{}, "", errContest(playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_NOT_IN_GROUP_CHECK, "you have no living character in this check")
		}
		force, err := s.dice.ForcedDice(ctx, tx, m.CampaignID, m.UserID)
		if err != nil {
			return groupCheckEvent{}, "", err
		}
		if force.refuses(in.inApp) {
			return groupCheckEvent{}, "", errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_WRONG_DICE_MODE, "this is not how the campaign has you roll your dice")
		}
		if err := s.rollMember(ctx, q, tx, m.CampaignID, session, checkID, who.ID, in, false); err != nil {
			return groupCheckEvent{}, "", err
		}
		if hold.offer != nil {
			return groupCheckEvent{}, "", errRollHeld
		}
		return groupCheckEvent{GroupCheckID: checkID, CharacterID: who.ID}, who.ID, nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "roll a group check", err)
	}
	if hold.offer != nil {
		return connect.NewResponse(&playv1.RollGroupCheckResponse{InspirationOffer: hold.offer}), nil
	}
	if !repeated {
		s.hub.Publish(m.CampaignID, live.Event{
			Audience: live.Audience{Master: true, UserID: m.UserID},
			Message: &playv1.WatchGameSessionResponse{Event: &playv1.WatchGameSessionResponse_GroupCheckChanged_{
				GroupCheckChanged: &playv1.WatchGameSessionResponse_GroupCheckChanged{},
			}},
		})
	}
	view, err := s.groupCheckView(ctx, m, checkID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.RollGroupCheckResponse{GroupCheck: view}), nil
}

// rollMember rolls a character's check for an open group check: its number for the skill
// and a normal d20 (the check has no combat, so no circumstance changes the roll).
func (s *Service) rollMember(ctx context.Context, q *playdb.Queries, tx pgx.Tx, campaignID string, session playdb.GameSession, checkID, characterID string, in checkInput, byMaster bool) error {
	check, err := q.GetGroupCheck(ctx, playdb.GetGroupCheckParams{GameSessionID: session.ID, ID: checkID})
	if errors.Is(err, pgx.ErrNoRows) {
		return connect.NewError(connect.CodeNotFound, errors.New("group check not found"))
	}
	if err != nil {
		return fmt.Errorf("find the group check: %w", err)
	}
	if check.Status != "open" {
		return errContest(playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_GROUP_CHECK_CLOSED, "the group check is closed")
	}
	members, err := q.ListGroupCheckMembers(ctx, check.ID)
	if err != nil {
		return fmt.Errorf("list the members: %w", err)
	}
	var member *playdb.GroupCheckMember
	for i := range members {
		if members[i].CharacterID == characterID {
			member = &members[i]
		}
	}
	switch {
	case member == nil:
		return errContest(playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_NOT_IN_GROUP_CHECK, "the character is not in this check")
	case member.RolledAt != nil:
		return errContest(playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_ALREADY_ANSWERED, "the character answered already")
	}
	options, err := s.roster.SceneOptions(ctx, tx, campaignID, characterID, []string{check.SkillKey})
	if err != nil {
		return err
	}
	bonus, known := 0, false
	if len(options) == 1 && options[0].Known {
		bonus, known = options[0].Bonus, true
	}
	// The same circumstances as any ability check: Poisoned, Frightened, exhaustion, the
	// Rage on Strength, armor on Stealth and a Help (SRD 5.1, "Group Checks" are ability checks).
	ability, isCheck := checkKey(check.SkillKey)
	cm, err := s.checkModeOf(ctx, tx, campaignID, session.ID, characterID, check.SkillKey, ability, isCheck)
	if err != nil {
		return err
	}
	// A character that holds a Bardic Inspiration die has the d20 kept until the player
	// answers whether to use it (a roll the master makes for the player waits for no one).
	var roll contestRoll
	gh := groupHoldOf(ctx)
	if gh != nil && !byMaster {
		step, err := s.outsideRoll(ctx, tx, q, gh.m, holdGroupCheck, characterID, gh.key, gh.raw, rollInput{inApp: in.inApp, typedFaces: in.faces}, bonus, cm.Mode, false)
		if err != nil {
			return err
		}
		if step.Held {
			gh.offer = step.Offer
			return nil
		}
		if step.Settled {
			roll = contestRoll{
				Skill: check.SkillKey, Faces: faces32(step.Roll.Faces), Modifier: clamp32(bonus, math.MinInt32, math.MaxInt32),
				Total: clamp32(step.Roll.Total, math.MinInt32, math.MaxInt32), Physical: step.Roll.Physical,
				Mode: checkModeKey(checkModeOfRoll(cm.Mode)), Unknown: !known,
			}
			if step.Bonus != nil {
				roll.BonusSides, roll.BonusFace = step.Bonus.Sides, step.Bonus.Face
			}
		}
	}
	if roll.Skill == "" {
		if roll, err = s.rollCheck(in, checkModeOfRoll(cm.Mode), bonus, check.SkillKey, nil, !known); err != nil {
			return err
		}
	}
	roll.Notes = notesOfSources(cm.Sources, cm.hide)
	c, err := s.openTx(ctx, combatTx{tx: tx, q: q, session: session, now: s.now(), characterID: &characterID})
	if err != nil {
		return err
	}
	if err := s.spendCheckHelps(ctx, c, cm.Helps); err != nil {
		return err
	}
	roll.ByMaster = byMaster
	stored, err := encodeRoll(roll)
	if err != nil {
		return err
	}
	now := s.now()
	if _, err := q.SetGroupCheckMemberRoll(ctx, playdb.SetGroupCheckMemberRollParams{
		GroupCheckID: check.ID, CharacterID: characterID, Roll: stored, RolledByMaster: byMaster, RolledAt: &now,
	}); err != nil {
		return fmt.Errorf("keep the roll: %w", err)
	}
	return nil
}

// RollForPlayer implements playv1connect.ContestServiceHandler.
func (s *Service) RollForPlayer(
	ctx context.Context,
	req *connect.Request[playv1.RollForPlayerRequest],
) (*connect.Response[playv1.RollForPlayerResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	key, err := parseKey(req.Msg.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}
	checkID, err := parseCombatID(req.Msg.GetGroupCheckId(), "group check")
	if err != nil {
		return nil, err
	}
	characterID, err := parseCombatID(req.Msg.GetCharacterId(), "character")
	if err != nil {
		return nil, err
	}
	in, err := parseCheckInput(req.Msg.GetRoll())
	if err != nil {
		return nil, err
	}
	_, repeated, err := s.groupWrite(ctx, m, key, idem.Hash(req.Msg), eventGroupCheckRolled, func(q *playdb.Queries, tx pgx.Tx, session playdb.GameSession) (groupCheckEvent, string, error) {
		if err := s.rollMember(ctx, q, tx, m.CampaignID, session, checkID, characterID, in, true); err != nil {
			return groupCheckEvent{}, "", err
		}
		return groupCheckEvent{GroupCheckID: checkID, CharacterID: characterID}, characterID, nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "roll for a player", err)
	}
	if !repeated {
		s.publishGroupCheckChanged(m.CampaignID)
	}
	view, err := s.groupCheckView(ctx, m, checkID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.RollForPlayerResponse{GroupCheck: view}), nil
}

// CloseGroupCheck implements playv1connect.ContestServiceHandler.
func (s *Service) CloseGroupCheck(
	ctx context.Context,
	req *connect.Request[playv1.CloseGroupCheckRequest],
) (*connect.Response[playv1.CloseGroupCheckResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	key, err := parseKey(req.Msg.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}
	checkID, err := parseCombatID(req.Msg.GetGroupCheckId(), "group check")
	if err != nil {
		return nil, err
	}
	_, repeated, err := s.groupWrite(ctx, m, key, idem.Hash(req.Msg), eventGroupCheckClosed, func(q *playdb.Queries, _ pgx.Tx, session playdb.GameSession) (groupCheckEvent, string, error) {
		check, err := q.GetGroupCheck(ctx, playdb.GetGroupCheckParams{GameSessionID: session.ID, ID: checkID})
		if errors.Is(err, pgx.ErrNoRows) {
			return groupCheckEvent{}, "", connect.NewError(connect.CodeNotFound, errors.New("group check not found"))
		}
		if err != nil {
			return groupCheckEvent{}, "", fmt.Errorf("find the group check: %w", err)
		}
		if check.Status != "open" {
			return groupCheckEvent{}, "", errContest(playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_GROUP_CHECK_CLOSED, "the group check is closed")
		}
		members, err := q.ListGroupCheckMembers(ctx, check.ID)
		if err != nil {
			return groupCheckEvent{}, "", fmt.Errorf("list the members: %w", err)
		}
		var verdict *bool
		if check.Dc != nil && check.IsGroup {
			passed := 0
			for _, mb := range members {
				if roll, err := decodeRoll(mb.Roll); err == nil && roll != nil && int(roll.Total) >= int(*check.Dc) {
					passed++
				}
			}
			verdict = new(combat.GroupCheckPasses(passed, len(members)))
		}
		if _, err := q.CloseGroupCheck(ctx, playdb.CloseGroupCheckParams{GameSessionID: session.ID, ID: check.ID, Passed: verdict, ClosedAt: new(s.now())}); err != nil {
			return groupCheckEvent{}, "", fmt.Errorf("close the group check: %w", err)
		}
		return groupCheckEvent{GroupCheckID: check.ID}, "", nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "close a group check", err)
	}
	if !repeated {
		s.publishGroupCheckChanged(m.CampaignID)
	}
	view, err := s.groupCheckView(ctx, m, checkID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.CloseGroupCheckResponse{GroupCheck: view}), nil
}

// GetGroupCheck implements playv1connect.ContestServiceHandler.
func (s *Service) GetGroupCheck(
	ctx context.Context,
	req *connect.Request[playv1.GetGroupCheckRequest],
) (*connect.Response[playv1.GetGroupCheckResponse], error) {
	m, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	view, err := s.latestGroupCheckView(ctx, m)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.GetGroupCheckResponse{GroupCheck: view}), nil
}

// latestGroupCheckView is the open group check as the caller reads it, or the latest one.
func (s *Service) latestGroupCheckView(ctx context.Context, m authz.Membership) (*playv1.GroupCheckView, error) {
	var view *playv1.GroupCheckView
	var check playdb.GroupCheck
	var members []playdb.GroupCheckMember
	err := db.ReadTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		session, err := openSessionWith(ctx, q, m.CampaignID)
		if err != nil {
			return err
		}
		check, err = q.GetOpenGroupCheck(ctx, session.ID)
		if errors.Is(err, pgx.ErrNoRows) {
			check, err = q.GetLatestGroupCheck(ctx, session.ID)
		}
		if errors.Is(err, pgx.ErrNoRows) {
			check = playdb.GroupCheck{}
			return nil
		}
		if err != nil {
			return fmt.Errorf("find the group check: %w", err)
		}
		members, err = q.ListGroupCheckMembers(ctx, check.ID)
		return err
	})
	if err != nil {
		return nil, s.dbError(ctx, "read the group check", err)
	}
	if check.ID == "" {
		return nil, nil
	}
	if view, err = s.buildGroupCheckView(ctx, m, check, members); err != nil {
		return nil, s.dbError(ctx, "read the group check", err)
	}
	return view, nil
}

// groupCheckView reads a group check back as the caller may read it.
func (s *Service) groupCheckView(ctx context.Context, m authz.Membership, checkID string) (*playv1.GroupCheckView, error) {
	var check playdb.GroupCheck
	var members []playdb.GroupCheckMember
	err := db.ReadTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		session, err := openSessionWith(ctx, q, m.CampaignID)
		if err != nil {
			return err
		}
		if check, err = q.GetGroupCheck(ctx, playdb.GetGroupCheckParams{GameSessionID: session.ID, ID: checkID}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return connect.NewError(connect.CodeNotFound, errors.New("group check not found"))
			}
			return fmt.Errorf("find the group check: %w", err)
		}
		members, err = q.ListGroupCheckMembers(ctx, check.ID)
		return err
	})
	if err != nil {
		return nil, s.dbError(ctx, "read the group check", err)
	}
	view, err := s.buildGroupCheckView(ctx, m, check, members)
	if err != nil {
		return nil, s.dbError(ctx, "read the group check", err)
	}
	return view, nil
}

// buildGroupCheckView is a group check as the caller may read it: the master everything;
// a player their own roll, and passed or failed only when the master shows the DC.
//
//nolint:gocyclo // one view for each audience, with the facts each may read in one place
func (s *Service) buildGroupCheckView(ctx context.Context, m authz.Membership, check playdb.GroupCheck, members []playdb.GroupCheckMember) (*playv1.GroupCheckView, error) {
	master := m.Role == authz.RoleMaster
	out := &playv1.GroupCheckView{
		Id: check.ID, SkillKey: check.SkillKey, SkillNamePt: s.roster.SceneCheckName(check.SkillKey), Open: check.Status == "open",
		ShowDc: check.ShowDc, CreatedAt: timestamppb.New(check.CreatedAt),
		Group: check.IsGroup, Save: strings.HasPrefix(check.SkillKey, "save:"),
	}
	if master {
		out.AskedCount = clamp32(len(members), 0, math.MaxInt32)
	}
	hasDC := check.Dc != nil
	if master && hasDC {
		out.Dc = *check.Dc
	}
	var mine link.Character
	var hasMine bool
	if !master {
		var err error
		if mine, hasMine, err = s.myCharacter(ctx, nil, m); err != nil {
			return nil, err
		}
		// A player who was not asked reads no group check at all (RN-10).
		if !hasMine || !slices.ContainsFunc(members, func(mb playdb.GroupCheckMember) bool { return mb.CharacterID == mine.ID }) {
			return nil, nil
		}
	}
	names := map[string]string{}
	ids := make([]string, 0, len(members))
	for _, mb := range members {
		ids = append(ids, mb.CharacterID)
	}
	if chars, err := s.roster.CombatCharacters(ctx, nil, m.CampaignID, ids); err == nil {
		for _, ch := range chars {
			names[ch.ID] = ch.Name
		}
	}
	passed := 0
	for _, mb := range members {
		roll, err := decodeRoll(mb.Roll)
		if err != nil {
			return nil, err
		}
		answered := roll != nil
		didPass := answered && hasDC && int(roll.Total) >= int(*check.Dc)
		if didPass {
			passed++
		}
		isMine := hasMine && mb.CharacterID == mine.ID
		if !master && !isMine {
			continue // a player reads their own roll only
		}
		view := &playv1.GroupCheckMemberView{CharacterId: mb.CharacterID, Name: names[mb.CharacterID], Answered: answered}
		if answered {
			view.Roll = roll.proto()
			if !master {
				view.Roll.BonusKnown = true
			}
			if hasDC && (master || check.ShowDc) {
				view.PassedKnown, view.Passed = true, didPass
			}
		}
		out.Members = append(out.Members, view)
		if isMine && !answered && check.Status == "open" {
			out.YouRoll = true
			if options, err := s.roster.SceneOptions(ctx, nil, m.CampaignID, mine.ID, []string{check.SkillKey}); err == nil && len(options) == 1 {
				// The mode the roll will have, so a player with physical dice types the right
				// number of faces.
				mode := playv1.RollModeKind_ROLL_MODE_KIND_NORMAL
				ability, isCheck := checkKey(check.SkillKey)
				if cm, err := s.checkModeOf(ctx, nil, m.CampaignID, check.GameSessionID, mine.ID, check.SkillKey, ability, isCheck); err == nil {
					mode = checkModeProto(checkModeOfRoll(cm.Mode))
				}
				out.YourOption = &playv1.CheckOption{
					Modifier: clamp32(options[0].Bonus, math.MinInt32, math.MaxInt32), Known: options[0].Known, Mode: mode,
				}
			}
		}
	}
	switch {
	case !check.IsGroup:
		// Each roll is judged alone: no count and no group verdict.
	case master:
		out.PassedCount = clamp32(passed, 0, math.MaxInt32)
		out.Needed = clamp32(combat.GroupCheckNeeded(len(members)), 0, math.MaxInt32)
		if hasDC {
			out.VerdictKnown, out.GroupPassed = true, combat.GroupCheckPasses(passed, len(members))
		}
	case check.Status != "open" && check.ShowDc && check.Passed != nil:
		out.VerdictKnown, out.GroupPassed = true, *check.Passed
	}
	return out, nil
}
