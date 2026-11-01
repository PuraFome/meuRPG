package play

import (
	"context"
	"fmt"

	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// Damage dice rolled again (the table's reroll effects that cover "damage"): the SRD's reroll means the die is rolled
// again and the new roll is used. The app applies it, the player is not asked, so it is not a held roll. One use
// (Cost units of the rule's resource) covers one damage roll: every die of the roll that shows AtMost or lower is
// rolled once more. Only the dice the app rolled can be: a typed sum has no faces, and the damage that waits for a
// reaction window keeps the dice it was rolled with (the reroll is not applied to the replay of a held roll).

// damageRerollUse is the use of a reroll rule a damage roll spent, for the undo.
type damageRerollUse struct {
	Resource string `json:"resource"`
	Cost     int32  `json:"cost"`
}

// damageRerollRules are the reroll rules of the attacker that cover a die of damage, in the sheet's order.
func damageRerollRules(t link.Traits) []link.RerollRule {
	var out []link.RerollRule
	for _, r := range t.RerollRules {
		if r.AtMost > 0 && r.Resource != "" && r.Cost > 0 {
			for _, a := range r.Applies {
				if a == rules.RollDamage {
					out = append(out, r)
					break
				}
			}
		}
	}
	return out
}

// rerollDamage rolls again the low dice of the damage just rolled when the attacker is a player's character with a
// reroll rule that covers it and enough uses of its resource, and spends the use. It runs after the reaction
// windows were decided (a held roll is replayed as it was rolled) and before the adjustments and the event.
func (f *damageFlow) rerollDamage(ctx context.Context) error {
	if f.c.replay != nil || f.p.Healing || !f.call.in.inApp || !strokeMayApply(f.attacker) {
		return nil
	}
	if f.pr == nil && f.expr.Count == 0 {
		return nil
	}
	sheet, err := f.s.sheetOf(ctx, f.c.tx, f.m.CampaignID, f.attacker)
	if err != nil {
		return err
	}
	rs := damageRerollRules(sheet.Traits)
	if len(rs) == 0 {
		return nil
	}
	vit, err := f.s.vitals.GetVitalsTx(ctx, f.c.tx, f.m.CampaignID, f.attacker.CharacterID)
	if err != nil {
		return err
	}
	left := usesLeftOf(vit)
	for _, r := range rs { // one rule for the roll: the first with enough uses
		if int(left[r.Resource]) < r.Cost {
			continue
		}
		changed, err := f.rerollLowDice(r.AtMost)
		if err != nil || !changed {
			return err
		}
		v, err := f.s.spendResource(ctx, f.c, f.attacker.CharacterID, r.Resource, int32(r.Cost)) //nolint:gosec // validated against the resource's maximum
		if err != nil {
			return err
		}
		f.out.vitals = append(f.out.vitals, v)
		f.reroll = &damageRerollUse{Resource: r.Resource, Cost: int32(r.Cost)} //nolint:gosec // see above
		return nil
	}
	return nil
}

// rerollLowDice rolls once more each die of the roll that shows atMost or lower and keeps the new face. It says
// whether any die was rolled again.
func (f *damageFlow) rerollLowDice(atMost int) (bool, error) {
	if f.pr != nil {
		changed, err := f.rerollPartsDice(atMost)
		if changed {
			f.roll = f.pr.legacy()
		}
		return changed, err
	}
	changed := false
	for i, face := range f.roll.Faces {
		if face > atMost {
			continue
		}
		n, err := f.s.roller.Roll(f.expr.Sides)
		if err != nil {
			return false, fmt.Errorf("reroll the damage: %w", err)
		}
		f.roll.Total += n - face
		f.roll.Faces[i], changed = n, true
	}
	return changed, nil
}

// rerollPartsDice does the same for the dice of the parts that count (and the conditional ones the target's kind
// keeps out, which a player reads in the total), keeping the faces replaced in the part's Rerolled list.
func (f *damageFlow) rerollPartsDice(atMost int) (bool, error) {
	changed := false
	for i := range f.pr.rolls {
		pr := &f.pr.rolls[i]
		if (!pr.Counted && !pr.Conditional) || pr.Physical {
			continue
		}
		for j, face := range pr.Faces {
			if int(face) > atMost {
				continue
			}
			n, err := f.s.roller.Roll(int(pr.Sides))
			if err != nil {
				return false, fmt.Errorf("reroll the damage: %w", err)
			}
			pr.Sum += clamp32(n, 0, 100) - face
			pr.Rerolled = append(pr.Rerolled, rerolled{Index: clamp32(j, 0, 200), From: face, To: clamp32(n, 0, 100)})
			pr.Faces[j], changed = clamp32(n, 0, 100), true
		}
	}
	if changed {
		f.pr.byType, f.pr.total = map[string]int{}, 0
		f.pr.sumUp()
	}
	return changed, nil
}
