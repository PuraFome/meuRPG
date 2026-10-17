package play

import (
	"context"
	"errors"
	"slices"

	"connectrpc.com/connect"

	"github.com/PuraFome/meuRPG/backend/internal/platform/conv"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// What a spell that lasts leaves on its targets (RN-22): Bênção, Perdição, Velocidade,
// Imobilizar Pessoa, Riso Histérico, Fogo das Fadas and Teia (SRD 5.1). The cast rolls the
// saving throw of the spell as it always did; this puts the effect on the targets it took
// hold of, with the duration of the spell on the clock (one minute is 10 rounds, counted to
// the start of the caster's turn) and the caster's concentration holding it. The rider of an
// instantaneous spell (Zombaria Viciosa's disadvantage) has a clock of its own, on its target's
// next turn.

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
	whyWearsArmor  = "o alvo veste armadura"
)

// applyLastingSpell puts the effect of a spell on the targets it took hold of. targs and
// made.Hits are in the same order. A spell without an effect that lasts does nothing here.
func (s *Service) applyLastingSpell(
	ctx context.Context, c *combatTx, cs []playdb.Combatant, sp link.Spell, caster playdb.Combatant, targs []playdb.Combatant, ability string, made *actionEvent,
) error {
	content, err := s.contentOf(ctx, c)
	if err != nil {
		return err
	}
	def, ok := content.CombatSpellEffect(sp.Key)
	if !ok || len(targs) == 0 || len(made.Hits) != len(targs) {
		return nil
	}
	kinds, err := s.creatureKindsOf(ctx, c, targs)
	if err != nil {
		return err
	}
	states, err := s.readStates(ctx, c.tx, c.enc.ID)
	if err != nil {
		return err
	}
	taken := s.markSpellHits(content, def, states, kinds, targs, made)
	if taken, err = s.dropWithoutHold(ctx, c, def, ability, caster, targs, taken, made); err != nil {
		return err
	}
	if len(taken) == 0 {
		return nil
	}
	dur := spellEffectDuration(content, def, sp.Key, caster)
	var dc *int32
	if sp.CasterDC > 0 {
		dc = new(clamp32(sp.CasterDC, 1, 40))
	}
	made.Lasting = &lastingEvent{Key: sp.Key, Change: "added"}
	// An effect that sets a base armor class (Mage Armor) is worked out for each target, and does
	// not take hold of a creature that wears armor.
	opts, err := s.castOptsOf(ctx, c, def, ability, caster.CharacterID)
	if err != nil {
		return err
	}
	groups, mods, err := s.spellEffectGroups(ctx, c, def, taken, opts)
	if err != nil {
		return err
	}
	var rows []playdb.CombatantState
	for i, g := range groups {
		made1, err := s.addEffects(ctx, c, cs, effectSpec{
			key: sp.Key, sourceKind: "spell", def: def, caster: &caster, group: made.CastID, targets: g, dur: dur,
			concentration: def.Concentration, dc: dc, modifiers: mods[i],
		})
		if err != nil {
			return err
		}
		rows = append(rows, made1...)
	}
	for _, r := range rows {
		made.Lasting.Targets = append(made.Lasting.Targets, r.CombatantID)
		made.Lasting.Effects = append(made.Lasting.Effects, r.ID)
	}
	return nil
}

// creatureKindsOf is the kind of creature an NPC was made from, by character, for the spells
// that pick a type.
func (s *Service) creatureKindsOf(ctx context.Context, c *combatTx, targs []playdb.Combatant) (map[string]string, error) {
	kinds := map[string]string{}
	var ids []string
	for _, t := range targs {
		if t.CharacterID != "" && t.MonsterKey == nil {
			ids = append(ids, t.CharacterID)
		}
	}
	if len(ids) == 0 {
		return kinds, nil
	}
	chars, err := s.roster.CombatCharacters(ctx, c.tx, c.session.CampaignID, ids)
	if err != nil {
		return nil, err
	}
	for _, ch := range chars {
		kinds[ch.ID] = ch.MonsterKey
	}
	return kinds, nil
}

// markSpellHits says, on each hit, whether the effect took hold of its target, and returns the
// targets it took hold of.
func (s *Service) markSpellHits(content *rules.Content, def *rules.EffectDef, states map[string][]playdb.CombatantState, kinds map[string]string, targs []playdb.Combatant, made *actionEvent) []playdb.Combatant {
	var taken []playdb.Combatant
	for i, t := range targs {
		hit := &made.Hits[i]
		key := conv.OrZero(t.MonsterKey)
		if key == "" {
			key = kinds[t.CharacterID]
		}
		if why := s.noEffectWhy(content, def, key); why != "" {
			hit.Lasting, hit.NoEffectWhy = lastingNoEffect, why
			continue
		}
		if slices.ContainsFunc(def.Conditions, func(k string) bool { return combat.ImmuneTo(effectModsOf(states, t.ID), k) }) {
			hit.Lasting, hit.NoEffectWhy = lastingNoEffect, whyImmune
			continue
		}
		// The spell asks a saving throw and holds the ones that failed it.
		if def.Applies == "failed_save" && (hit.Save == nil || hit.Save.Saved) {
			continue
		}
		hit.Lasting = lastingApplied
		taken = append(taken, t)
	}
	return taken
}

// dropWithoutHold takes out of the targets an effect that sets a base armor class cannot take hold
// of (Mage Armor on a creature that wears armor, SRD 5.1) and marks their hits "no effect".
func (s *Service) dropWithoutHold(ctx context.Context, c *combatTx, def *rules.EffectDef, ability string, caster playdb.Combatant, targs, taken []playdb.Combatant, made *actionEvent) ([]playdb.Combatant, error) {
	if !hasBaseAC(def) {
		return taken, nil
	}
	opts, err := s.castOptsOf(ctx, c, def, ability, caster.CharacterID)
	if err != nil {
		return nil, err
	}
	var kept []playdb.Combatant
	for _, t := range taken {
		_, ok, err := s.modifiersForTarget(ctx, c, def, t, opts)
		if err != nil {
			return nil, err
		}
		if ok {
			kept = append(kept, t)
			continue
		}
		i := slices.IndexFunc(targs, func(o playdb.Combatant) bool { return o.ID == t.ID })
		made.Hits[i].Lasting, made.Hits[i].NoEffectWhy = lastingNoEffect, whyWearsArmor
	}
	return kept, nil
}

// spellEffectDuration is how long the effect of a spell lasts.
func spellEffectDuration(content *rules.Content, def *rules.EffectDef, spellKey string, caster playdb.Combatant) durationSpec {
	rounds := content.SpellEffectRounds(spellKey)
	switch {
	case def.Duration != nil:
		// The rider of an instantaneous spell keeps its own clock (Zombaria Viciosa: until the end
		// of the target's next turn). With no anchor, endsOf anchors it on each target, at the
		// target's next turn: this round when it has not acted yet, the next when it has.
		return durationSpec{Kind: def.Duration.Kind}
	case rounds > 0:
		return durationSpec{Kind: rules.EffectDurationRounds, Rounds: int32(rounds), Anchor: caster.ID} //nolint:gosec // 10 rounds a minute
	case def.Concentration:
		return durationSpec{Kind: rules.EffectDurationConcentration}
	}
	return durationSpec{Kind: rules.EffectDurationUntilDismissed}
}

// spellEffectGroups are the targets that get the effect together, each group with the modifiers
// it gets: all of them, or one by one for an effect that sets a base armor class.
func (s *Service) spellEffectGroups(ctx context.Context, c *combatTx, def *rules.EffectDef, taken []playdb.Combatant, opts castOpts) ([][]playdb.Combatant, [][]rules.EffectModifier, error) {
	common, _, err := s.modifiersFor(ctx, c, def, "", opts)
	if err != nil {
		return nil, nil, err
	}
	if !hasBaseAC(def) {
		return [][]playdb.Combatant{taken}, [][]rules.EffectModifier{common}, nil
	}
	var groups [][]playdb.Combatant
	var mods [][]rules.EffectModifier
	for _, t := range taken {
		m, ok, err := s.modifiersForTarget(ctx, c, def, t, opts)
		if err != nil {
			return nil, nil, err
		}
		if ok {
			groups, mods = append(groups, []playdb.Combatant{t}), append(mods, m)
		}
	}
	return groups, mods, nil
}

// noEffectWhy says why a spell takes no hold of a target, "" when it may: a spell that picks a
// humanoid (Imobilizar Pessoa), a condition the creature is immune to, a creature too simple for
// Riso Histérico (SRD 5.1: an Intelligence score of 4 or less).
func (s *Service) noEffectWhy(content *rules.Content, def *rules.EffectDef, monsterKey string) string {
	if monsterKey == "" {
		return ""
	}
	creature, known := content.CreatureByKey(monsterKey)
	if !known {
		return ""
	}
	if def.TargetType == "humanoid" && !combat.IsHumanoid(creature.Type) {
		return whyNotHumanoid
	}
	if slices.ContainsFunc(def.Conditions, func(k string) bool { return immuneCreature(creature, k) }) {
		return whyImmune
	}
	if def.Key == "spell:hideous-laughter" && slices.ContainsFunc(creature.Abilities, func(a rules.AbilityScore) bool { return string(a.Ability) == "int" && a.Base <= 4 }) {
		return whyTooSimple
	}
	return ""
}

// immuneCreature says the creature is immune to the condition.
func immuneCreature(creature rules.Creature, condition string) bool {
	return slices.ContainsFunc(creature.ConditionImmunities, func(n rules.NamedKey) bool { return n.Key == condition })
}

// abilityChoiceOf checks the ability a spell is cast for: Enhance Ability needs one of the six
// (SRD 5.1: Bull's Strength, Cat's Grace, Bear's Endurance, Fox's Cunning, Owl's Wisdom, Eagle's
// Splendor); any other spell takes none and ignores it.
func (s *Service) abilityChoiceOf(ctx context.Context, c *combatTx, spellKey, ability string) (string, error) {
	content, err := s.contentOf(ctx, c)
	if err != nil {
		return "", err
	}
	def, ok := content.CombatSpellEffect(spellKey)
	if !ok || !slices.ContainsFunc(def.Modifiers, func(m rules.EffectModifier) bool { return m.Choose }) {
		return "", nil
	}
	if !slices.Contains(effectAbilities, ability) {
		return "", connect.NewError(connect.CodeInvalidArgument, errors.New("ability_key must be one of str, dex, con, int, wis or cha for this spell"))
	}
	return ability, nil
}

// effectAbilities are the abilities an effect may be chosen for.
var effectAbilities = []string{"str", "dex", "con", "int", "wis", "cha"}
