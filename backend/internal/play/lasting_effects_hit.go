package play

import (
	"context"
	"slices"

	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// What a spell that adds weapon damage to the caster's hits leaves (#577): the effect rides on the CASTER, not
// on the spell's target. Its row carries the weapon_hit_damage modifier with the dice worked out for the slot
// the cast used (Dice set, PerSlotDice zeroed, as defAtSlot does for a trigger), and the spell's save DC.
// For the trigger marked_target the cast also writes a second row of the same group on the TARGET: a mark with
// no modifier, no condition, hidden from the players, whose source is the caster. The pair is found by the
// group, the key and the source, so it needs no column of its own, ends with the caster's concentration like any
// row it holds, and the undo of the cast (ev.Lasting.Effects) takes both away. A recast ends the old pair first:
// the new concentration ends the old one's rows (concentrate) before this runs.

// hitModifierOf is the weapon_hit_damage modifier of an effect's modifiers.
func hitModifierOf(mods []rules.EffectModifier) (rules.EffectModifier, bool) {
	i := slices.IndexFunc(mods, func(m rules.EffectModifier) bool { return m.Kind == rules.ModifierWeaponHitDamage })
	if i < 0 {
		return rules.EffectModifier{}, false
	}
	return mods[i], true
}

// hitDiceAt is the dice a weapon_hit_damage rolls when cast levelsAbove slot levels above the spell's own: Dice
// and PerSlotDice for each level, up to MaxDice when it has one.
func hitDiceAt(m rules.EffectModifier, levelsAbove int) int {
	n := m.Dice + m.PerSlotDice*max(levelsAbove, 0)
	if m.MaxDice > 0 {
		n = min(n, m.MaxDice)
	}
	return n
}

// hitModifiersAtSlot is the modifiers with each weapon_hit_damage worked out for the slot: its Dice set to what the
// slot gives and its PerSlotDice zeroed, so the row says what to roll. The given modifiers are not changed.
func hitModifiersAtSlot(mods []rules.EffectModifier, levelsAbove int) []rules.EffectModifier {
	out := slices.Clone(mods)
	for i, m := range out {
		if m.Kind == rules.ModifierWeaponHitDamage {
			out[i].Dice, out[i].PerSlotDice = hitDiceAt(m, levelsAbove), 0
		}
	}
	return out
}

// applyHitSpell puts the effect of a spell with a weapon_hit_damage on the caster, and the mark of a marked_target
// on its target.
func (s *Service) applyHitSpell(
	ctx context.Context, c *combatTx, cs []playdb.Combatant, sp link.Spell, slotLevel int, caster playdb.Combatant, targs []playdb.Combatant, ability string, def *rules.EffectDef, made *actionEvent,
) error {
	content, err := s.contentOf(ctx, c)
	if err != nil {
		return err
	}
	opts, err := s.castOptsOf(ctx, c, def, ability, caster.CharacterID)
	if err != nil {
		return err
	}
	mods, _, err := s.modifiersFor(ctx, c, def, "", opts)
	if err != nil {
		return err
	}
	mods = hitModifiersAtSlot(mods, slotLevel-sp.Level)
	hit, _ := hitModifierOf(mods)
	if hit.Trigger == rules.HitTriggerMarkedTarget && len(targs) != 1 {
		return nil // the mark needs the one creature it is put on
	}
	spec := effectSpec{
		key: sp.Key, sourceKind: "spell", def: def, caster: &caster, group: made.CastID, targets: []playdb.Combatant{caster},
		dur: spellEffectDuration(content, def, sp.Key, caster), concentration: def.Concentration,
		conditions: []string{}, modifiers: mods,
	}
	if sp.CasterDC > 0 {
		spec.dc = new(clamp32(sp.CasterDC, 1, 40))
	}
	made.Lasting = &lastingEvent{Key: sp.Key, Change: "added"}
	rows, err := s.addEffects(ctx, c, cs, spec)
	if err != nil {
		return err
	}
	if hit.Trigger == rules.HitTriggerMarkedTarget {
		marks, err := s.addEffects(ctx, c, cs, markSpec(spec, targs))
		if err != nil {
			return err
		}
		rows = append(rows, marks...)
	}
	for i := range made.Hits {
		made.Hits[i].Lasting = lastingApplied
	}
	for _, r := range rows {
		made.Lasting.Targets = append(made.Lasting.Targets, r.CombatantID)
		made.Lasting.Effects = append(made.Lasting.Effects, r.ID)
	}
	return nil
}

// markSpec is the effect of the mark a marked_target puts on its target: the spell's own clock and group, and
// nothing a player reads.
func markSpec(spec effectSpec, targs []playdb.Combatant) effectSpec {
	spec.def, spec.targets, spec.dc = nil, targs, nil
	spec.modifiers, spec.conditions = []rules.EffectModifier{}, []string{}
	spec.visible = new(false)
	return spec
}
