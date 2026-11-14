package play

import (
	"context"
	"encoding/json"
	"errors"
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
// incapacitated: aurasOf leaves the aura out then and it is back when the holder is up again.

// sourceKindFeature is the source of the rows a feature writes.
const sourceKindFeature = "feature"

// isFeatureAura says the row is the aura of a feature.
func isFeatureAura(st playdb.CombatantState) bool {
	return conv.OrZero(st.SourceKind) == sourceKindFeature && isMarked(st)
}

// restFeatureAuras puts the feature auras of a character's combatant to sleep (down: it fell to 0 hit points) or wakes them
// (it is above 0 again) in the session's combat. A sleeping aura keeps its radius with a minus sign and reaches nobody
// (auraReaches). It runs for every change of a character's vitals (changeVitals): damage, healing, the master's hand, an
// undo. A character that is incapacitated by a condition is another way: aurasOf reads the combatant's conditions.
func (s *Service) restFeatureAuras(ctx context.Context, q *playdb.Queries, sessionID, characterID string, down bool) error {
	enc, err := q.GetOpenEncounter(ctx, sessionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("find the open encounter: %w", err)
	}
	rows, err := q.ListLastingEffects(ctx, enc.ID)
	if err != nil {
		return fmt.Errorf("list the effects: %w", err)
	}
	cs, err := q.ListCombatants(ctx, enc.ID)
	if err != nil {
		return fmt.Errorf("list the combatants: %w", err)
	}
	for _, st := range rows {
		i := slices.IndexFunc(cs, func(c playdb.Combatant) bool { return c.ID == st.CombatantID })
		if i < 0 || !backedByCharacter(cs[i]) || cs[i].CharacterID != characterID || !isFeatureAura(st) {
			continue
		}
		mods, changed := auraAsleep(storedModifiers(st), down)
		if !changed {
			continue
		}
		body, _ := json.Marshal(mods) // plain values: they cannot fail to encode
		if err := q.SetLastingEffectModifiers(ctx, playdb.SetLastingEffectModifiersParams{ID: st.ID, Modifiers: body}); err != nil {
			return fmt.Errorf("rest the aura of a feature: %w", err)
		}
	}
	return nil
}

// auraAsleep is the modifiers with the mark of the aura asleep (a radius below 0) or awake, and whether it changed.
func auraAsleep(mods []rules.EffectModifier, asleep bool) ([]rules.EffectModifier, bool) {
	out := slices.Clone(mods)
	changed := false
	for i, m := range out {
		if m.Kind != rules.ModifierAura || (m.RadiusFt < 0) == asleep {
			continue
		}
		out[i].RadiusFt, changed = -m.RadiusFt, true
	}
	return out, changed
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
	down, err := s.isDown(ctx, c.tx, c.session.CampaignID, a)
	if err != nil {
		return err
	}
	for _, fa := range sheet.Traits.Auras {
		if slices.ContainsFunc(rows, func(st playdb.CombatantState) bool {
			return st.CombatantID == a.ID && isFeatureAura(st) && conv.OrZero(st.SourceKey) == fa.Source
		}) {
			continue
		}
		if err := s.addFeatureAura(ctx, c, all, a, fa, down); err != nil {
			return err
		}
	}
	return nil
}

// addFeatureAura writes the row of one aura on its bearer; one that joins at 0 hit points waits asleep for it to stand.
func (s *Service) addFeatureAura(ctx context.Context, c *combatTx, all []playdb.Combatant, bearer playdb.Combatant, fa link.FeatureAura, down bool) error {
	var mods []rules.EffectModifier
	if err := json.Unmarshal(fa.Modifiers, &mods); err != nil {
		return fmt.Errorf("read the modifiers of an aura: %w", err)
	}
	if down {
		mods, _ = auraAsleep(mods, true)
	}
	spec := effectSpec{
		key: fa.Source, sourceKind: sourceKindFeature, caster: &bearer, targets: []playdb.Combatant{bearer},
		dur: durationSpec{Kind: rules.EffectDurationUntilDismissed}, audience: fa.Visibility, label: fa.LabelPT, modifiers: mods,
	}
	_, err := s.addEffects(ctx, c, all, spec)
	return err
}
