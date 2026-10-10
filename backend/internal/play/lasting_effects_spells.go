package play

import (
	"context"
	"errors"
	"slices"

	"connectrpc.com/connect"

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
func (s *Service) applyLastingSpell( //nolint:gocyclo // the steps of putting a spell's effect on its targets
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
	// The kind of creature an NPC was made from, for the spells that pick a type.
	kinds := map[string]string{}
	var ids []string
	for _, t := range targs {
		if t.CharacterID != "" && t.MonsterKey == nil {
			ids = append(ids, t.CharacterID)
		}
	}
	if len(ids) > 0 {
		chars, err := s.roster.CombatCharacters(ctx, c.tx, c.session.CampaignID, ids)
		if err != nil {
			return err
		}
		for _, ch := range chars {
			kinds[ch.ID] = ch.MonsterKey
		}
	}
	states, err := s.readStates(ctx, c.tx, c.enc.ID)
	if err != nil {
		return err
	}
	var taken []playdb.Combatant
	for i, t := range targs {
		hit := &made.Hits[i]
		key := deref(t.MonsterKey)
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
	// An effect that sets a base armor class (Mage Armor) is worked out for each target, and does
	// not take hold of a creature that wears armor.
	opts, err := s.castOptsOf(ctx, c, def, ability, caster.CharacterID)
	if err != nil {
		return err
	}
	groups := [][]playdb.Combatant{taken}
	common, _, err := s.modifiersFor(ctx, c, def, "", opts)
	if err != nil {
		return err
	}
	mods := [][]rules.EffectModifier{common}
	if hasBaseAC(def) {
		groups, mods = nil, nil
		for _, t := range taken {
			m, ok, err := s.modifiersForTarget(ctx, c, def, t, opts)
			if err != nil {
				return err
			}
			if ok {
				groups, mods = append(groups, []playdb.Combatant{t}), append(mods, m)
			}
		}
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

// noEffectWhy says why a spell takes no hold of a target, "" when it may: a spell that picks a
// humanoid (Imobilizar Pessoa), a condition the creature is immune to, a creature too simple for
// Riso Histérico (SRD 5.1: an Intelligence score of 4 or less).
func (s *Service) noEffectWhy(content *rules.Content, def *rules.EffectDef, monsterKey string) string {
	var creature rules.Creature
	known := false
	if monsterKey != "" {
		creature, known = content.CreatureByKey(monsterKey)
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
