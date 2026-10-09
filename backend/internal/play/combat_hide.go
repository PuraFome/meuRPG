package play

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/platform/idem"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// Hide (SRD 5.1, "Hide", "Hiding", "Passive Perception", "Unseen Attackers and Targets").
//
// The Hide action is a Dexterity (Stealth) check. The master decides whether there is a
// place to hide, and what each creature notices: the hider's total against the creature's
// passive Perception (10 plus the Perception bonus, plus 5 for advantage, minus 5 for
// disadvantage); the SRD does not say what a tie does, and the app keeps the status quo:
// a tie keeps the hider noticed. The master may also say a creature sees the hider
// clearly (cover the app does not see), and the hider cannot hide from it.
//
// The result is one state per creature (combat_hiding): the hider is hidden while one
// creature has not noticed it. A hidden attacker has advantage on its attack roll against
// a creature that has not noticed it, and the attack gives its position away, hit or miss:
// the hiding ends for every creature (hide_ended). The player reads only "Você está
// escondida": never from whom, never who noticed, never anybody's Perception (RN-10,
// RN-20).

// observerView is what the app compares for one creature that could see a hider.
type observerView struct {
	passive int
	known   bool
	noticed bool
}

// observerOf is how a creature stands to a hider whose Stealth total is given: its passive
// Perception (the sheet's or the stat block's, with the circumstances that give its
// Perception advantage or disadvantage counted), and whether the total does not beat it.
// q and tx are the change's own (nil for a read).
func (s *Service) observerOf(ctx context.Context, tx pgx.Tx, campaignID string, enc playdb.Encounter, o playdb.Combatant, stealth int, cs []playdb.Combatant, q *playdb.Queries) (observerView, error) {
	n, err := s.numbersOf(ctx, tx, campaignID, o, skillPercept)
	if err != nil {
		return observerView{}, err
	}
	if q == nil {
		q = s.queries
	}
	notes, err := s.checkSources(ctx, &combatTx{q: q, enc: enc}, o, skillPercept, cs)
	if err != nil {
		return observerView{}, err
	}
	passive := passivePerceptionOf(n, notesMode(notes))
	return observerView{passive: passive, known: n.Known, noticed: combat.NoticesHider(stealth, passive)}, nil
}

// Hide implements playv1connect.ContestServiceHandler.
func (s *Service) Hide(
	ctx context.Context,
	req *connect.Request[playv1.HideRequest],
) (*connect.Response[playv1.HideResponse], error) {
	m, key, encID, err := contestCaller(ctx, req.Msg.GetCampaignId(), req.Msg.GetIdempotencyKey(), req.Msg.GetEncounterId())
	if err != nil {
		return nil, err
	}
	combatantID, err := parseCombatID(req.Msg.GetCombatantId(), "combatant")
	if err != nil {
		return nil, err
	}
	actionKey := req.Msg.GetActionKey()
	if actionKey == "" {
		actionKey = actionHide
	}
	in, err := parseCheckInput(req.Msg.GetRoll())
	if err != nil {
		return nil, err
	}

	var made actionEvent
	var vitals *playv1.CharacterVitals // the resource a feature spent
	res, err := s.write(ctx, combatWrite{m: m, key: key, hash: idem.Hash(req.Msg), kind: eventHideAttempted, encounterID: encID}, func(c *combatTx) (any, error) {
		made, vitals = actionEvent{}, nil
		if err := mustBeRunning(c); err != nil {
			return nil, err
		}
		cs, err := c.q.ListCombatants(ctx, c.enc.ID)
		if err != nil {
			return nil, fmt.Errorf("list the combatants: %w", err)
		}
		v := c.viewer(m, cs)
		who, err := findCombatant(cs, combatantID, v)
		if err != nil {
			return nil, err
		}
		if err := v.mayAct(who); err != nil {
			return nil, err
		}
		if err := s.mustActNow(ctx, c, who); err != nil {
			return nil, err
		}
		if !v.master {
			if err := s.mustRollThisWay(ctx, c.tx, m, in.rollInput()); err != nil {
				return nil, err
			}
		}
		if vitals, err = s.spendHide(ctx, c, m, v, who, actionKey); err != nil {
			return nil, err
		}
		roll, err := s.rollFor(ctx, c, who, skillStealth, in, cs)
		if err != nil {
			return nil, err
		}
		roll.ByMaster = v.master && !initiatorIsNPC(who)
		stored, err := encodeRoll(roll)
		if err != nil {
			return nil, err
		}
		attempt, err := c.q.InsertHideAttempt(ctx, playdb.InsertHideAttemptParams{
			EncounterID: c.enc.ID, HiderID: who.ID, Status: hidePending, Roll: stored, Round: c.enc.Round, CreatedAt: c.now,
		})
		if err != nil {
			return nil, fmt.Errorf("keep the hide attempt: %w", err)
		}
		if err := touch(ctx, c); err != nil {
			return nil, err
		}
		c.characterID = &who.CharacterID
		made = actionEvent{
			Round: c.enc.Round, Secret: who.Hidden, Actor: who.ID, Key: actionKey,
			Contest: &contestEvent{ContestID: attempt.ID, Line: contestLineHideTried},
		}
		return made, nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "hide", err)
	}
	ev, err := resultEvent(res, made)
	if err != nil {
		return nil, s.dbError(ctx, "read the hide", err)
	}
	out, err := s.finish(ctx, m, res, func(ctx context.Context, d *encounterData) {
		// The master decides the result: only he is told it waits for him. The hider's player
		// reads their own pending attempt.
		s.publishEncounterChangedFor(ctx, m.CampaignID, d, ev.Actor)
		s.publishLogChanged(ctx, m.CampaignID, d.enc.ID, false)
		s.publishVitals(m.CampaignID, vitals)
	})
	if err != nil {
		return nil, err
	}
	attempt, err := s.hideAttemptFor(ctx, m, res.encounterID, ev.Contest.ContestID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.HideResponse{Encounter: out, Attempt: attempt}), nil
}

// spendHide spends what the Hide costs the combatant: the action of the standard Hide, or
// the economy and the resource of a feature that gives Hide (the rogue's Cunning Action).
// It refuses a combatant that has no such action, or whose economy is spent.
func (s *Service) spendHide(ctx context.Context, c *combatTx, m authz.Membership, v combatViewer, who playdb.Combatant, actionKey string) (*playv1.CharacterVitals, error) {
	opts, err := s.optionsOf(ctx, c.tx, m.CampaignID, who)
	if err != nil {
		return nil, err
	}
	feature := actionKey != actionHide
	list := opts.GetStandardActions()
	if feature {
		list = opts.GetFeatureActions()
	}
	i := slices.IndexFunc(list, func(a *rulesv1.ActionOption) bool { return a.GetAction().GetKey() == actionKey })
	if i < 0 {
		return nil, errContest(playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_NOT_AVAILABLE, "the combatant has no such action")
	}
	option := list[i]
	var fa link.FeatureAction
	if feature {
		sheet, err := s.sheetOf(ctx, c.tx, m.CampaignID, who)
		if err != nil {
			return nil, err
		}
		fi := slices.IndexFunc(sheet.FeatureActions, func(a link.FeatureAction) bool { return a.Key == actionKey })
		if fi < 0 || sheet.FeatureActions[fi].Standard != actionHide {
			return nil, errContest(playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_NOT_AVAILABLE, "the action is not a Hide")
		}
		fa = sheet.FeatureActions[fi]
	}
	economy := option.GetAction().GetEconomy()
	if !option.GetEnabled() && !v.master {
		if feature {
			if err := featureError(option.GetReason(), v.master); err != nil {
				return nil, err
			}
		}
		if economy == rulesv1.ActionEconomy_ACTION_ECONOMY_BONUS_ACTION {
			return nil, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_BONUS_ACTION_USED, "the bonus action of this turn is used")
		}
		return nil, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_ACTION_USED, "the action of this turn is used")
	}
	if _, err := breakRun(ctx, c, who); err != nil {
		return nil, err
	}
	after := who
	switch economy {
	case rulesv1.ActionEconomy_ACTION_ECONOMY_BONUS_ACTION:
		after.BonusActionUsed = true
	case rulesv1.ActionEconomy_ACTION_ECONOMY_REACTION, rulesv1.ActionEconomy_ACTION_ECONOMY_FREE, rulesv1.ActionEconomy_ACTION_ECONOMY_MOVEMENT:
		return nil, errContest(playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_NOT_AVAILABLE, "Hide is an action or a bonus action")
	default:
		after.ActionUsed = true
	}
	if err := c.q.SetCombatantEconomy(ctx, playdb.SetCombatantEconomyParams{
		ID: who.ID, ActionUsed: after.ActionUsed, BonusActionUsed: after.BonusActionUsed, ReactionUsed: after.ReactionUsed, Dashed: after.Dashed,
	}); err != nil {
		return nil, fmt.Errorf("spend the action: %w", err)
	}
	if feature && fa.Resource != "" && !fa.Pool && who.Kind == kindPlayer {
		return s.spendResource(ctx, c, who.CharacterID, fa.Resource, 1)
	}
	return nil, nil
}

// ResolveHide implements playv1connect.ContestServiceHandler.
func (s *Service) ResolveHide(
	ctx context.Context,
	req *connect.Request[playv1.ResolveHideRequest],
) (*connect.Response[playv1.ResolveHideResponse], error) {
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
	attemptID, err := parseCombatID(req.Msg.GetAttemptId(), "hide attempt")
	if err != nil {
		return nil, err
	}
	refusal, err := trimReason(req.Msg.GetRefusal())
	if err != nil {
		return nil, err
	}
	var clearly []string
	for _, id := range req.Msg.GetSeesClearlyIds() {
		cid, err := parseCombatID(id, "combatant")
		if err != nil {
			return nil, err
		}
		if !slices.Contains(clearly, cid) {
			clearly = append(clearly, cid)
		}
	}
	refuse := req.Msg.GetRefuse()

	var made actionEvent
	res, err := s.write(ctx, combatWrite{m: m, key: key, hash: idem.Hash(req.Msg), kind: eventHideResolved, encounterID: encID}, func(c *combatTx) (any, error) {
		made = actionEvent{}
		if err := notEnded(c.enc); err != nil {
			return nil, err
		}
		attempt, err := c.q.GetHideAttempt(ctx, playdb.GetHideAttemptParams{EncounterID: c.enc.ID, ID: attemptID})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, connect.NewError(connect.CodeNotFound, errors.New("hide attempt not found"))
			}
			return nil, fmt.Errorf("find the hide attempt: %w", err)
		}
		if attempt.Status != hidePending {
			return nil, errContest(playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_HIDE_NOT_PENDING, "the hide attempt is answered already")
		}
		cs, err := c.q.ListCombatants(ctx, c.enc.ID)
		if err != nil {
			return nil, fmt.Errorf("list the combatants: %w", err)
		}
		hider, ok := combatantByID(cs, attempt.HiderID)
		if !ok {
			return nil, errContest(playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_HIDE_NOT_PENDING, "the hider left the combat")
		}
		roll, err := decodeRoll(attempt.Roll)
		if err != nil || roll == nil {
			return nil, fmt.Errorf("read the roll of hide attempt %s: %w", attempt.ID, err)
		}
		line := contestLineHideRefused
		status := hideRefused
		var reason *string
		if refuse {
			if refusal != "" {
				reason = &refusal
			}
		} else {
			status, line = hideApplied, contestLineHideApplied
			if err := c.q.DeleteHidingOfHider(ctx, hider.ID); err != nil {
				return nil, fmt.Errorf("forget the earlier hiding: %w", err)
			}
			for _, o := range cs {
				if o.ID == hider.ID || o.Defeated || !oppositeSide(hider, o) {
					continue
				}
				ob, err := s.observerOf(ctx, c.tx, m.CampaignID, c.enc, o, int(roll.Total), cs, c.q)
				if err != nil {
					return nil, err
				}
				noticed := ob.noticed || slices.Contains(clearly, o.ID)
				if err := c.q.UpsertHiding(ctx, playdb.UpsertHidingParams{
					HiderID: hider.ID, ObserverID: o.ID, EncounterID: c.enc.ID, Noticed: noticed,
					Total: roll.Total, Passive: clamp32(ob.passive, math.MinInt32, math.MaxInt32), CreatedAt: c.now,
				}); err != nil {
					return nil, fmt.Errorf("keep what a creature noticed: %w", err)
				}
			}
		}
		if _, err := c.q.SetHideAttemptDecision(ctx, playdb.SetHideAttemptDecisionParams{
			EncounterID: c.enc.ID, ID: attempt.ID, Status: status, Refusal: reason, ResolvedAt: &c.now,
		}); err != nil {
			return nil, fmt.Errorf("decide the hide attempt: %w", err)
		}
		if err := touch(ctx, c); err != nil {
			return nil, err
		}
		c.characterID = &hider.CharacterID
		made = actionEvent{
			Round: c.enc.Round, Secret: true, Actor: hider.ID,
			Contest: &contestEvent{ContestID: attempt.ID, Line: line},
		}
		return made, nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "decide a hide", err)
	}
	ev, err := resultEvent(res, made)
	if err != nil {
		return nil, s.dbError(ctx, "read the hide", err)
	}
	out, err := s.finish(ctx, m, res, func(ctx context.Context, d *encounterData) {
		s.publishEncounterChangedFor(ctx, m.CampaignID, d, ev.Actor)
		s.publishLogChanged(ctx, m.CampaignID, d.enc.ID, false)
	})
	if err != nil {
		return nil, err
	}
	attempt, err := s.hideAttemptFor(ctx, m, res.encounterID, attemptID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.ResolveHideResponse{Encounter: out, Attempt: attempt}), nil
}

// hideAttemptFor reads a hide attempt back as the caller may read it.
func (s *Service) hideAttemptFor(ctx context.Context, m authz.Membership, encounterID, attemptID string) (*playv1.HideAttemptView, error) {
	d, err := s.readContestData(ctx, m.CampaignID, encounterID)
	if err != nil {
		return nil, err
	}
	r, err := s.contestReaderFor(ctx, m, d)
	if err != nil {
		return nil, err
	}
	for _, a := range d.attempts {
		if a.ID == attemptID {
			view, err := r.hideAttemptView(ctx, a)
			if err != nil {
				return nil, s.dbError(ctx, "read the hide attempt", err)
			}
			return view, nil
		}
	}
	return nil, nil
}

// hideSnap is one creature's state before a hiding ended, for an undo.
type hideSnap struct {
	Observer string `json:"o"`
	Noticed  bool   `json:"n,omitempty"`
	Total    int32  `json:"t"`
	Passive  int32  `json:"p"`
}

// endHiding ends the hiding of a combatant for every creature: an attack gives its position
// away, hit or miss, and so does casting (SRD 5.1, "Unseen Attackers and Targets"). It
// writes hide_ended before the change's own event, and returns what the hiding was, for the
// undo of that change to give back. Nothing when the combatant was not hidden.
func (s *Service) endHiding(ctx context.Context, c *combatTx, who playdb.Combatant) ([]hideSnap, error) {
	rows, err := c.q.ListHiding(ctx, c.enc.ID)
	if err != nil {
		return nil, fmt.Errorf("list the hiding: %w", err)
	}
	var snaps []hideSnap
	for _, h := range rows {
		if h.HiderID == who.ID {
			snaps = append(snaps, hideSnap{Observer: h.ObserverID, Noticed: h.Noticed, Total: h.Total, Passive: h.Passive})
		}
	}
	if len(snaps) == 0 {
		return nil, nil
	}
	if err := c.q.DeleteHidingOfHider(ctx, who.ID); err != nil {
		return nil, fmt.Errorf("end the hiding: %w", err)
	}
	if err := insertEvent(ctx, c, eventHideEnded, &c.actorUserID, nil, actionEvent{Round: c.enc.Round, Secret: true, Actor: who.ID}); err != nil {
		return nil, err
	}
	return snaps, nil
}

// restoreHiding gives a hiding back: the undo of the attack or the spell that ended it.
func restoreHiding(ctx context.Context, c *combatTx, hider string, snaps []hideSnap) error {
	for _, h := range snaps {
		if err := c.q.UpsertHiding(ctx, playdb.UpsertHidingParams{
			HiderID: hider, ObserverID: h.Observer, EncounterID: c.enc.ID, Noticed: h.Noticed, Total: h.Total, Passive: h.Passive, CreatedAt: c.now,
		}); err != nil {
			return fmt.Errorf("give the hiding back: %w", err)
		}
	}
	return nil
}

// attackNotes are the circumstances of this module that give an attack roll advantage: a
// hidden attacker against a creature that has not noticed it ("Atacante não visto", SRD
// 5.1, "Unseen Attackers and Targets": when a creature can't see you, you have advantage
// on attack rolls against it) and the Help of an ally aimed at the target ("Ajuda de
// Orla", SRD 5.1, "Help"). It is the seam where the attack roll modes plug in: the sources
// that rest on the combat are worked out here, and the roll reads them.
func (s *Service) attackNotes(ctx context.Context, c *combatTx, attacker, target playdb.Combatant, cs []playdb.Combatant) ([]rollNote, error) {
	var out []rollNote
	rows, err := c.q.ListHiding(ctx, c.enc.ID)
	if err != nil {
		return nil, fmt.Errorf("list the hiding: %w", err)
	}
	if slices.ContainsFunc(rows, func(h playdb.CombatHiding) bool {
		return h.HiderID == attacker.ID && h.ObserverID == target.ID && !h.Noticed
	}) {
		out = append(out, rollNote{Kind: noteUnseen, Label: labelUnseen, Adv: true})
	}
	helps, err := s.liveHelps(ctx, c.q, c.enc, cs)
	if err != nil {
		return nil, err
	}
	for _, h := range helps {
		if h.Kind == helpAttack && h.AllyCharacterID == attacker.CharacterID && deref(h.TargetID) == target.ID {
			out = append(out, rollNote{Kind: noteHelp, Label: "Ajuda de " + labelOfCharacter(cs, h.HelperCharacterID), Adv: true})
			break
		}
	}
	return out, nil
}

// afterAttack is what an attack roll does to the contests' state: the Help aimed at the
// target is used by the first attack roll against it, and the hiding ends for every
// creature, hit or miss. It returns what the hiding was, for the attack's undo.
func (s *Service) afterAttack(ctx context.Context, c *combatTx, attacker, target playdb.Combatant, cs []playdb.Combatant, made *actionEvent) error {
	helps, err := s.liveHelps(ctx, c.q, c.enc, cs)
	if err != nil {
		return err
	}
	for _, h := range helps {
		if h.Kind == helpAttack && h.AllyCharacterID == attacker.CharacterID && deref(h.TargetID) == target.ID {
			if err := c.q.SetHelpConsumed(ctx, playdb.SetHelpConsumedParams{ID: h.ID, ConsumedAt: &c.now}); err != nil {
				return fmt.Errorf("use the help: %w", err)
			}
			made.HelpUsed = append(made.HelpUsed, h.ID)
		}
	}
	made.HidBefore, err = s.endHiding(ctx, c, attacker)
	return err
}

// restoreContestState gives back what an attack or a cast took from the contests' state:
// the hiding it ended and the Help it used (the master's undo).
func restoreContestState(ctx context.Context, c *combatTx, ev actionEvent) error {
	if err := restoreHiding(ctx, c, ev.Actor, ev.HidBefore); err != nil {
		return err
	}
	for _, id := range ev.HelpUsed {
		if err := c.q.SetHelpConsumed(ctx, playdb.SetHelpConsumedParams{ID: id, ConsumedAt: nil}); err != nil {
			return fmt.Errorf("give the help back: %w", err)
		}
	}
	return nil
}
