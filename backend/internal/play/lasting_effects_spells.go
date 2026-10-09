package play

import (
	"context"
	"slices"

	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// What a spell that lasts leaves on its targets (RN-22): Bênção, Perdição, Velocidade,
// Imobilizar Pessoa, Riso Histérico, Fogo das Fadas and Teia (SRD 5.1). The cast rolls the
// saving throw of the spell as it always did; this puts the effect on the targets it took
// hold of, with the duration of the spell on the clock (one minute is 10 rounds, counted to
// the start of the caster's turn) and the caster's concentration holding it.

// The outcomes of a spell's effect on a target, as a cast event keeps them.
const (
	lastingApplied  = "applied"
	lastingNoEffect = "no_effect"
)

// Why a spell had no effect on a target: only the master reads it (RN-20: a player reads
// "A magia não teve efeito." and never why).
const (
	whyNotHumanoid = "alvo não é humanoide"
	whyImmune      = "o alvo é imune à condição"
	whyTooSimple   = "a Inteligência do alvo é 4 ou menos"
)

// applyLastingSpell puts the effect of a spell on the targets it took hold of. targs and
// made.Hits are in the same order. A spell without an effect that lasts does nothing here.
func (s *Service) applyLastingSpell(ctx context.Context, c *combatTx, cs []playdb.Combatant, sp link.Spell, caster playdb.Combatant, targs []playdb.Combatant, made *actionEvent) error {
	content, err := s.contentOf(ctx, c)
	if err != nil {
		return err
	}
	def, ok := content.CombatSpellEffect(sp.Key)
	if !ok || len(targs) == 0 || len(made.Hits) != len(targs) {
		return nil
	}
	var taken []playdb.Combatant
	for i, t := range targs {
		hit := &made.Hits[i]
		if why := s.noEffectWhy(content, def, t); why != "" {
			hit.Lasting, hit.NoEffectWhy = lastingNoEffect, why
			continue
		}
		if def.Applies == "failed_save" {
			// The spell asks a saving throw and holds the ones that failed it.
			if hit.Save == nil || hit.Save.Saved {
				continue
			}
		}
		hit.Lasting = lastingApplied
		taken = append(taken, t)
	}
	if len(taken) == 0 {
		return nil
	}
	dur := durationSpec{Kind: rules.EffectDurationUntilDismissed}
	switch rounds := content.SpellEffectRounds(sp.Key); {
	case rounds > 0:
		dur = durationSpec{Kind: rules.EffectDurationRounds, Rounds: int32(rounds), Anchor: caster.ID} //nolint:gosec // 10 rounds a minute
	case def.Concentration:
		dur = durationSpec{Kind: rules.EffectDurationConcentration}
	}
	var dc *int32
	if sp.CasterDC > 0 {
		dc = new(clamp32(sp.CasterDC, 1, 40))
	}
	made.Lasting = &lastingEvent{Key: sp.Key, Change: "added"}
	rows, err := s.addEffects(ctx, c, cs, effectSpec{
		key: sp.Key, sourceKind: "spell", def: def, caster: &caster, group: made.CastID, targets: taken, dur: dur,
		concentration: def.Concentration, dc: dc,
	})
	if err != nil {
		return err
	}
	for _, r := range rows {
		made.Lasting.Targets = append(made.Lasting.Targets, r.CombatantID)
		made.Lasting.Effects = append(made.Lasting.Effects, r.ID)
	}
	return nil
}

// noEffectWhy says why a spell takes no hold of a target, "" when it may: a spell that picks a
// humanoid (Imobilizar Pessoa), a condition the creature is immune to, a creature too simple for
// Riso Histérico (SRD 5.1: an Intelligence score of 4 or less).
func (s *Service) noEffectWhy(content *rules.Content, def *rules.EffectDef, t playdb.Combatant) string {
	var creature rules.Creature
	known := false
	if t.MonsterKey != nil {
		creature, known = content.CreatureByKey(*t.MonsterKey)
	}
	if def.TargetType == "humanoid" && known && !combat.IsHumanoid(creature.Type) {
		return whyNotHumanoid
	}
	if known {
		for _, k := range def.Conditions {
			if slices.ContainsFunc(creature.ConditionImmunities, func(n rules.NamedKey) bool { return n.Key == k }) {
				return whyImmune
			}
		}
		if def.Key == "spell:hideous-laughter" {
			for _, a := range creature.Abilities {
				if string(a.Ability) == "int" && a.Base <= 4 {
					return whyTooSimple
				}
			}
		}
	}
	return ""
}
