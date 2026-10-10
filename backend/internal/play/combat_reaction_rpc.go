package play

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/platform/idem"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/reaction"
)

// answered is what answering a window did.
type answered struct {
	ev     actionEvent
	vitals []*playv1.CharacterVitals
	result *playv1.ReactionResult
	// kind is the session event kind when it is not reaction_answered (a Shield
	// that was used keeps reaction_used, which the undo knows).
	kind string
	// fill completes the result with what the replay of the held action made (what a
	// roll became), after the commit.
	fill func()
}

// AnswerReaction implements playv1connect.CombatServiceHandler.
func (s *Service) AnswerReaction(
	ctx context.Context,
	req *connect.Request[playv1.AnswerReactionRequest],
) (*connect.Response[playv1.AnswerReactionResponse], error) {
	m, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId())
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
	winID, err := parseCombatID(req.Msg.GetWindowId(), "window")
	if err != nil {
		return nil, err
	}
	choice := req.Msg.GetAnswer()
	if choice != playv1.ReactionChoice_REACTION_CHOICE_USE && choice != playv1.ReactionChoice_REACTION_CHOICE_PASS {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("answer must be USE or PASS"))
	}

	// An opportunity attack's offer is a window too: "Deixar passar" turns it down; the
	// attack itself is RollAttack's (opportunity_offer_id).
	if _, err := s.queries.GetOpportunityOffer(ctx, playdb.GetOpportunityOfferParams{EncounterID: encID, ID: winID}); err == nil {
		if choice == playv1.ReactionChoice_REACTION_CHOICE_USE {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("an opportunity attack is made with RollAttack and opportunity_offer_id"))
		}
		out, err := s.answerOffer(ctx, m, req.Msg.GetEncounterId(), winID, req.Msg.GetIdempotencyKey(), offerDeclined)
		if err != nil {
			return nil, err
		}
		return connect.NewResponse(&playv1.AnswerReactionResponse{Encounter: out, Result: &playv1.ReactionResult{Kind: playv1.ReactionKind_REACTION_KIND_OPPORTUNITY, ByMaster: m.Role == authz.RoleMaster}}), nil
	}

	var got answered
	res, err := s.write(ctx, combatWrite{m: m, key: key, hash: idem.Hash(req.Msg), kind: eventReactionAnswered, altKind: eventReactionUsed, encounterID: encID}, func(c *combatTx) (any, error) {
		got = answered{}
		if err := notEnded(c.enc); err != nil {
			return nil, err
		}
		w, reactor, err := s.windowToAnswer(ctx, c, m, winID)
		if err != nil {
			return nil, err
		}
		use := choice == playv1.ReactionChoice_REACTION_CHOICE_USE
		switch reaction.Kind(w.Kind) {
		case reaction.Contest:
			return nil, errContestWindow()
		case reaction.Shield:
			err = s.answerShield(ctx, c, w, reactor, use, req.Msg, &got)
		case reaction.MasterCheck:
			err = s.answerMasterCheck(ctx, c, m, w, use, &got)
		case reaction.Concentration:
			err = s.answerConcentration(ctx, c, m, w, reactor, use, &got)
		default:
			err = s.answerMore(ctx, c, m, w, reactor, use, req.Msg, &got)
		}
		if err != nil {
			return nil, err
		}
		if got.kind != "" {
			c.kind = got.kind
		}
		if c.enc, err = c.q.TouchEncounter(ctx, c.enc.ID); err != nil {
			return nil, fmt.Errorf("touch the encounter: %w", err)
		}
		got.ev.Round = c.enc.Round
		return got.ev, nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "answer a reaction", err)
	}
	ev, err := resultEvent(res, got.ev)
	if err != nil {
		return nil, s.dbError(ctx, "read the answer", err)
	}
	out, err := s.finish(ctx, m, res, func(ctx context.Context, d *encounterData) {
		s.publishEncounterChanged(ctx, m.CampaignID, d.enc)
		s.publishLogChanged(ctx, m.CampaignID, d.enc.ID, !ev.Secret && !ev.AttackerHidden)
		for _, v := range got.vitals {
			s.publishVitals(m.CampaignID, v)
		}
	})
	if err != nil {
		return nil, err
	}
	if got.fill != nil {
		got.fill()
	}
	return connect.NewResponse(&playv1.AnswerReactionResponse{Encounter: out, Result: got.result}), nil
}

// answerShield answers a Shield window: "Usar Escudo Arcano" casts it, "Deixar
// passar" lets the hit go on.
func (s *Service) answerShield(ctx context.Context, c *combatTx, w playdb.ReactionWindow, reactor playdb.Combatant, use bool, req *playv1.AnswerReactionRequest, got *answered) error {
	if w.PendingDamageID == nil {
		return fmt.Errorf("the shield window %s holds no hit", w.ID)
	}
	p, err := c.q.GetPendingDamage(ctx, playdb.GetPendingDamageParams{EncounterID: c.enc.ID, ID: *w.PendingDamageID})
	if err != nil {
		return fmt.Errorf("find the pending damage: %w", err)
	}
	cs, err := c.q.ListCombatants(ctx, c.enc.ID)
	if err != nil {
		return fmt.Errorf("list the combatants: %w", err)
	}
	attacker, _ := findCombatant(cs, deref(p.AttackerID), combatViewer{master: true})
	if !use {
		ev, err := s.passShield(ctx, c, w, p, reactor)
		got.ev = ev
		got.result = &playv1.ReactionResult{Kind: playv1.ReactionKind_REACTION_KIND_SHIELD, ByMaster: c.master}
		return err
	}
	ev, vitals, err := s.useShield(ctx, c, w, p, attacker, reactor, req.GetSlot())
	if err != nil {
		return err
	}
	got.ev, got.kind = ev, eventReactionUsed
	if vitals != nil {
		got.vitals = append(got.vitals, vitals)
	}
	got.result = &playv1.ReactionResult{
		Kind: playv1.ReactionKind_REACTION_KIND_SHIELD, ByMaster: c.master, Used: true,
		Result: &playv1.ReactionResult_Shield{Shield: &playv1.ShieldResult{Stopped: ev.Stopped}},
	}
	return nil
}

// answerMasterCheck answers the master's check of the table rule "Sempre": only the
// master, and only "Sem reação" (an enemy that has a reaction has a window of its own).
func (s *Service) answerMasterCheck(ctx context.Context, c *combatTx, m authz.Membership, w playdb.ReactionWindow, use bool, got *answered) error {
	if m.Role != authz.RoleMaster {
		return errWindowDenied()
	}
	if use {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("an enemy that can react has its own window: answer that one"))
	}
	if _, err := s.closeWindow(ctx, c, w, windowAnswered, reaction.ReasonNone, windowOutcome{ByMaster: true}, ""); err != nil {
		return err
	}
	t := windowTriggerOf(w)
	got.ev = actionEvent{Round: c.enc.Round, Secret: true, Actor: t.Actor, Reaction: &reactionEvent{Kind: string(reaction.MasterCheck), Window: w.ID, ByMaster: true}}
	got.result = &playv1.ReactionResult{Kind: playv1.ReactionKind_REACTION_KIND_MASTER_CHECK, ByMaster: true}
	return nil
}

// answerConcentration is the master keeping a concentration without a roll through
// AnswerReaction; the saves themselves go through ResolveConcentrationSave.
func (s *Service) answerConcentration(_ context.Context, _ *combatTx, _ authz.Membership, _ playdb.ReactionWindow, _ playdb.Combatant, _ bool, _ *answered) error {
	return connect.NewError(connect.CodeInvalidArgument, errors.New("a concentration save is answered with ResolveConcentrationSave"))
}

// answerMore answers the reactions that are not Shield, the master's check or the
// concentration save.
func (s *Service) answerMore(ctx context.Context, c *combatTx, m authz.Membership, w playdb.ReactionWindow, reactor playdb.Combatant, use bool, req *playv1.AnswerReactionRequest, got *answered) error {
	switch reaction.Kind(w.Kind) {
	case reaction.CounterspellKind:
		return s.answerCounterspell(ctx, c, w, reactor, use, req, got)
	case reaction.UncannyDodgeKind:
		return s.answerUncannyDodge(ctx, c, w, reactor, use, got)
	case reaction.DeflectKind:
		return s.answerDeflect(ctx, c, w, reactor, use, req, got)
	case reaction.CuttingWords:
		return s.answerCuttingWords(ctx, c, w, reactor, use, req, got)
	case reaction.HellishRebukeKind:
		return s.answerHellishRebuke(ctx, c, m, w, reactor, use, req, got)
	case reaction.FeatherFall:
		return s.answerFeatherFall(ctx, c, w, reactor, use, req, got)
	}
	return connect.NewError(connect.CodeUnimplemented, fmt.Errorf("the reaction %s is not built yet", w.Kind))
}
