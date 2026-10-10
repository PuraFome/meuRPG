package play

import (
	"fmt"
	"math"
	"slices"

	"connectrpc.com/connect"

	"context"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/platform/idem"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// Standing up from Prone (SRD 5.1, "Being Prone" and "Movement and Position"): it costs an
// amount of movement equal to half the creature's speed, and a creature cannot stand up
// without that much movement left, for example when its speed is 0. While Prone, every foot
// of movement costs 1 extra foot (crawling): see crawls.

// The log line of a combatant standing up, an event of combatant_moved (so the master's
// undo puts the movement back) that carries it in Contest.Line.
const contestLineStoodUp = "stood_up"

// The log lines this file adds to the contests' table: contestLineProto lives in the
// contests' file, which another change owns, so the lines register themselves here.
func init() {
	contestLineProto[contestLineStoodUp] = playv1.ContestLogLine_CONTEST_LOG_LINE_STOOD_UP
	contestLineProto[contestLineHideRevealed] = playv1.ContestLogLine_CONTEST_LOG_LINE_HIDE_REVEALED
}

// isProne says whether the combatant carries the Prone condition.
func isProne(c playdb.Combatant) bool { return slices.Contains(c.Conditions, condProne) }

// crawls says the combatant moves by crawling: Prone, on foot. A creature that flies and
// is better in the air than on the ground (the speed it moves on) is not crawling.
func crawls(c playdb.Combatant) bool { return isProne(c) && !flies(c) }

// standUpCostDFt is what standing up costs the combatant, in tenths of a foot: half its
// speed, rounded down, and 0 when it is not Prone or has no speed (the call is then
// refused). The Dash does not count: it adds movement, not speed (SRD 5.1, "Dash").
func standUpCostDFt(c playdb.Combatant) int {
	if !isProne(c) {
		return 0
	}
	c.Dashed = false
	return speedDFt(c) / 2
}

// StandUp implements playv1connect.CombatServiceHandler.
func (s *Service) StandUp(
	ctx context.Context,
	req *connect.Request[playv1.StandUpRequest],
) (*connect.Response[playv1.StandUpResponse], error) {
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
	combID, err := parseCombatID(req.Msg.GetCombatantId(), "combatant")
	if err != nil {
		return nil, err
	}
	v := viewerOf(m)

	var made actionEvent
	var hidden bool
	res, err := s.write(ctx, combatWrite{m: m, key: key, hash: idem.Hash(req.Msg), kind: eventCombatantMoved, encounterID: encID}, func(c *combatTx) (any, error) {
		if err := notEnded(c.enc); err != nil {
			return nil, err
		}
		cs, err := c.q.ListCombatants(ctx, c.enc.ID)
		if err != nil {
			return nil, fmt.Errorf("list the combatants: %w", err)
		}
		v = c.viewer(m, cs)
		target, err := findCombatant(cs, combID, v)
		if err != nil {
			return nil, err
		}
		if err := v.mayAct(target); err != nil {
			return nil, err
		}
		if c.enc.Status != statusActive {
			return nil, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NOT_ACTIVE, "the combat is not running")
		}
		// Movement is a turn's: the master stands a creature up on its turn too.
		if !actsNow(c.enc, target) {
			return nil, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NOT_YOUR_TURN, "it is not your turn")
		}
		if !v.master {
			if err := s.mustNotBeDown(ctx, c.tx, m.CampaignID, target); err != nil {
				return nil, err
			}
			if err := s.mustNotBeSurprised(ctx, c, target); err != nil {
				return nil, err
			}
		}
		if err := s.mustNotHold(ctx, c); err != nil {
			return nil, err
		}
		if err := s.mustNotWait(ctx, c, target); err != nil {
			return nil, err
		}
		if !isProne(target) {
			return nil, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NOT_PRONE, "the combatant is not prone")
		}
		cost := standUpCostDFt(target)
		base := target
		base.Dashed = false
		if speedDFt(base) == 0 {
			return nil, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_CANNOT_STAND_UP, "the combatant cannot stand up: its speed is 0")
		}
		if left := movementLeftDFt(target); cost > left {
			return nil, tooFar(cost - left)
		}
		used := int(target.MovementUsedDft) + cost
		if err := c.q.SetCombatantMove(ctx, playdb.SetCombatantMoveParams{
			ID: target.ID, GridCol: target.GridCol, GridRow: target.GridRow,
			MovementUsedFt: clamp32(used/10, 0, math.MaxInt32), MovementUsedDft: clamp32(used, 0, math.MaxInt32),
			LastMoveDft: target.LastMoveDft, CoverMark: target.CoverMark, // nobody changed square
		}); err != nil {
			return nil, fmt.Errorf("spend the movement: %w", err)
		}
		if err := setConditionsOf(ctx, c, target, withoutCondition(target.Conditions, condProne)); err != nil {
			return nil, err
		}
		if err := touch(ctx, c); err != nil {
			return nil, err
		}
		c.characterID = &target.CharacterID
		hidden = target.Hidden
		// A combatant_moved event, as SpendMovement writes: the master's undo puts the
		// movement back from From, and the conditions from CondBefore. Col and Row are
		// where it stands, so the fog's reading of an undone move finds no change.
		made = actionEvent{
			Round: c.enc.Round, Secret: target.Hidden, Actor: target.ID, OnTurn: true, From: moveStateOf(target),
			CostDFt: clamp32(cost, 0, math.MaxInt32), CostFt: clamp32(cost/10, 0, math.MaxInt32),
			CondSet: true, CondBefore: slices.Clone(target.Conditions),
			Contest: &contestEvent{Line: contestLineStoodUp},
		}
		if placed(target) {
			made.Col, made.Row = *target.GridCol, *target.GridRow
		}
		return made, nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "stand up", err)
	}
	out, err := s.finish(ctx, m, res, func(ctx context.Context, d *encounterData) {
		s.publishEncounterChanged(ctx, m.CampaignID, d.enc)
		s.publishLogChanged(ctx, m.CampaignID, d.enc.ID, !hidden)
	})
	if err != nil {
		return nil, err
	}
	var left int32
	if i := slices.IndexFunc(out.GetCombatants(), func(c *playv1.Combatant) bool { return c.GetId() == combID }); i >= 0 {
		left = out.GetCombatants()[i].GetMovementLeftDft()
	}
	return connect.NewResponse(&playv1.StandUpResponse{Encounter: out, MovementLeftDft: left}), nil
}
