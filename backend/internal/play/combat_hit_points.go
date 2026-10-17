package play

import (
	"context"
	"errors"
	"fmt"
	"math"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/platform/conv"
	"github.com/PuraFome/meuRPG/backend/internal/platform/idem"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// The master's hand on an NPC's hit points (AdjustCombatantHitPoints). The call reads the request, runs the
// transaction (find the combatant, work out the hit points, write them) and answers with the encounter. The
// golden of combat_actions_rest_frozen_hp_test.go reads it.

// The ways the master changes the hit points of an NPC; the event keeps the word.
const (
	adjustDamage = "damage"
	adjustHeal   = "heal"
	adjustSet    = "set"
)

// adjustHitPointsCall is what AdjustCombatantHitPoints reads from its request, checked.
type adjustHitPointsCall struct {
	key, encounterID, combatantID string
	// mode and amount are the change of the hit points, and temp the temporary hit points to set; the
	// mode is empty when the request only sets the temporary ones.
	mode   string
	amount int32
	temp   *int32
}

// readAdjustHitPointsCall reads the request, in the order the answers are given: the ids, the change.
func readAdjustHitPointsCall(req *playv1.AdjustCombatantHitPointsRequest) (adjustHitPointsCall, error) {
	var call adjustHitPointsCall
	var err error
	if call.key, call.encounterID, err = parseCall(req.GetIdempotencyKey(), req.GetEncounterId()); err != nil {
		return call, err
	}
	if call.combatantID, err = parseCombatID(req.GetCombatantId(), "combatant"); err != nil {
		return call, err
	}
	call.mode, call.amount = adjustChangeOf(req)
	call.temp = req.HitPointsTemporary
	return call, call.check()
}

// adjustChangeOf is the way and the number the request changes the hit points by.
func adjustChangeOf(req *playv1.AdjustCombatantHitPointsRequest) (mode string, amount int32) {
	switch change := req.GetChange().(type) {
	case *playv1.AdjustCombatantHitPointsRequest_Damage:
		return adjustDamage, change.Damage
	case *playv1.AdjustCombatantHitPointsRequest_Heal:
		return adjustHeal, change.Heal
	case *playv1.AdjustCombatantHitPointsRequest_HitPoints:
		return adjustSet, change.HitPoints
	}
	return "", 0
}

// check refuses a request that changes nothing or by a number out of range.
func (call adjustHitPointsCall) check() error {
	switch {
	case call.mode == "" && call.temp == nil:
		return connect.NewError(connect.CodeInvalidArgument, errors.New("set damage, heal, hit_points or hit_points_temporary"))
	case call.mode != "" && (call.amount < 0 || call.amount > maxHitPointChange):
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("%s must be 0 to %d", call.mode, maxHitPointChange))
	case call.temp != nil && (*call.temp < 0 || *call.temp > maxTempHitPoints):
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("hit_points_temporary must be 0 to %d", maxTempHitPoints))
	}
	return nil
}

// AdjustCombatantHitPoints implements playv1connect.CombatServiceHandler.
func (s *Service) AdjustCombatantHitPoints(
	ctx context.Context,
	req *connect.Request[playv1.AdjustCombatantHitPointsRequest],
) (*connect.Response[playv1.AdjustCombatantHitPointsResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	call, err := readAdjustHitPointsCall(req.Msg)
	if err != nil {
		return nil, err
	}
	res, err := s.write(ctx, combatWrite{m: m, key: call.key, hash: idem.Hash(req.Msg), kind: eventHitPointsAdjusted, encounterID: call.encounterID}, func(c *combatTx) (any, error) {
		return s.adjustHitPointsWrite(ctx, c, m, call)
	})
	if err != nil {
		return nil, s.dbError(ctx, "adjust a combatant's hit points", err)
	}
	out, err := s.finish(ctx, m, res, func(ctx context.Context, d *encounterData) {
		s.publishEncounterChangedFor(ctx, m.CampaignID, d, call.combatantID)
		s.publishLogChanged(ctx, m.CampaignID, d.enc.ID, false) // the master's correction: his line only
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.AdjustCombatantHitPointsResponse{Encounter: out}), nil
}

// adjustHitPointsWrite is the transaction: the checks, the change and its event. It answers the event the
// write stores.
func (s *Service) adjustHitPointsWrite(ctx context.Context, c *combatTx, m authz.Membership, call adjustHitPointsCall) (any, error) {
	target, err := locateHitPointHolder(ctx, c, m, call.combatantID)
	if err != nil {
		return nil, err
	}
	after, err := hitPointsAfter(target, call)
	if err != nil {
		return nil, err
	}
	if err := c.q.SetCombatantHitPoints(ctx, playdb.SetCombatantHitPointsParams{ID: target.ID, HpCurrent: &after.HP, HpTemp: &after.Temp, Defeated: after.Defeated}); err != nil {
		return nil, fmt.Errorf("save the hit points: %w", err)
	}
	if err := touch(ctx, c); err != nil {
		return nil, err
	}
	before := hpOf(target)
	c.characterID = &target.CharacterID
	return actionEvent{
		Round: c.enc.Round, Secret: true, Actor: target.ID, Mode: call.mode, Amount: max(call.amount, 0),
		Delta:  after.HP - before.HP,
		Before: &before, After: &after,
	}, nil
}

// locateHitPointHolder finds the combatant, which has to keep its hit points in the combat.
func locateHitPointHolder(ctx context.Context, c *combatTx, m authz.Membership, combatantID string) (playdb.Combatant, error) {
	if err := notEnded(c.enc); err != nil {
		return playdb.Combatant{}, err
	}
	cs, err := listCombatants(ctx, c)
	if err != nil {
		return playdb.Combatant{}, err
	}
	target, err := findCombatant(cs, combatantID, viewerOf(m))
	if err != nil {
		return target, err
	}
	if !holdsHP(target) {
		return target, connect.NewError(connect.CodeInvalidArgument, errors.New("a player's character keeps its hit points in its vitals: use AdjustCharacterVitals"))
	}
	return target, nil
}

// hitPointsAfter works out the hit points the request leaves.
func hitPointsAfter(target playdb.Combatant, call adjustHitPointsCall) (hpState, error) {
	hp, tmp, hpMax := int(conv.OrZero(target.HpCurrent)), int(conv.OrZero(target.HpTemp)), int(conv.OrZero(target.HpMax))
	amount := int(call.amount)
	switch call.mode {
	case adjustDamage:
		dmg := combat.ApplyDamage(hp, tmp, amount)
		hp, tmp = dmg.HP, dmg.TempHP
	case adjustHeal:
		hp = combat.ApplyHeal(hp, hpMax, amount).HP
	case adjustSet:
		if amount > hpMax {
			return hpState{}, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("hit_points must be 0 to %d", hpMax))
		}
		hp = amount
	}
	if call.temp != nil {
		tmp = int(*call.temp)
	}
	return hpState{HP: clamp32(hp, 0, math.MaxInt32), Temp: clamp32(tmp, 0, math.MaxInt32), Defeated: hp == 0}, nil
}
