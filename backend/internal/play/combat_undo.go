package play

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// The master's "Desfazer" (MR-012, MR-014): one step back, a compensating
// event. Only the very last change of the session can be undone, and only
// when it is one of the actions below: any later event (a turn passing, a move,
// a correction of the vitals) puts what the action changed beyond a simple
// put-back, so there is nothing to undo then. The master's confirming a death
// is not one of them: the character is dead in the characters module. The events keep what was before
// (actionEvent: Before, ActionBefore...), and the undo writes it back.

// undoableKinds are the events UndoLastAction can take back.
var undoableKinds = []string{
	eventAttackRolled, eventDamageRolled, eventDamageApplied, eventDamageDiscarded, eventActionTaken, eventHitPointsAdjusted,
	eventSpellCast, eventReactionUsed, eventReactionDeclined, eventDeathSaveRolled, eventConditionsSet,
	eventCombatantMoved, eventWildShapeStarted, eventWildShapeEnded, eventFamiliarSight,
}

// recentEvents is how many of the session's latest events the search for the
// last action reads: each undo leaves two rows behind, so this allows dozens.
const recentEvents = 100

// lastAction finds the event an undo would take back among the session's
// latest events, newest first: the first one an undo did not take back
// already, if it belongs to this combat and is an action. Anything else
// being the last change leaves nothing to undo.
func lastAction(recent []playdb.ListRecentSessionEventsRow, encounterID string) (playdb.ListRecentSessionEventsRow, bool) {
	undone := map[string]bool{}
	for _, e := range recent {
		if e.Kind == eventActionUndone {
			if ev, err := readEvent(e.Payload); err == nil {
				undone[ev.Undone] = true
			}
			continue
		}
		if undone[e.ID] {
			continue
		}
		// The creatures a cast or an ended concentration made or dismissed are
		// written before the change's own event, which is what an undo acts on: they
		// never close the undo chain.
		if e.Kind == eventCreatureSummoned || e.Kind == eventCreatureDismissed {
			continue
		}
		// The offers a move made are written before the move's own event, the same
		// way: the undo acts on the move.
		if e.Kind == eventOpportunityOffered {
			continue
		}
		// A form that ended because the beast fell, and a sight that ended at a turn
		// start or by the player's hand, are no action to undo: the first is part of the
		// damage that did it (written before it), the others change nothing an undo
		// can put back.
		if e.Kind == eventWildShapeEnded || e.Kind == eventFamiliarSight {
			ev, err := readEvent(e.Payload)
			if err == nil && e.Kind == eventWildShapeEnded && ev.Reason == endedByDamage {
				continue
			}
			if err != nil || (e.Kind == eventFamiliarSight && ev.Sight != "start") {
				return playdb.ListRecentSessionEventsRow{}, false
			}
		}
		if slices.Contains(undoableKinds, e.Kind) && e.EncounterID != nil && *e.EncounterID == encounterID {
			// A move written before the undo knew moves says where it came from
			// nowhere: there is nothing to put back.
			if e.Kind == eventCombatantMoved {
				if ev, err := readEvent(e.Payload); err != nil || ev.From == nil {
					return playdb.ListRecentSessionEventsRow{}, false
				}
			}
			return e, true
		}
		return playdb.ListRecentSessionEventsRow{}, false
	}
	return playdb.ListRecentSessionEventsRow{}, false
}

// UndoLastAction implements playv1connect.CombatServiceHandler.
func (s *Service) UndoLastAction(
	ctx context.Context,
	req *connect.Request[playv1.UndoLastActionRequest],
) (*connect.Response[playv1.UndoLastActionResponse], error) {
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
	expected, err := parseKey(req.Msg.GetExpectedEventId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("expected_event_id must be a UUID"))
	}

	var made actionEvent
	var vitals []*playv1.CharacterVitals // the characters' vitals put back, if any
	res, err := s.write(ctx, combatWrite{m: m, key: key, kind: eventActionUndone, encounterID: encID}, func(c *combatTx) (any, error) {
		vitals = nil
		if err := notEnded(c.enc); err != nil {
			return nil, err
		}
		recent, err := c.q.ListRecentSessionEvents(ctx, playdb.ListRecentSessionEventsParams{GameSessionID: c.session.ID, Limit: recentEvents})
		if err != nil {
			return nil, fmt.Errorf("read the latest events: %w", err)
		}
		last, ok := lastAction(recent, c.enc.ID)
		if !ok {
			return nil, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NOTHING_TO_UNDO, "there is no action to undo")
		}
		if last.ID != expected {
			return nil, connect.NewError(connect.CodeAborted, errors.New("another action is the last now"))
		}
		ev, err := readEvent(last.Payload)
		if err != nil {
			return nil, err
		}
		if vitals, err = s.takeBack(ctx, c, last.Kind, ev); err != nil {
			return nil, err
		}
		if c.enc, err = c.q.TouchEncounter(ctx, c.enc.ID); err != nil {
			return nil, fmt.Errorf("touch the encounter: %w", err)
		}
		made = actionEvent{Round: c.enc.Round, Secret: ev.Secret, Actor: ev.Actor, Target: ev.Target, Undone: last.ID, UndoneKind: last.Kind}
		return made, nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "undo the last action", err)
	}
	ev, err := resultEvent(res, made)
	if err != nil {
		return nil, s.dbError(ctx, "read the undone action", err)
	}
	out, err := s.finish(ctx, m, res, func(d *encounterData) {
		s.publishEncounterChanged(m.CampaignID, d.enc)
		s.publishLogChanged(m.CampaignID, d.enc.ID, !ev.Secret)
		for _, vit := range vitals {
			s.publishVitals(m.CampaignID, vit)
		}
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.UndoLastActionResponse{Encounter: out}), nil
}

// takeBack puts back what the event changed, inside the undo's transaction.
// It returns the vitals of the characters it restored. A combatant or a pending
// damage that is gone (its character was deleted meanwhile) is skipped: there
// is nothing left to put back on it.
func (s *Service) takeBack(ctx context.Context, c *combatTx, kind string, ev actionEvent) ([]*playv1.CharacterVitals, error) {
	cs, err := c.q.ListCombatants(ctx, c.enc.ID)
	if err != nil {
		return nil, fmt.Errorf("list the combatants: %w", err)
	}
	find := func(id string) (playdb.Combatant, bool) {
		i := slices.IndexFunc(cs, func(o playdb.Combatant) bool { return o.ID == id })
		if i < 0 {
			return playdb.Combatant{}, false
		}
		return cs[i], true
	}
	setStatus := func(id, status string) error {
		_, err := c.q.SetPendingDamageStatus(ctx, playdb.SetPendingDamageStatusParams{ID: id, Status: status})
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("put back the pending damage: %w", err)
		}
		return nil
	}
	setEconomy := func(who playdb.Combatant, action, bonus, reaction, dashed bool) error {
		if err := c.q.SetCombatantEconomy(ctx, playdb.SetCombatantEconomyParams{ID: who.ID, ActionUsed: action, BonusActionUsed: bonus, ReactionUsed: reaction, Dashed: dashed}); err != nil {
			return fmt.Errorf("put back the economy: %w", err)
		}
		return nil
	}
	setRun := func(who playdb.Combatant, run int32) error {
		if run == 0 {
			return nil
		}
		if err := c.q.SetCombatantRun(ctx, playdb.SetCombatantRunParams{ID: who.ID, LastMoveDft: run}); err != nil {
			return fmt.Errorf("put back the running start: %w", err)
		}
		return nil
	}
	setAttacks := func(who playdb.Combatant, n int32) error {
		if err := c.q.SetCombatantAttacksMade(ctx, playdb.SetCombatantAttacksMadeParams{ID: who.ID, AttacksMade: n}); err != nil {
			return fmt.Errorf("put back the attacks: %w", err)
		}
		return nil
	}
	setHP := func(who playdb.Combatant, hp hpState) error {
		if err := c.q.SetCombatantHitPoints(ctx, playdb.SetCombatantHitPointsParams{ID: who.ID, HpCurrent: &hp.HP, HpTemp: &hp.Temp, Defeated: hp.Defeated}); err != nil {
			return fmt.Errorf("put back the hit points: %w", err)
		}
		return nil
	}
	setDeath := func(who playdb.Combatant, d *deathState) error {
		if d == nil {
			return nil
		}
		if err := c.q.SetCombatantDeathSaves(ctx, playdb.SetCombatantDeathSavesParams{ID: who.ID, DeathSuccesses: d.Successes, DeathFailures: d.Failures, DeathSaveRolled: d.Rolled, Defeated: d.Dead}); err != nil {
			return fmt.Errorf("put back the death saves: %w", err)
		}
		return nil
	}
	// A player's character's vitals put back: the maximum may have dropped
	// since (a level lost), and the vitals refuse a value above it, which would
	// leave this undo stuck for good, so the values are cut to the current
	// maximums.
	putVitals := func(who playdb.Combatant, hp hpState) (*playv1.CharacterVitals, error) {
		now, err := s.vitals.GetVitalsTx(ctx, c.tx, c.session.CampaignID, who.CharacterID)
		if err != nil {
			return nil, err
		}
		cur, temp := min(hp.HP, now.GetHitPointsMax()), min(hp.Temp, maxTempHitPoints)
		_, after, err := s.vitalsOf(ctx, c, who.CharacterID, &playv1.AdjustCharacterVitalsRequest{HitPointsCurrent: &cur, HitPointsTemporary: &temp})
		if err != nil {
			return nil, err
		}
		// The Wild Shape form as it was (MR-037): the beast and its hit points, or the
		// druid's own shape, with the combatant's numbers.
		have, want := after.GetWildShape(), hp.Shape
		if (have == nil) == (want == nil) && (want == nil || (have.GetBeastKey() == want.Beast && have.GetHitPointsCurrent() == want.HP)) {
			return after, nil
		}
		beast, beastHP := "", int32(0)
		if want != nil {
			beast, beastHP = want.Beast, want.HP
		}
		restored, body, err := s.vitals.SetWildShape(ctx, c.tx, c.session.CampaignID, who.CharacterID, beast, beastHP)
		if err != nil {
			return nil, err
		}
		return restored, applyBody(ctx, c, who, body)
	}
	var vitals []*playv1.CharacterVitals
	keep := func(v *playv1.CharacterVitals) {
		if v != nil {
			vitals = append(vitals, v)
		}
	}

	switch kind {
	case eventAttackRolled:
		// The action (or the reaction) comes back and the damage the hit opened goes
		// away.
		if who, ok := find(ev.Actor); ok {
			if ev.AsReaction {
				err = setEconomy(who, who.ActionUsed, who.BonusActionUsed, ev.ReactionBefore, who.Dashed)
			} else if err = setEconomy(who, ev.ActionBefore, who.BonusActionUsed, who.ReactionUsed, who.Dashed); err == nil {
				if err = setAttacks(who, ev.AttacksBefore); err == nil {
					err = setRun(who, ev.RunBefore)
				}
			}
			if err != nil {
				return nil, err
			}
		}
		if ev.OfferID != "" { // an opportunity attack: its offer waits again
			if err := offerWaits(ctx, c, ev.OfferID); err != nil {
				return nil, err
			}
		}
		if ev.Pending != "" {
			if err := c.q.DeletePendingDamage(ctx, ev.Pending); err != nil {
				return nil, fmt.Errorf("delete the pending damage: %w", err)
			}
		}
	case eventDamageRolled:
		// The damage waits to be rolled again; an NPC that took it is as it was, and
		// a character a heal reached has its hit points and death saves back. A
		// spell's roll settled every pending damage of the cast.
		if err := putBackMove(ctx, c, ev.Target, ev.ReturnedFrom); err != nil {
			return nil, err
		}
		hits := ev.Settled
		if len(hits) == 0 {
			hits = []damageHit{{Pending: ev.Pending, Target: ev.Target, Applied: ev.Applied, Before: ev.Before, DeathBefore: ev.DeathBefore}}
		}
		for _, h := range hits {
			if _, err := c.q.ClearPendingDamageRoll(ctx, h.Pending); err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return nil, fmt.Errorf("put back the pending damage: %w", err)
			}
			who, ok := find(h.Target)
			if !ok || !h.Applied || h.Before == nil {
				continue
			}
			if holdsHP(who) {
				if err := setHP(who, *h.Before); err != nil {
					return nil, err
				}
				continue
			}
			after, err := putVitals(who, *h.Before)
			if err != nil {
				return nil, err
			}
			keep(after)
			if err := setDeath(who, h.DeathBefore); err != nil {
				return nil, err
			}
		}
	case eventDamageApplied:
		// The character's vitals as they were, its death saves too, and the damage
		// waits for the master again.
		if err := putBackMove(ctx, c, ev.Target, ev.ReturnedFrom); err != nil {
			return nil, err
		}
		who, ok := find(ev.Target)
		if !ok || ev.Before == nil {
			break
		}
		after, err := putVitals(who, *ev.Before)
		if err != nil {
			return nil, err
		}
		keep(after)
		if err := setDeath(who, ev.DeathBefore); err != nil {
			return nil, err
		}
		if _, err := c.q.ClearPendingDamageApplied(ctx, ev.Pending); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("put back the pending damage: %w", err)
		}
	case eventDamageDiscarded:
		prev := ev.PrevStatus
		if prev != pendingAwaitingReaction && prev != pendingAwaitingRoll && prev != pendingRolled {
			prev = pendingRolled // never: only these can be discarded
		}
		return nil, setStatus(ev.Pending, prev)
	case eventCombatantMoved:
		// The square, the movement walked, the running start and the master's cover
		// mark are as they were: a move, a jump (the movement a jump spent comes
		// back) and the cover a move cleared. An event written before the tenths of
		// a foot has nothing to put back.
		who, ok := find(ev.Actor)
		if ev.From == nil {
			return nil, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NOTHING_TO_UNDO, "this move cannot be undone")
		}
		if !ok {
			break
		}
		var col, row *int32
		if ev.From.Placed {
			col, row = &ev.From.Col, &ev.From.Row
		}
		cover := ev.From.Cover
		if cover == "" {
			cover = "none"
		}
		if err := c.q.SetCombatantMove(ctx, playdb.SetCombatantMoveParams{
			ID: who.ID, GridCol: col, GridRow: row, MovementUsedFt: ev.From.UsedDFt / 10, MovementUsedDft: ev.From.UsedDFt, LastMoveDft: ev.From.LastDFt, CoverMark: cover,
		}); err != nil {
			return nil, fmt.Errorf("put back the move: %w", err)
		}
		// The offers the move made go with it. Only the move that is the last action
		// is undone, so they are all still pending: an answer comes after it, and has
		// to be undone first (which makes the offer wait again).
		if ev.MoveID != "" {
			if err := c.q.DeleteOpportunityOffersOfMove(ctx, ev.MoveID); err != nil {
				return nil, fmt.Errorf("take the opportunity offers away: %w", err)
			}
		}
	case eventActionTaken:
		who, ok := find(ev.Actor)
		if !ok {
			break
		}
		if err := setEconomy(who, ev.ActionBefore, ev.BonusBefore, ev.ReactionBefore, ev.DashedBefore); err != nil {
			return nil, err
		}
		if err := c.q.SetCombatantDisengaged(ctx, playdb.SetCombatantDisengagedParams{ID: who.ID, Disengaged: ev.DisengagedBefore}); err != nil {
			return nil, fmt.Errorf("put back the disengage: %w", err)
		}
		if err := setAttacks(who, ev.AttacksBefore); err != nil {
			return nil, err
		}
		if err := setRun(who, ev.RunBefore); err != nil {
			return nil, err
		}
		if ev.Resource != "" && who.Kind == kindPlayer {
			v, err := s.spendResource(ctx, c, who.CharacterID, ev.Resource, -1)
			if err != nil {
				return nil, err
			}
			keep(v)
		}
		if ev.Heal && ev.Before != nil {
			if holdsHP(who) {
				return vitals, setHP(who, *ev.Before)
			}
			v, err := putVitals(who, *ev.Before)
			if err != nil {
				return nil, err
			}
			keep(v)
			if err := setDeath(who, ev.DeathBefore); err != nil {
				return nil, err
			}
		}
	case eventHitPointsAdjusted:
		if who, ok := find(ev.Actor); ok && ev.Before != nil {
			return nil, setHP(who, *ev.Before)
		}
	case eventSpellCast:
		// The slot and the economy come back, the concentration is as it was, and
		// the pending damages the cast opened go away.
		who, ok := find(ev.Actor)
		if !ok {
			break
		}
		if err := setEconomy(who, ev.ActionBefore, ev.BonusBefore, ev.ReactionBefore, ev.DashedBefore); err != nil {
			return nil, err
		}
		if err := setRun(who, ev.RunBefore); err != nil {
			return nil, err
		}
		if ev.Slot != nil && who.Kind == kindPlayer {
			v, err := s.spendSlot(ctx, c, who.CharacterID, *ev.Slot, -1)
			if err != nil {
				return nil, err
			}
			keep(v)
		}
		if ev.Concentrate {
			var before *string
			if ev.ConcBefore != "" {
				before = &ev.ConcBefore
			}
			if err := c.q.SetCombatantConcentration(ctx, playdb.SetCombatantConcentrationParams{ID: who.ID, ConcentrationSpell: before}); err != nil {
				return nil, fmt.Errorf("put back the concentration: %w", err)
			}
		}
		// The creatures the cast made go away, and the ones it dismissed (the old
		// concentration's) are back with their group's initiative (MR-037).
		if len(ev.Created) > 0 {
			if err := s.removeCreatureCombatants(ctx, c, ev.Created); err != nil {
				return nil, err
			}
		}
		if len(ev.Dismissed) > 0 {
			if err := s.rejoinCreatures(ctx, c, ev.Dismissed); err != nil {
				return nil, err
			}
		}
		for _, h := range ev.Hits {
			if h.Pending != "" {
				if err := c.q.DeletePendingDamage(ctx, h.Pending); err != nil {
					return nil, fmt.Errorf("delete the pending damage: %w", err)
				}
			}
			// A spell that read hit points changed the targets at once: their hit
			// points, death saves and conditions come back.
			target, ok := find(h.Target)
			if !ok || h.Fx == "" {
				continue
			}
			if h.Restore != nil {
				if holdsHP(target) {
					err = setHP(target, *h.Restore)
				} else {
					var after *playv1.CharacterVitals
					if after, err = putVitals(target, *h.Restore); err == nil {
						keep(after)
					}
				}
				if err != nil {
					return nil, err
				}
			}
			if err := setDeath(target, h.DeathBefore); err != nil {
				return nil, err
			}
			if h.CondSet {
				if err := c.q.SetCombatantConditions(ctx, playdb.SetCombatantConditionsParams{ID: target.ID, Conditions: nonNil(h.CondBefore)}); err != nil {
					return nil, fmt.Errorf("put back the conditions: %w", err)
				}
			}
		}
	case eventReactionUsed:
		// The slot, the reaction and the armor class bonus come back, and the hit
		// waits for the reaction again.
		who, ok := find(ev.Actor)
		if !ok {
			break
		}
		if ev.Slot != nil {
			v, err := s.spendSlot(ctx, c, who.CharacterID, *ev.Slot, -1)
			if err != nil {
				return nil, err
			}
			keep(v)
		}
		if err := setEconomy(who, who.ActionUsed, who.BonusActionUsed, ev.ReactionBefore, who.Dashed); err != nil {
			return nil, err
		}
		if err := c.q.SetCombatantAcBonus(ctx, playdb.SetCombatantAcBonusParams{ID: who.ID, AcBonus: ev.ACBonusBefore}); err != nil {
			return nil, fmt.Errorf("put back the armor class bonus: %w", err)
		}
		if _, err := c.q.SetPendingDamageStatus(ctx, playdb.SetPendingDamageStatusParams{ID: ev.Pending, Status: pendingAwaitingReaction}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("put back the pending damage: %w", err)
		}
	case eventReactionDeclined:
		if ev.OfferID != "" { // an opportunity offer turned down or skipped: it waits again
			return nil, offerWaits(ctx, c, ev.OfferID)
		}
		return nil, setStatus(ev.Pending, pendingAwaitingReaction)
	case eventDeathSaveRolled:
		who, ok := find(ev.Actor)
		if !ok {
			break
		}
		if ev.Before != nil { // a natural 20 brought it back with 1 hit point
			after, err := putVitals(who, *ev.Before)
			if err != nil {
				return nil, err
			}
			keep(after)
		}
		return vitals, setDeath(who, ev.DeathBefore)
	case eventConditionsSet:
		who, ok := find(ev.Actor)
		if !ok {
			break
		}
		if ev.CondSet {
			if err := c.q.SetCombatantConditions(ctx, playdb.SetCombatantConditionsParams{ID: who.ID, Conditions: nonNil(ev.CondBefore)}); err != nil {
				return nil, fmt.Errorf("put back the conditions: %w", err)
			}
		}
		if ev.ConcEnded != "" {
			if err := c.q.SetCombatantConcentration(ctx, playdb.SetCombatantConcentrationParams{ID: who.ID, ConcentrationSpell: &ev.ConcEnded}); err != nil {
				return nil, fmt.Errorf("put back the concentration: %w", err)
			}
		}
		if len(ev.Dismissed) > 0 {
			if err := s.rejoinCreatures(ctx, c, ev.Dismissed); err != nil {
				return nil, err
			}
		}
	case eventWildShapeStarted, eventWildShapeEnded:
		if err := s.shapeUndo(ctx, c, kind, ev, find, keep); err != nil {
			return nil, err
		}
	case eventFamiliarSight:
		if err := s.sightUndo(ctx, c, ev, find, keep); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("event kind %q cannot be undone", kind)
	}
	return vitals, nil
}

// nonNil is the list, or an empty one: the conditions column is never NULL.
func nonNil(l []string) []string {
	if l == nil {
		return []string{}
	}
	return l
}
