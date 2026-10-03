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
// put-back, so there is nothing to undo then. The events keep what was before
// (actionEvent: Before, ActionBefore...), and the undo writes it back.

// undoableKinds are the events UndoLastAction can take back.
var undoableKinds = []string{
	eventAttackRolled, eventDamageRolled, eventDamageApplied, eventDamageDiscarded, eventActionTaken, eventHitPointsAdjusted,
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
		if slices.Contains(undoableKinds, e.Kind) && e.EncounterID != nil && *e.EncounterID == encounterID {
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
	var vitals *playv1.CharacterVitals // a character's vitals put back, if any
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
		s.publishVitals(m.CampaignID, vitals)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.UndoLastActionResponse{Encounter: out}), nil
}

// takeBack puts back what the event changed, inside the undo's transaction.
// It returns the vitals of a character it restored. A combatant or a pending
// damage that is gone (its character was deleted meanwhile) is skipped: there
// is nothing left to put back on it.
func (s *Service) takeBack(ctx context.Context, c *combatTx, kind string, ev actionEvent) (*playv1.CharacterVitals, error) {
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
	setStatus := func(status string) error {
		_, err := c.q.SetPendingDamageStatus(ctx, playdb.SetPendingDamageStatusParams{ID: ev.Pending, Status: status})
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
	setHP := func(who playdb.Combatant, hp hpState) error {
		if err := c.q.SetCombatantHitPoints(ctx, playdb.SetCombatantHitPointsParams{ID: who.ID, HpCurrent: &hp.HP, HpTemp: &hp.Temp, Defeated: hp.Defeated}); err != nil {
			return fmt.Errorf("put back the hit points: %w", err)
		}
		return nil
	}

	switch kind {
	case eventAttackRolled:
		// The action comes back and the damage the hit opened goes away.
		if who, ok := find(ev.Actor); ok {
			if err := setEconomy(who, ev.ActionBefore, who.BonusActionUsed, who.ReactionUsed, who.Dashed); err != nil {
				return nil, err
			}
		}
		if ev.Pending != "" {
			if err := c.q.DeletePendingDamage(ctx, ev.Pending); err != nil {
				return nil, fmt.Errorf("delete the pending damage: %w", err)
			}
		}
	case eventDamageRolled:
		// The damage waits to be rolled again; an NPC that took it is as it was.
		if _, err := c.q.ClearPendingDamageRoll(ctx, ev.Pending); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("put back the pending damage: %w", err)
		}
		if who, ok := find(ev.Target); ok && ev.Applied && ev.Before != nil {
			if err := setHP(who, *ev.Before); err != nil {
				return nil, err
			}
		}
	case eventDamageApplied:
		// The character's vitals as they were, and the damage waits for the master again.
		who, ok := find(ev.Target)
		if !ok || ev.Before == nil {
			break
		}
		hp, temp := ev.Before.HP, ev.Before.Temp
		// The maximum may have dropped since (a level lost): the vitals refuse a
		// value above it, which would leave this undo stuck for good, so the
		// values put back are cut to the current maximums.
		now, err := s.vitals.GetVitalsTx(ctx, c.tx, c.session.CampaignID, who.CharacterID)
		if err != nil {
			return nil, err
		}
		hp, temp = min(hp, now.GetHitPointsMax()), min(temp, maxTempHitPoints)
		_, after, err := s.vitals.AdjustVitals(ctx, c.tx, c.session.CampaignID, who.CharacterID,
			&playv1.AdjustCharacterVitalsRequest{HitPointsCurrent: &hp, HitPointsTemporary: &temp})
		if err != nil {
			return nil, err
		}
		if err := setStatus(pendingRolled); err != nil {
			return nil, err
		}
		return after, nil
	case eventDamageDiscarded:
		prev := ev.PrevStatus
		if prev != pendingAwaitingRoll && prev != pendingRolled {
			prev = pendingRolled // never: only these two can be discarded
		}
		return nil, setStatus(prev)
	case eventActionTaken:
		if who, ok := find(ev.Actor); ok {
			return nil, setEconomy(who, ev.ActionBefore, ev.BonusBefore, ev.ReactionBefore, ev.DashedBefore)
		}
	case eventHitPointsAdjusted:
		if who, ok := find(ev.Actor); ok && ev.Before != nil {
			return nil, setHP(who, *ev.Before)
		}
	default:
		return nil, fmt.Errorf("event kind %q cannot be undone", kind)
	}
	return nil, nil
}
