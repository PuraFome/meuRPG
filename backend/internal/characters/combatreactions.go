package characters

import (
	"context"
	"slices"

	"github.com/jackc/pgx/v5"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	"github.com/PuraFome/meuRPG/backend/internal/characters/charactersdb"
	"github.com/PuraFome/meuRPG/backend/internal/platform/uid"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// ReactionStats implements play.CombatRoster: what the combatant's sheet says
// about its reactions (PM-04). A full sheet gives its spells, slots, the casting
// numbers and the features (Uncanny Dodge, Deflect Missiles, Cutting Words and the
// tiefling's Infernal Legacy); a basic sheet made from a creature gives the
// Spellcasting trait of its stat block (slots) and the uses of its Innate
// Spellcasting; any other basic sheet has none.
func (s *Service) ReactionStats(ctx context.Context, tx pgx.Tx, campaignID, characterID string) (link.ReactionStats, error) {
	_, d, content, err := s.fighter(ctx, tx, campaignID, characterID)
	if err != nil {
		return link.ReactionStats{}, err
	}
	id, _ := uid.Canonical(characterID)
	rows, err := s.queriesIn(tx).ListCombatCharacters(ctx, charactersdb.ListCombatCharactersParams{CampaignID: campaignID, Ids: []string{id}})
	if err != nil {
		return link.ReactionStats{}, s.dbError(ctx, "read a character for a combat", err)
	}
	var sheet *charactersv1.CharacterSheet
	if len(rows) > 0 {
		if sheet, err = loadSheet(rows[0].ID, rows[0].Sheet); err != nil {
			return link.ReactionStats{}, s.dbError(ctx, "read a character for a combat", err)
		}
	}
	if basic := sheet.GetBasic(); basic != nil {
		return creatureReactionStats(content, basic.GetMonsterKey()), nil
	}
	return fullReactionStats(d, sheet.GetFull()), nil
}

// creatureReactionStats reads a stat block's reactions: its Spellcasting trait, its innate
// spells with uses a day, and whether it is immune to being charmed.
func creatureReactionStats(content *rules.Content, monsterKey string) link.ReactionStats {
	out := link.ReactionStats{StatBlock: true}
	if monsterKey == "" {
		return out
	}
	if cr, ok := content.CreatureByKey(monsterKey); ok {
		out.CharmImmune = slices.ContainsFunc(cr.ConditionImmunities, func(n rules.NamedKey) bool { return n.Key == "condition:charmed" })
	}
	if c, ok := content.MonsterSpellcasting(monsterKey); ok {
		out.Spells, out.SlotsTotal, out.CastingMod, out.SaveDC = c.Spells, c.Slots, c.AbilityMod, c.SaveDC
		for _, in := range c.Innate {
			out.Innate = append(out.Innate, link.InnateUse{Spell: in.Spell, Resource: in.Resource, Max: in.Max})
			if out.ResourceMax == nil {
				out.ResourceMax = map[string]int{}
			}
			out.ResourceMax[in.Resource] = in.Max
		}
	}
	if p, ok := content.LegendaryProfile(monsterKey); ok {
		for _, r := range p.Reactions {
			out.MonsterReactions = append(out.MonsterReactions, link.MonsterReaction{Kind: r.Kind, ACBonus: r.ACBonus})
		}
	}
	return out
}

// fullReactionStats reads a full sheet's reactions from what the rules derived.
func fullReactionStats(d rules.Derived, full *charactersv1.FullSheet) link.ReactionStats {
	out := link.ReactionStats{CuttingAsk: cuttingAskText(full.GetCuttingWordsAsk())}
	reactionSpellcasting(&out, d)
	for _, f := range d.Features {
		switch f.Key {
		case "feature:uncanny-dodge":
			out.UncannyDodge = true
		case "feature:deflect-missiles":
			out.Deflect = true
		case "feature:cutting-words":
			out.CuttingWords = true
		case "trait:infernal-legacy":
			out.InfernalLegacy = d.TotalLevel >= infernalLegacyLevel
		case "feature:fighter-fighting-style-protection", "feature:fighting-style-protection":
			out.Protection = full.GetShield() && !d.ShieldIdle
		}
	}
	out.MonkLevel, _ = lastClassLevel(d, "class:monk")
	out.BardLevel, _ = lastClassLevel(d, "class:bard")
	out.DexMod = abilityMod(d, rules.DEX)
	for _, m := range d.Maneuvers {
		if m.Applies == rules.ManeuverReduceMelee {
			out.Reduce = append(out.Reduce, link.ReduceManeuver{Key: m.Key, NamePT: m.NamePT, Resource: m.Resource, Sides: m.Sides, Mod: abilityMod(d, rules.Ability(m.Ability))})
		}
	}
	out.Proficiency = d.ProficiencyBonus
	// Charisma is the spellcasting ability of the Infernal Legacy.
	out.LegacyDC = 8 + d.ProficiencyBonus + abilityMod(d, rules.CHA)
	return out
}

// reactionSpellcasting sets what a full sheet casts a reaction spell with: the
// prepared spells, the slots and pact slots, the uses of each resource, the best
// save DC and the spellcasting modifier.
func reactionSpellcasting(out *link.ReactionStats, d rules.Derived) {
	for _, sp := range d.Spells {
		if sp.Prepared {
			out.Spells = append(out.Spells, sp.Spell.Key)
		}
	}
	copy(out.SlotsTotal[:], d.SpellSlots)
	out.ResourceMax = map[string]int{}
	for _, r := range d.Resources {
		out.ResourceMax[r.Key] = r.Max
	}
	if d.PactMagic != nil {
		out.PactLevel, out.PactSlots = d.PactMagic.SlotLevel, d.PactMagic.Slots
	}
	for _, sc := range d.Spellcasting {
		out.SaveDC = max(out.SaveDC, sc.SaveDC)
	}
	if c := casterFor(d, nil); c != nil {
		out.CastingMod = abilityMod(d, c.Ability)
	}
}

// infernalLegacyLevel is the character level the tiefling casts Hellish Rebuke
// with its trait from (SRD, Infernal Legacy).
const infernalLegacyLevel = 3

// cuttingAskText is the bard's "Perguntar" setting as the combat reads it.
func cuttingAskText(a charactersv1.CuttingWordsAsk) string {
	switch a {
	case charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL:
		return "all"
	case charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_NEVER:
		return "never"
	}
	return "attacks"
}
