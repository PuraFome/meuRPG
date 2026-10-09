package play

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/platform/idem"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/reaction"
)

// The concentration save (SRD 5.1, Duration): a creature that takes damage while it
// concentrates makes a Constitution saving throw, DC 10 or half the damage, and
// loses the concentration on a failure; damage from several sources is a save for
// each, and one attack or one spell is one source. At 0 hit points the concentration
// ends with no save. A save is a reaction window of its own kind: the damage that
// opened it stays applied, and the turn waits for the owner's answer (the app's die,
// a typed d20 or "Deixar o mestre rolar por mim") or the master's.

// concentrationEnd marks the event of a concentration that ended at 0 hit points, which
// the undo of the damage takes back with it.
const concentrationEnd = "concentration_end"

// damageLanded is what a damage did to its target, for what follows it.
type damageLanded struct {
	attacker string // the combatant that dealt it, empty for a trap
	key      string // the attack or spell
	pending  playdb.PendingDamage
	hit      damageHit
}

// afterDamage opens what a landed damage asks of its target: the end of a
// concentration at 0 hit points, or its saving throw. It runs in the transaction that
// applied the damage, once for each source of damage (a cast is one source).
func (s *Service) afterDamage(ctx context.Context, c *combatTx, target playdb.Combatant, d damageLanded) error {
	if d.pending.Healing || !d.hit.Applied || isCreature(target) {
		return nil
	}
	if err := s.hellishWindow(ctx, c, target, d); err != nil {
		return err
	}
	if err := s.effectDamageSaves(ctx, c, target, d); err != nil {
		return err
	}
	if target.ConcentrationSpell == nil {
		return nil
	}
	after := d.hit.After
	if after != nil && after.HP == 0 {
		// At 0 hit points the creature is incapacitated and loses the concentration
		// with no save (SRD, Duration).
		ev := actionEvent{Round: c.enc.Round, Secret: target.Hidden, Actor: target.ID, Reaction: &reactionEvent{Kind: concentrationEnd, Group: d.pending.ID}}
		if err := s.stopConcentrating(ctx, c, target, &ev); err != nil {
			return err
		}
		return insertEvent(ctx, c, eventConditionsSet, &c.actorUserID, nil, ev)
	}
	if d.hit.ConcentrationDC <= 0 {
		return nil
	}
	taken := takenBy(d.hit.Amount, d.hit.Before, d.hit.After)
	if d.pending.CastID != nil {
		all, err := c.q.ListCastPendingDamages(ctx, playdb.ListCastPendingDamagesParams{EncounterID: c.enc.ID, CastID: d.pending.CastID})
		if err != nil {
			return fmt.Errorf("list the cast's pending damage: %w", err)
		}
		taken = 0
		for _, o := range all {
			if o.TargetID == target.ID && !o.Healing && o.Status == pendingApplied {
				taken += num(o.Taken)
			}
		}
	}
	trigger := windowTrigger{Actor: d.attacker, Target: target.ID, Key: d.key, Spell: *target.ConcentrationSpell, Taken: taken, Pending: d.pending.ID}
	_, err := s.openWindows(ctx, c, newGroup(), nil, []windowSpec{{kind: reaction.Concentration, reactor: &target, trigger: trigger}})
	return err
}

// handSaveToMaster marks a concentration save as left to the master: the owner can no longer
// roll it, and the master's card asks for it.
func (s *Service) handSaveToMaster(ctx context.Context, c *combatTx, w playdb.ReactionWindow, t windowTrigger) error {
	t.Handed = true
	body, err := json.Marshal(t)
	if err != nil {
		return fmt.Errorf("encode the window's trigger: %w", err)
	}
	if _, err := c.q.SetReactionWindowStep(ctx, playdb.SetReactionWindowStepParams{ID: w.ID, Step: w.Step, Trigger: body}); err != nil {
		return fmt.Errorf("hand the save to the master: %w", err)
	}
	if c.enc, err = c.q.TouchEncounter(ctx, c.enc.ID); err != nil {
		return fmt.Errorf("touch the encounter: %w", err)
	}
	return nil
}

// concentrationRollOf reads how a concentration save is answered: the app's die, a
// physical one, "leave it to the master" or the master's "keep".
func concentrationRollOf(msg *playv1.ResolveConcentrationSaveRequest) (in rollInput, handOver, keep bool, err error) {
	bad := func(text string) error { return connect.NewError(connect.CodeInvalidArgument, errors.New(text)) }
	switch roll := msg.GetRoll().(type) {
	case *playv1.ResolveConcentrationSaveRequest_RollInApp:
		if !roll.RollInApp {
			return in, false, false, bad("roll_in_app must be true")
		}
		in.inApp = true
	case *playv1.ResolveConcentrationSaveRequest_D20Face:
		in.typed = int(roll.D20Face)
		if in.typed < 1 || in.typed > 20 {
			return in, false, false, bad("d20_face must be 1 to 20")
		}
	case *playv1.ResolveConcentrationSaveRequest_HandToMaster:
		if !roll.HandToMaster {
			return in, false, false, bad("hand_to_master must be true")
		}
		handOver = true
	case *playv1.ResolveConcentrationSaveRequest_Keep:
		if !roll.Keep {
			return in, false, false, bad("keep must be true")
		}
		keep = true
	default:
		return in, false, false, bad("set roll_in_app, d20_face, hand_to_master or keep")
	}
	return in, handOver, keep, nil
}

// ResolveConcentrationSave implements playv1connect.CombatServiceHandler.
func (s *Service) ResolveConcentrationSave(
	ctx context.Context,
	req *connect.Request[playv1.ResolveConcentrationSaveRequest],
) (*connect.Response[playv1.ResolveConcentrationSaveResponse], error) {
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
	in, handOver, keep, err := concentrationRollOf(req.Msg)
	if err != nil {
		return nil, err
	}

	var made actionEvent
	var result *playv1.ConcentrationSaveResult
	res, err := s.write(ctx, combatWrite{m: m, key: key, hash: idem.Hash(req.Msg), kind: eventConcentrationSaveRolled, encounterID: encID}, func(c *combatTx) (any, error) {
		result = nil
		if err := notEnded(c.enc); err != nil {
			return nil, err
		}
		w, reactor, err := s.windowToAnswer(ctx, c, m, winID)
		if err != nil {
			return nil, err
		}
		if w.Kind != string(reaction.Concentration) {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("this window is not a concentration save: use AnswerReaction"))
		}
		t := windowTriggerOf(w)
		master := m.Role == authz.RoleMaster
		switch {
		case keep && !master:
			return nil, errWindowDenied()
		case handOver && master:
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("the master rolls it: use roll_in_app or d20_face"))
		case t.Handed && !master:
			return nil, errWindowDenied() // the roll was left to the master
		}
		if handOver {
			return nil, s.handSaveToMaster(ctx, c, w, t)
		}
		dc := clamp32(reaction.ConcentrationDC(int(t.Taken)), 0, math.MaxInt32)
		ev := actionEvent{Round: c.enc.Round, Secret: reactor.Hidden, Actor: reactor.ID, Target: t.Actor, Key: t.Key, Pending: t.Pending}
		re := &reactionEvent{Kind: string(reaction.Concentration), Window: w.ID, ByMaster: master, Spell: t.Spell, SaveDC: dc}
		out := windowOutcome{ByMaster: master, Used: true}
		kept := true
		if !keep {
			if !master {
				if err := s.mustRollThisWay(ctx, c.tx, m, in); err != nil {
					return nil, err
				}
			}
			save, err := s.saveOf(ctx, c.tx, c.session.CampaignID, reactor, "con")
			if err != nil {
				return nil, err
			}
			face, roll, err := s.d20(in, save.Bonus)
			if err != nil {
				return nil, err
			}
			kept = roll.Total >= int(dc)
			re.SaveD20, re.SaveBonus, re.Saved, re.Kept = clamp32(face, 1, 20), clamp32(save.Bonus, math.MinInt32, math.MaxInt32), kept, kept
			ev.D20, ev.Modifier, ev.Total, ev.Physical = re.SaveD20, re.SaveBonus, clamp32(roll.Total, math.MinInt32, math.MaxInt32), roll.Physical
			result = &playv1.ConcentrationSaveResult{
				Save: diceRoll(1, 20, []int32{re.SaveD20}, re.SaveBonus, ev.Total, roll.Physical), Dc: dc, Kept: kept, SpellKey: t.Spell,
			}
		} else {
			re.Kept = true
			result = &playv1.ConcentrationSaveResult{Dc: dc, Kept: true, SpellKey: t.Spell}
		}
		if !kept {
			// The concentration ends, and the spell with it: the line of the log says
			// what stopped (written before the save's own event, as a cast's creatures are).
			stop := actionEvent{Round: c.enc.Round, Secret: reactor.Hidden, Actor: reactor.ID}
			if err := s.stopConcentrating(ctx, c, reactor, &stop); err != nil {
				return nil, err
			}
			if err := insertEvent(ctx, c, eventConditionsSet, &c.actorUserID, nil, stop); err != nil {
				return nil, err
			}
		}
		if _, err := s.closeWindow(ctx, c, w, windowAnswered, reaction.ReasonNone, out, ""); err != nil {
			return nil, err
		}
		ev.Reaction = re
		if c.enc, err = c.q.TouchEncounter(ctx, c.enc.ID); err != nil {
			return nil, fmt.Errorf("touch the encounter: %w", err)
		}
		c.characterID = &reactor.CharacterID
		made = ev
		return ev, nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "resolve a concentration save", err)
	}
	out, err := s.finish(ctx, m, res, func(ctx context.Context, d *encounterData) {
		s.publishEncounterChanged(ctx, m.CampaignID, d.enc)
		s.publishLogChanged(ctx, m.CampaignID, d.enc.ID, !made.Secret)
	})
	if err != nil {
		return nil, err
	}
	if result != nil {
		result.SpellNamePt = s.namesFor(ctx, m.CampaignID)(result.SpellKey)
	}
	return connect.NewResponse(&playv1.ResolveConcentrationSaveResponse{Encounter: out, Result: result}), nil
}

// windowToAnswer finds the window an answer is for, inside the change's transaction,
// and checks the caller may answer it and that it is its turn: the master answers any
// window; a player only the ones of their own character, and any other window id, real
// or not, is refused the same way (RN-10). A window that is gone or already answered
// is "not your turn to answer".
func (s *Service) windowToAnswer(ctx context.Context, c *combatTx, m authz.Membership, id string) (playdb.ReactionWindow, playdb.Combatant, error) {
	var none playdb.ReactionWindow
	master := m.Role == authz.RoleMaster
	w, err := c.q.GetReactionWindowForUpdate(ctx, playdb.GetReactionWindowForUpdateParams{EncounterID: c.enc.ID, ID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		if master {
			return none, playdb.Combatant{}, connect.NewError(connect.CodeNotFound, errors.New("reaction window not found"))
		}
		return none, playdb.Combatant{}, errWindowDenied()
	}
	if err != nil {
		return none, playdb.Combatant{}, fmt.Errorf("find the reaction window: %w", err)
	}
	var reactor playdb.Combatant
	if w.ReactorID != nil {
		cs, err := c.q.ListCombatants(ctx, c.enc.ID)
		if err != nil {
			return none, reactor, fmt.Errorf("list the combatants: %w", err)
		}
		if i := slices.IndexFunc(cs, func(x playdb.Combatant) bool { return x.ID == *w.ReactorID }); i >= 0 {
			reactor = cs[i]
		}
	}
	if !master && (w.ReactorID == nil || reactor.ID == "" || reactor.UserID == nil || *reactor.UserID != m.UserID || isCreature(reactor)) {
		return none, reactor, errWindowDenied()
	}
	if w.Status != windowOpen {
		return none, reactor, errNotYourTurnToAnswer()
	}
	if err := s.mustBeFirstOfGroup(ctx, c, w); err != nil {
		return none, reactor, err
	}
	return w, reactor, nil
}
