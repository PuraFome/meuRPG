package play

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/PuraFome/meuRPG/backend/internal/platform/conv"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// The aura of a feature (the aura effect of a table's feature) is a row on its holder like a spell's aura, made when
// the holder joins a combat and gone when it leaves or the combat ends: it is never a record of the character's, so
// giveEffectBack drops it and the next combat makes it again from the sheet. It has no clock: the holder carries it as
// long as it is in the fight (until the master ends it by hand). Nothing is written when the holder is downed or
// incapacitated: aurasOf leaves the aura out then and it is back when the holder is up again.

// sourceKindFeature is the source of the rows a feature writes.
const sourceKindFeature = "feature"

// isFeatureAura says the row is the aura of a feature.
func isFeatureAura(st playdb.CombatantState) bool {
	return conv.OrZero(st.SourceKind) == sourceKindFeature && isMarked(st)
}

// featureAurasToCombat makes the auras of the features of the characters that joined, on each of them, once.
func (s *Service) featureAurasToCombat(ctx context.Context, c *combatTx, all, added []playdb.Combatant) error {
	for _, a := range added {
		if !backedByCharacter(a) {
			continue
		}
		sheet, err := s.sheetOf(ctx, c.tx, c.session.CampaignID, a)
		if err != nil {
			return fmt.Errorf("read the sheet for its auras: %w", err)
		}
		if len(sheet.Traits.Auras) == 0 {
			continue
		}
		rows, err := c.q.ListLastingEffects(ctx, c.enc.ID)
		if err != nil {
			return fmt.Errorf("list the effects: %w", err)
		}
		for _, fa := range sheet.Traits.Auras {
			if slices.ContainsFunc(rows, func(st playdb.CombatantState) bool {
				return st.CombatantID == a.ID && isFeatureAura(st) && conv.OrZero(st.SourceKey) == fa.Source
			}) {
				continue
			}
			var mods []rules.EffectModifier
			if err := json.Unmarshal(fa.Modifiers, &mods); err != nil {
				return fmt.Errorf("read the modifiers of an aura: %w", err)
			}
			caster := a
			spec := effectSpec{
				key: fa.Source, sourceKind: sourceKindFeature, caster: &caster, targets: []playdb.Combatant{a},
				dur: durationSpec{Kind: rules.EffectDurationUntilDismissed}, audience: fa.Visibility, label: fa.LabelPT, modifiers: mods,
			}
			if _, err := s.addEffects(ctx, c, all, spec); err != nil {
				return err
			}
		}
	}
	return nil
}
