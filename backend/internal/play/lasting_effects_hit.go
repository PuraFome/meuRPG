package play

import (
	"cmp"
	"context"
	"errors"
	"slices"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// What a spell that adds weapon damage to the caster's hits leaves (#577): the effect rides on the CASTER, not
// on the spell's target. Its row carries the weapon_hit_damage modifier with the dice worked out for the slot
// the cast used (Dice set, PerSlotDice zeroed, as defAtSlot does for a trigger), and the spell's save DC.
// For the trigger marked_target the cast also writes a second row of the same group on the TARGET: a mark with
// no modifier and no condition, as visible to the players as the effect (they read its name and nothing more),
// whose source is the caster. An effect that also has what a spell does to its targets (conditions, other
// modifiers) leaves that on them as a row of its own, which is then the mark. The pair is found by the
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

// withoutHitModifiers is the modifiers that are not a weapon_hit_damage: what a spell with both kinds of block puts
// on its targets (the hit modifier is the caster's alone).
func withoutHitModifiers(mods []rules.EffectModifier) []rules.EffectModifier {
	return slices.DeleteFunc(slices.Clone(mods), func(m rules.EffectModifier) bool { return m.Kind == rules.ModifierWeaponHitDamage })
}

// onlyHitModifiers is the weapon_hit_damage modifiers: what the caster's own row carries.
func onlyHitModifiers(mods []rules.EffectModifier) []rules.EffectModifier {
	return slices.DeleteFunc(slices.Clone(mods), func(m rules.EffectModifier) bool { return m.Kind != rules.ModifierWeaponHitDamage })
}

// targetSideOf is the effect a spell with a weapon_hit_damage leaves on its targets: the effect without the hit
// modifier, nil when nothing else is left (a spell that only adds damage to the caster's hits).
func targetSideOf(def *rules.EffectDef) *rules.EffectDef {
	rest := withoutHitModifiers(def.Modifiers)
	if len(rest) == 0 && len(def.Conditions) == 0 {
		return nil
	}
	cast := *def
	cast.Modifiers, cast.TagsPT = rest, rules.EffectTagsPT(rest)
	return &cast
}

// applyHitSpell puts the effect of a spell with a weapon_hit_damage in its two places: what the effect does to
// its targets (conditions, the other modifiers) lands on them like any lasting spell, and the hit modifier stays on the
// caster's own row, with the dice worked out for the slot. A marked_target also leaves a mark on its target. One
// concentration holds all the rows, so its end ends them together.
func (s *Service) applyHitSpell(
	ctx context.Context, c *combatTx, cs []playdb.Combatant, sp link.Spell, slotLevel int, caster playdb.Combatant, targs []playdb.Combatant, ability string, def *rules.EffectDef, made *actionEvent,
) error {
	var onTargets []playdb.CombatantState
	if side := targetSideOf(def); side != nil {
		var err error
		if onTargets, err = s.putLastingOnTargets(ctx, c, cs, sp, slotLevel, caster, targs, ability, side, made); err != nil {
			return err
		}
	} else {
		for i := range made.Hits {
			made.Hits[i].Lasting = lastingApplied
		}
	}
	rows, err := s.putHitRows(ctx, c, cs, sp, slotLevel, caster, targs, def, made, onTargets)
	if err != nil {
		return err
	}
	if made.Lasting == nil {
		made.Lasting = &lastingEvent{Key: sp.Key, Change: "added"}
	}
	for _, r := range rows {
		made.Lasting.Targets = append(made.Lasting.Targets, r.CombatantID)
		made.Lasting.Effects = append(made.Lasting.Effects, r.ID)
	}
	return nil
}

// putHitRows writes the caster's row of the hit modifier and, for a marked_target whose target the effect did not
// reach with a row of its own, the mark.
func (s *Service) putHitRows(
	ctx context.Context, c *combatTx, cs []playdb.Combatant, sp link.Spell, slotLevel int, caster playdb.Combatant, targs []playdb.Combatant, def *rules.EffectDef, made *actionEvent, onTargets []playdb.CombatantState,
) ([]playdb.CombatantState, error) {
	content, err := s.contentOf(ctx, c)
	if err != nil {
		return nil, err
	}
	hits, _ := hitModifierOf(def.Modifiers)
	spec := effectSpec{
		key: sp.Key, sourceKind: "spell", def: def, caster: &caster, group: made.CastID, targets: []playdb.Combatant{caster},
		dur: spellEffectDuration(content, def, sp.Key, caster), concentration: def.Concentration,
		conditions: []string{}, modifiers: hitModifiersAtSlot(onlyHitModifiers(def.Modifiers), slotLevel-sp.Level),
	}
	if sp.CasterDC > 0 {
		spec.dc = new(clamp32(sp.CasterDC, 1, 40))
	}
	rows, err := s.addEffects(ctx, c, cs, spec)
	if err != nil || hits.Trigger != rules.HitTriggerMarkedTarget || len(targs) != 1 {
		return rows, err
	}
	if slices.ContainsFunc(onTargets, func(r playdb.CombatantState) bool { return r.CombatantID == targs[0].ID }) {
		return rows, nil // the effect's own row on the target is the mark
	}
	marks, err := s.addEffects(ctx, c, cs, markSpec(spec, def, targs))
	return append(rows, marks...), err
}

// markSpec is the effect of the mark a marked_target puts on its target: the spell's own clock and group, and
// nothing a player reads but the spell's name (or its player label), as visible as the effect is.
func markSpec(spec effectSpec, def *rules.EffectDef, targs []playdb.Combatant) effectSpec {
	spec.def, spec.targets, spec.dc = nil, targs, nil
	spec.modifiers, spec.conditions = []rules.EffectModifier{}, []string{}
	spec.audience, spec.label = def.Visibility, cmp.Or(def.PlayerLabelPT, def.NamePT)
	return spec
}

// checkHitTargets refuses, before anything is spent, a cast of a marked_target spell that has no one creature to
// mark: it needs exactly one target, and not the caster.
func (f *castFlow) checkHitTargets(ctx context.Context) error {
	content, err := f.s.contentOf(ctx, f.c)
	if err != nil {
		return err
	}
	def, ok := content.CombatSpellEffect(f.sp.Key)
	if !ok {
		return nil
	}
	if hit, has := hitModifierOf(def.Modifiers); !has || hit.Trigger != rules.HitTriggerMarkedTarget {
		return nil
	}
	if len(f.targs) != 1 || f.targs[0].ID == f.caster.ID {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("this spell marks one creature: pick exactly one target, and not the caster"))
	}
	return nil
}

// tagsOfRow are the tags of the effect for one of its rows when the effect has a weapon_hit_damage: the caster's
// row reads the damage it adds to its hits, any other row what the spell does to its creature, never the damage.
func tagsOfRow(def *rules.EffectDef, mods []rules.EffectModifier) []string {
	if _, has := hitModifierOf(def.Modifiers); !has {
		return def.TagsPT
	}
	if _, ownsHit := hitModifierOf(mods); ownsHit {
		return rules.EffectTagsPT(onlyHitModifiers(def.Modifiers))
	}
	return rules.EffectTagsPT(withoutHitModifiers(def.Modifiers))
}

// modifierCard is a modifier of an effect's card: the hit of a weapon_hit_damage is the details of what the caster's
// own card reads. Only the master and the combatant's own player are given a card at all (decorateCard), and the
// mark a marked_target leaves on its target carries no modifier, so no other player is ever given these numbers.
func (ev *effectViewer) modifierCard(m rules.EffectModifier) *playv1.EffectModifier {
	out := modifierProto(m)
	if m.Kind != rules.ModifierWeaponHitDamage {
		return out
	}
	out.Hit = &playv1.WeaponHitDamage{
		Trigger: m.Trigger, Die: clamp32(m.Die, 0, 100), Dice: clamp32(m.Dice, 0, 100), PerSlotDice: clamp32(m.PerSlotDice, 0, 100),
		MaxDice: clamp32(m.MaxDice, 0, 100), DamageTypeKey: m.DamageType, Save: m.Save, ConditionKeys: m.OnFailConditions,
		ConditionDuration: m.ConditionDuration,
	}
	if m.DamageType != "" {
		out.Hit.DamageTypePt = ev.names(m.DamageType)
	}
	for _, k := range m.OnFailConditions {
		out.Hit.ConditionNamesPt = append(out.Hit.ConditionNamesPt, ev.names(k))
	}
	return out
}
