package play

import (
	"context"
	"fmt"
	"slices"

	"github.com/PuraFome/meuRPG/backend/internal/platform/conv"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// Damage dice rolled again (the table's reroll effects that cover "damage"): the SRD's reroll means the die is rolled
// again and the new roll is used. The app applies it, the player is not asked, so it is not a held roll. One use
// (Cost units of the rule's resource, or none for a rule with no resource) covers one damage roll: every die of the
// roll that shows AtMost or lower is rolled once more, and a die is rolled again once (a die Great Weapon Fighting
// already rolled again is kept). Only the dice the app rolled can be: a typed sum has no faces. A damage that waits
// for a reaction window is not rerolled, nor is its replay: the reroll runs after the windows were decided, on a
// roll no window held.

// damageRerollUse is the use of a reroll rule a damage roll spent, for the undo.
type damageRerollUse struct {
	Resource string `json:"resource"`
	Cost     int32  `json:"cost"`
}

// damageRerollRules are the reroll rules of the attacker that cover a die of damage, in the sheet's order. A rule
// with no resource is free.
func damageRerollRules(t link.Traits) []link.RerollRule {
	return rerollRulesFor(t, rules.RollDamage)
}

// mayRerollDamage says the roll is one the app can roll again: a player's character, damage (not healing) with dice
// the app rolled, and not the replay of a roll a reaction window held.
func (f *damageFlow) mayRerollDamage() bool {
	if f.c.replay != nil || f.p.Healing || !f.call.in.inApp || !strokeMayApply(f.attacker) {
		return false
	}
	return f.pr != nil || f.expr.Count > 0
}

// rerollDamage rolls again the low dice of the damage just rolled when the attacker has a reroll rule that covers
// it, and spends the rule's use. It runs after the reaction windows were decided (a held roll is replayed as it was
// rolled) and before the adjustments and the event. One rule serves the roll: the first that has the uses and
// changes a die.
func (f *damageFlow) rerollDamage(ctx context.Context) error {
	if !f.mayRerollDamage() {
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
	for _, r := range rs {
		if r.Resource != "" && int(left[r.Resource]) < r.Cost {
			continue
		}
		if changed, err := f.rerollLowDice(r.AtMost, r.Source); err != nil || !changed {
			if err != nil {
				return err
			}
			continue // nothing to roll again: the next rule may
		}
		return f.spendReroll(ctx, r)
	}
	return nil
}

// spendReroll spends the use the rule's reroll cost and notes it in the event for the undo. A free rule spends
// nothing.
func (f *damageFlow) spendReroll(ctx context.Context, r link.RerollRule) error {
	if r.Resource == "" {
		return nil
	}
	cost := conv.Int32(r.Cost)
	v, err := f.s.spendResource(ctx, f.c, f.attacker.CharacterID, r.Resource, cost)
	if err != nil {
		return err
	}
	f.out.vitals = append(f.out.vitals, v)
	f.reroll = &damageRerollUse{Resource: r.Resource, Cost: cost}
	return nil
}

// rerollLowDice rolls once more each die of the roll that shows atMost or lower and keeps the new face. It says
// whether any die was rolled again.
func (f *damageFlow) rerollLowDice(atMost int, source string) (bool, error) {
	if f.pr != nil {
		changed, err := f.rerollPartsDice(atMost, source)
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

// rerollPartsDice does the same for the dice of the parts, keeping the faces replaced in the part's Rerolled list
// (source_key: the rule's feature). Every part is rolled again, whether it counts or not: a player reads the dice of a
// conditional part (the Divine Smite die) as counted whatever the target is, so skipping the part the target's kind
// keeps out would tell the player what the target is. The total still adds only the parts that count (sumUp).
func (f *damageFlow) rerollPartsDice(atMost int, source string) (bool, error) {
	changed := false
	for i := range f.pr.rolls {
		did, err := f.s.rerollPartDice(&f.pr.rolls[i], atMost, source)
		if err != nil {
			return false, err
		}
		changed = changed || did
	}
	if changed {
		f.pr.byType, f.pr.total = map[string]int{}, 0
		f.pr.sumUp()
	}
	return changed, nil
}

// rerollPartDice rolls again the dice of a part that show atMost or lower, but not one that was rolled
// again already (Great Weapon Fighting): a die is rolled again once.
func (s *Service) rerollPartDice(pr *partRoll, atMost int, source string) (bool, error) {
	changed := false
	for j, face := range pr.Faces {
		if int(face) > atMost || slices.ContainsFunc(pr.Rerolled, func(r rerolled) bool { return int(r.Index) == j }) {
			continue
		}
		n, err := s.roller.Roll(int(pr.Sides))
		if err != nil {
			return false, fmt.Errorf("reroll the damage: %w", err)
		}
		to := clamp32(n, 0, 100)
		pr.Sum += to - face
		pr.Rerolled = append(pr.Rerolled, rerolled{Index: clamp32(j, 0, 200), From: face, To: to, SourceKey: source})
		pr.Faces[j], changed = to, true
	}
	return changed, nil
}
