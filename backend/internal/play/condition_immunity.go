package play

import (
	"context"
	"slices"

	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// A character's sheet can make it immune to conditions (a table's condition_immunity effect): no
// spell, rider, trap or hold puts the condition on it. A creature's or an NPC's immunities come from
// its stat block and are read where the condition is applied (noEffectWhy); the immunity a lasting
// effect gives (Heroism) is read from the states (combat.ImmuneTo).

// sheetImmuneTo says the traits make the sheet immune to the condition.
func sheetImmuneTo(t link.Traits, condition string) bool {
	return slices.ContainsFunc(t.ConditionImmunities, func(i link.ConditionImmunity) bool {
		return slices.Contains(i.Conditions, condition)
	})
}

// immuneToCondition says the combatant's sheet makes it immune to the condition. A creature or an NPC
// with a stat block has no sheet immunity.
func (c *combatTx) immuneToCondition(ctx context.Context, who playdb.Combatant, condition string) (bool, error) {
	if isCreature(who) || condition == "" {
		return false, nil
	}
	sheet, err := c.svc.sheetOf(ctx, c.tx, c.session.CampaignID, who)
	if err != nil {
		return false, err
	}
	return sheetImmuneTo(sheet.Traits, condition), nil
}

// sheetImmunities are the conditions each of the combatants is immune to by its sheet, by combatant.
func (c *combatTx) sheetImmunities(ctx context.Context, targets []playdb.Combatant) (map[string]link.Traits, error) {
	out := make(map[string]link.Traits, len(targets))
	for _, t := range targets {
		if isCreature(t) {
			continue
		}
		sheet, err := c.svc.sheetOf(ctx, c.tx, c.session.CampaignID, t)
		if err != nil {
			return nil, err
		}
		out[t.ID] = sheet.Traits
	}
	return out, nil
}
