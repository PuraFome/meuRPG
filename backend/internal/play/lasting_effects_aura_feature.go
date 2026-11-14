package play

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/jackc/pgx/v5"

	"github.com/PuraFome/meuRPG/backend/internal/platform/conv"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// The aura of a feature (the aura effect of a table's feature) is a row on its holder like a spell's aura, made when
// the holder joins a combat and gone when it leaves or the combat ends: it is never a record of the character's, so
// giveEffectBack drops it and the next combat makes it again from the sheet. It has no clock: the holder carries it as
// long as it is in the fight (until the master ends it by hand). Nothing is written when the holder is downed or
// incapacitated: aurasOf decides it when the aura is read (from the hit points and the conditions as they are then), so
// it is back when the holder is up again, whichever way the hit points changed.

// sourceKindFeature is the source of the rows a feature writes.
const sourceKindFeature = "feature"

// isFeatureAura says the row is the aura of a feature.
func isFeatureAura(st playdb.CombatantState) bool {
	return conv.OrZero(st.SourceKind) == sourceKindFeature && isMarked(st)
}

// downFeatureBearers are the combatants (by ID) that bear the aura of a feature and are at 0 hit points: aurasOf leaves
// their auras out. It reads a character's hit points only when the combat has such an aura, so a combat with none reads
// nothing more. rows are the states of the combat; tx is the change's transaction, nil for a read.
func (s *Service) downFeatureBearers(ctx context.Context, tx pgx.Tx, campaignID string, cs []playdb.Combatant, rows []playdb.CombatantState) (map[string]bool, error) {
	var down map[string]bool
	for _, st := range rows {
		if !isFeatureAura(st) || down[st.CombatantID] {
			continue
		}
		i := slices.IndexFunc(cs, func(c playdb.Combatant) bool { return c.ID == st.CombatantID })
		if i < 0 || !backedByCharacter(cs[i]) {
			continue
		}
		gone, err := s.isDown(ctx, tx, campaignID, cs[i])
		if err != nil {
			return nil, err
		}
		if gone {
			if down == nil {
				down = map[string]bool{}
			}
			down[cs[i].ID] = true
		}
	}
	return down, nil
}

// carriedToCombat brings in what the characters that joined carry: their inspiration die and the auras of their features.
func (s *Service) carriedToCombat(ctx context.Context, c *combatTx, all, added []playdb.Combatant) error {
	if err := s.dieToCombat(ctx, c, all, added); err != nil {
		return err
	}
	return s.featureAurasToCombat(ctx, c, all, added)
}

// giveBackOrDrop gives an effect back to its character when the combat ends, or drops it when it is the aura of a feature:
// the sheet makes it again in the next combat.
func (s *Service) giveBackOrDrop(ctx context.Context, c *combatTx, cs []playdb.Combatant, who playdb.Combatant, st playdb.CombatantState) error {
	if !isFeatureAura(st) {
		return s.giveEffectBack(ctx, c, cs, who, st)
	}
	if err := c.q.DeleteLastingEffect(ctx, st.ID); err != nil {
		return fmt.Errorf("end the aura of a feature: %w", err)
	}
	return nil
}

// featureAurasToCombat makes the auras of the features of the characters that joined, on each of them, once.
func (s *Service) featureAurasToCombat(ctx context.Context, c *combatTx, all, added []playdb.Combatant) error {
	for _, a := range added {
		if !backedByCharacter(a) {
			continue
		}
		if err := s.featureAurasOn(ctx, c, all, a); err != nil {
			return err
		}
	}
	return nil
}

// featureAurasOn makes the auras of the sheet of one character that joined, the ones it has not yet.
func (s *Service) featureAurasOn(ctx context.Context, c *combatTx, all []playdb.Combatant, a playdb.Combatant) error {
	sheet, err := s.sheetOf(ctx, c.tx, c.session.CampaignID, a)
	if err != nil {
		return fmt.Errorf("read the sheet for its auras: %w", err)
	}
	if len(sheet.Traits.Auras) == 0 {
		return nil
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
		if err := s.addFeatureAura(ctx, c, all, a, fa); err != nil {
			return err
		}
	}
	return nil
}

// addFeatureAura writes the row of one aura on its bearer.
func (s *Service) addFeatureAura(ctx context.Context, c *combatTx, all []playdb.Combatant, bearer playdb.Combatant, fa link.FeatureAura) error {
	var mods []rules.EffectModifier
	if err := json.Unmarshal(fa.Modifiers, &mods); err != nil {
		return fmt.Errorf("read the modifiers of an aura: %w", err)
	}
	spec := effectSpec{
		key: fa.Source, sourceKind: sourceKindFeature, caster: &bearer, targets: []playdb.Combatant{bearer},
		dur: durationSpec{Kind: rules.EffectDurationUntilDismissed}, audience: fa.Visibility, label: fa.LabelPT, modifiers: mods,
	}
	_, err := s.addEffects(ctx, c, all, spec)
	return err
}
