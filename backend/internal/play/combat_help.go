package play

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/platform/idem"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// Help (SRD 5.1, "Help" and "Working Together").
//
// The Help action has two forms. Check: the ally has advantage on the next ability check
// it makes for the task, if it makes it before the help ends. Attack: the ally's first
// attack roll against a creature within 5 feet of the helper has advantage. The helper has
// to be able to attempt the task alone (the sheet has a number for the check); the master
// decides when a task admits help.
//
// A Help lasts until it is used, or until the end of the helper's next turn (the app's
// reading of "until the end of the helper's next turn"; the SRD's own words are "before
// the start of your next turn"). Outside a combat a Help would last until the master
// clears it; the combat is the only place that makes one today. A Help is a source of
// advantage every player reads ("Ajuda de Orla"); the one aimed at a creature a player does
// not see is not theirs to read. A creature's Help (a familiar) is the master's word: the
// app keeps Helps between characters and NPCs.

// liveHelps are the Helps that hold now in a combat: not used, not cleared, and not past the
// end of the helper's next turn.
func (s *Service) liveHelps(ctx context.Context, q *playdb.Queries, enc playdb.Encounter, cs []playdb.Combatant) ([]playdb.CombatHelp, error) {
	rows, err := q.ListLiveHelps(ctx, enc.GameSessionID)
	if err != nil {
		return nil, fmt.Errorf("list the helps: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	current := -1
	if enc.CurrentCombatantID != nil {
		if c, ok := combatantByID(cs, *enc.CurrentCombatantID); ok {
			current = int(c.OrderIndex)
		}
	}
	var out []playdb.CombatHelp
	for _, h := range rows {
		if h.EncounterID == nil {
			out = append(out, h) // outside a combat: until the master clears it
			continue
		}
		if *h.EncounterID != enc.ID {
			continue
		}
		helper, ok := combatantOfCharacter(cs, h.HelperCharacterID)
		if !ok {
			continue
		}
		if combat.HelpLastsThrough(int(num(h.CreatedRound)), int(enc.Round), current, int(helper.OrderIndex)) {
			out = append(out, h)
		}
	}
	return out, nil
}

// Help implements playv1connect.ContestServiceHandler.
//
//nolint:gocognit // the steps of one change in one closure, as RollAttack's are; a helper would only pass the transaction around
func (s *Service) Help(
	ctx context.Context,
	req *connect.Request[playv1.HelpRequest],
) (*connect.Response[playv1.HelpResponse], error) {
	m, key, encID, err := contestCaller(ctx, req.Msg.GetCampaignId(), req.Msg.GetIdempotencyKey(), req.Msg.GetEncounterId())
	if err != nil {
		return nil, err
	}
	helperID, err := parseCombatID(req.Msg.GetCombatantId(), "combatant")
	if err != nil {
		return nil, err
	}
	allyID, err := parseCombatID(req.Msg.GetAllyId(), "combatant")
	if err != nil {
		return nil, err
	}
	kind := req.Msg.GetKind()
	var targetID string
	taskKey := req.Msg.GetTaskKey()
	switch kind {
	case playv1.HelpKind_HELP_KIND_CHECK:
		if req.Msg.GetTargetId() != "" {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("target_id is for a Help with an attack"))
		}
		if taskKey == "" || len(taskKey) > 100 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_key must name the check of the task"))
		}
	case playv1.HelpKind_HELP_KIND_ATTACK:
		if taskKey != "" {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_key is for a Help with a check"))
		}
		if targetID, err = parseCombatID(req.Msg.GetTargetId(), "combatant"); err != nil {
			return nil, err
		}
	case playv1.HelpKind_HELP_KIND_UNSPECIFIED:
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("kind must be CHECK or ATTACK"))
	}

	var made actionEvent
	res, err := s.write(ctx, combatWrite{m: m, key: key, hash: idem.Hash(req.Msg), kind: eventHelpGiven, encounterID: encID}, func(c *combatTx) (any, error) {
		made = actionEvent{}
		if err := mustBeRunning(c); err != nil {
			return nil, err
		}
		cs, err := c.q.ListCombatants(ctx, c.enc.ID)
		if err != nil {
			return nil, fmt.Errorf("list the combatants: %w", err)
		}
		v := c.viewer(m, cs)
		helper, err := findCombatant(cs, helperID, v)
		if err != nil {
			return nil, err
		}
		if err := v.mayAct(helper); err != nil {
			return nil, err
		}
		if err := s.mustActNow(ctx, c, helper); err != nil {
			return nil, err
		}
		ally, err := findCombatant(cs, allyID, v)
		if err != nil {
			return nil, err
		}
		if ally.ID == helper.ID || ally.Defeated || ally.Side != helper.Side || isCreature(ally) || isCreature(helper) {
			return nil, errContest(playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_NOT_AN_ALLY, "that is not an ally the combatant can help")
		}
		// Help is an action of the sheet: it is there, and the action is free.
		opts, err := s.optionsOf(ctx, c.tx, m.CampaignID, helper)
		if err != nil {
			return nil, err
		}
		i := slices.IndexFunc(opts.GetStandardActions(), func(a *rulesv1.ActionOption) bool { return a.GetAction().GetKey() == actionHelp })
		switch {
		case i < 0:
			return nil, errContest(playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_NOT_AVAILABLE, "the combatant has no Help action")
		case !opts.GetStandardActions()[i].GetEnabled() && !v.master:
			return nil, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_ACTION_USED, "the action of this turn is used")
		}

		row := playdb.InsertHelpParams{
			GameSessionID: c.session.ID, EncounterID: &c.enc.ID, HelperCharacterID: helper.CharacterID, AllyCharacterID: ally.CharacterID,
			ExpiresRound: new(c.enc.Round + 1), CreatedRound: &c.enc.Round, CreatedAt: c.now,
		}
		switch kind {
		case playv1.HelpKind_HELP_KIND_CHECK:
			if err := s.mustBeATask(ctx, c, helper, taskKey); err != nil {
				return nil, err
			}
			row.Kind, row.Task = helpCheck, &taskKey
		case playv1.HelpKind_HELP_KIND_ATTACK:
			target, err := findCombatant(cs, targetID, v)
			if err != nil {
				return nil, err
			}
			if target.Defeated || target.Side == helper.Side || target.ID == helper.ID {
				return nil, errContest(playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_NOT_AN_ALLY, "the target is not a creature the ally can attack")
			}
			if !v.master && !isTheatre(c.enc) {
				if !placed(helper) || !placed(target) {
					return nil, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NOT_PLACED, "the combatants must be on the map")
				}
				if dist, _ := distanceFt(helper, target); dist > combat.MeleeReachFt {
					return nil, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_TARGET_OUT_OF_REACH, "the target is beyond 5 feet of the helper",
						func(b *playv1.EncounterBlocked) { b.MissingFt = dist - combat.MeleeReachFt })
				}
			}
			row.Kind, row.TargetID = helpAttack, &target.ID
		}
		if _, err := breakRun(ctx, c, helper); err != nil {
			return nil, err
		}
		if err := spendAction(ctx, c, helper, v.master); err != nil {
			return nil, err
		}
		inserted, err := c.q.InsertHelp(ctx, row)
		if err != nil {
			return nil, fmt.Errorf("keep the help: %w", err)
		}
		if err := touch(ctx, c); err != nil {
			return nil, err
		}
		c.characterID = &helper.CharacterID
		made = actionEvent{
			Round: c.enc.Round, Secret: secretOf(helper, ally), Actor: helper.ID, Target: ally.ID,
			Contest: &contestEvent{HelpID: inserted.ID, Line: contestLineHelped},
		}
		return made, nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "help", err)
	}
	ev, err := resultEvent(res, made)
	if err != nil {
		return nil, s.dbError(ctx, "read the help", err)
	}
	out, err := s.finish(ctx, m, res, func(ctx context.Context, d *encounterData) {
		s.publishEncounterChangedFor(ctx, m.CampaignID, d, ev.Actor, ev.Target)
		s.publishLogChanged(ctx, m.CampaignID, d.enc.ID, !ev.Secret)
	})
	if err != nil {
		return nil, err
	}
	view, err := s.helpFor(ctx, m, res.encounterID, ev.Contest.HelpID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.HelpResponse{Encounter: out, Help: view}), nil
}

// mustBeATask checks that a Help with a check names a check the app knows (a skill or an
// ability check, never a saving throw) that the helper could attempt alone: its sheet has a
// number for it.
func (s *Service) mustBeATask(ctx context.Context, c *combatTx, helper playdb.Combatant, key string) error {
	if !strings.HasPrefix(key, "skill:") && !strings.HasPrefix(key, "ability:") {
		return errContest(playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_TASK_NOT_AVAILABLE, "the task is not a skill or an ability check")
	}
	if s.roster.SceneCheckName(key) == "" {
		return errContest(playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_TASK_NOT_AVAILABLE, "the task is not a check of the SRD")
	}
	n, err := s.numbersOf(ctx, c.tx, c.session.CampaignID, helper, key)
	if err != nil {
		return err
	}
	if !n.Known {
		return errContest(playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_TASK_NOT_AVAILABLE, "the helper could not attempt the task alone")
	}
	return nil
}

// helpFor reads a Help back as the caller may read it.
func (s *Service) helpFor(ctx context.Context, m authz.Membership, encounterID, helpID string) (*playv1.HelpView, error) {
	d, err := s.readContestData(ctx, m.CampaignID, encounterID)
	if err != nil {
		return nil, err
	}
	r, err := s.contestReaderFor(ctx, m, d)
	if err != nil {
		return nil, err
	}
	for _, h := range r.helpViews() {
		if h.GetId() == helpID {
			return h, nil
		}
	}
	return nil, nil
}

// ClearHelp implements playv1connect.ContestServiceHandler.
func (s *Service) ClearHelp(
	ctx context.Context,
	req *connect.Request[playv1.ClearHelpRequest],
) (*connect.Response[playv1.ClearHelpResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	key, err := parseKey(req.Msg.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}
	encID, err := parseCombatID(req.Msg.GetEncounterId(), "encounter")
	if err != nil {
		return nil, err
	}
	helpID, err := parseCombatID(req.Msg.GetHelpId(), "help")
	if err != nil {
		return nil, err
	}
	var made actionEvent
	res, err := s.write(ctx, combatWrite{m: m, key: key, hash: idem.Hash(req.Msg), kind: eventHelpCleared, encounterID: encID}, func(c *combatTx) (any, error) {
		made = actionEvent{}
		if err := notEnded(c.enc); err != nil {
			return nil, err
		}
		help, err := c.q.GetHelp(ctx, playdb.GetHelpParams{GameSessionID: c.session.ID, ID: helpID})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("help not found"))
		}
		if err != nil {
			return nil, fmt.Errorf("find the help: %w", err)
		}
		if help.ConsumedAt != nil || help.ClearedAt != nil {
			return nil, errNotAwaiting()
		}
		if err := c.q.SetHelpCleared(ctx, playdb.SetHelpClearedParams{ID: help.ID, ClearedAt: &c.now}); err != nil {
			return nil, fmt.Errorf("clear the help: %w", err)
		}
		if err := touch(ctx, c); err != nil {
			return nil, err
		}
		made = actionEvent{Round: c.enc.Round, Contest: &contestEvent{HelpID: help.ID}}
		return made, nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "clear a help", err)
	}
	out, err := s.finish(ctx, m, res, s.changed(m.CampaignID))
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.ClearHelpResponse{Encounter: out}), nil
}
