package play

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/PuraFome/meuRPG/backend/internal/platform/dice"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// rechargeDie is the sides of the die a Recharge action rolls (SRD 5.1, "Recharge X-Y").
const rechargeDie = 6

// rechargeAtTurnStart rolls a d6 for each Recharge action that a creature of ids spent, in the order of its
// stat block, when its turn starts: a face that recharges the action makes it available again. A defeated
// creature does not roll, an action that is available does not, and an action with uses a day or one that waits
// for a rest never does. Each roll is an event the master alone reads (the row is secret and the log line is
// his), and nothing is published for it: a turn start already tells everybody that the combat changed.
func (s *Service) rechargeAtTurnStart(ctx context.Context, c *combatTx, ids []string, round int32) error {
	cs, err := listCombatants(ctx, c)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if who, ok := combatantByID(cs, id); ok && s.mayRecharge(who) {
			if err := s.rechargeCreature(ctx, c, who, round); err != nil {
				return err
			}
		}
	}
	return nil
}

// mayRecharge says the combatant is a living creature that spent something that recharges.
func (s *Service) mayRecharge(who playdb.Combatant) bool {
	return !who.Defeated && (who.Kind == kindNPC || who.Kind == kindCreature) && strings.Contains(string(who.CreatureSpent), spentRechargePrefix)
}

// rechargeCreature rolls for each spent Recharge action of the creature, in its stat block's order.
func (s *Service) rechargeCreature(ctx context.Context, c *combatTx, who playdb.Combatant, round int32) error {
	plan, ok, err := s.monsterPlanOf(ctx, c, who)
	if err != nil || !ok {
		return err
	}
	for _, a := range plan.Actions {
		if a.Usage.Kind != rules.UsageRecharge || limitUsed(a, who) < 1 {
			continue
		}
		if err := s.rollRecharge(ctx, c, who.ID, a, round); err != nil {
			return err
		}
	}
	return nil
}

// monsterPlanOf reads the stat block plan of a combatant that has one.
func (s *Service) monsterPlanOf(ctx context.Context, c *combatTx, who playdb.Combatant) (*rules.MonsterPlan, bool, error) {
	key, err := s.monsterKeyOf(ctx, c, who)
	if err != nil || key == "" {
		return nil, false, err
	}
	content, err := s.contentOf(ctx, c)
	if err != nil {
		return nil, false, err
	}
	plan, ok := content.MonsterPlan(key)
	return plan, ok, nil
}

// rollRecharge rolls the d6 of one spent Recharge action, makes the action available on a face that
// recharges it, and writes the master's event.
func (s *Service) rollRecharge(ctx context.Context, c *combatTx, creatureID string, a rules.ActionPlan, round int32) error {
	roll, err := dice.Roll(s.roller, dice.Expr{Count: 1, Sides: rechargeDie})
	if err != nil {
		return fmt.Errorf("roll a Recharge: %w", err)
	}
	recharged := a.Usage.RechargedBy(roll.Total)
	if recharged {
		if _, err := bumpCreatureSpent(ctx, c.q, creatureID, limitKeyOf(a), -1); err != nil {
			return err
		}
	}
	body, err := json.Marshal(actionEvent{
		Round: round, Secret: true, Actor: creatureID, Key: a.Key,
		RechargeFace: clamp32(roll.Total, 1, rechargeDie), Recharged: recharged, RechargeName: nameOfPlan(a),
	})
	if err != nil {
		return fmt.Errorf("encode the Recharge event: %w", err)
	}
	_, err = insertRaw(ctx, c, eventRechargeRolled, &c.actorUserID, nil, body)
	return err
}
