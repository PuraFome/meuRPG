package play

import (
	"context"

	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// The creature type in the spells (SRD 5.1, RN-02, RN-10): Cure Wounds, Healing Word, Heal, Mass Cure Wounds, Spare the Dying
// and Blight do nothing to an undead or a construct, and Blight reads a plant. The type is the sheet's (Traits.CreatureType):
// a character and a creature with no type entry have none, and are as they ever were. The pure rules are rules/combat's.

// typeOfTarget is the SRD creature type of a combatant ("undead"), empty when it has none.
func (s *Service) typeOfTarget(ctx context.Context, c *combatTx, t playdb.Combatant) (string, error) {
	sheet, err := s.sheetOf(ctx, c.tx, c.session.CampaignID, t)
	if err != nil {
		return "", err
	}
	return sheet.Traits.CreatureType, nil
}

// noLifeMagic says why a spell of life does nothing to the target ("" when it may): the slot is spent anyway, a player is told
// only that nothing happened and the master reads the reason (whyNoLifeMagic).
func (s *Service) noLifeMagic(ctx context.Context, c *combatTx, t playdb.Combatant) (string, error) {
	typ, err := s.typeOfTarget(ctx, c, t)
	if err != nil || !combat.LifeMagicNoEffect(typ) {
		return "", err
	}
	return whyNoLifeMagic, nil
}

// damagesFor are the damages a spell opens for a target: its own, or, for a plant, each as the most its dice can give, which needs
// no roll (and is halved when the target saved).
func damagesFor(rule combat.TypeRule, parts []link.Dice) []link.Dice {
	if !rule.Plant {
		return parts
	}
	out := make([]link.Dice, len(parts))
	for i, d := range parts {
		out[i] = link.Dice{Bonus: combat.MaxRoll(d.Count, d.Sides, d.Bonus), DamageType: d.DamageType}
	}
	return out
}
