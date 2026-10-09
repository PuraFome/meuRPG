package characters

import (
	"context"
	"slices"

	"github.com/jackc/pgx/v5"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	"github.com/PuraFome/meuRPG/backend/internal/characters/charactersdb"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// ReactionStats implements play.CombatRoster: what the combatant's sheet says
// about its reactions (PM-04). A full sheet gives its spells, slots, the casting
// numbers and the features (Uncanny Dodge, Deflect Missiles, Cutting Words and the
// tiefling's Infernal Legacy); a basic sheet made from a creature gives the
// slot-based Spellcasting trait of its stat block; any other basic sheet has none.
func (s *Service) ReactionStats(ctx context.Context, tx pgx.Tx, campaignID, characterID string) (link.ReactionStats, error) {
	_, d, content, err := s.fighter(ctx, tx, campaignID, characterID)
	if err != nil {
		return link.ReactionStats{}, err
	}
	id, _ := parseUUID(characterID)
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

// creatureReactionStats reads a stat block's reactions: its Spellcasting trait,
// and whether it is immune to being charmed.
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
	}
	return out
}

// fullReactionStats reads a full sheet's reactions from what the rules derived.
func fullReactionStats(d rules.Derived, full *charactersv1.FullSheet) link.ReactionStats {
	out := link.ReactionStats{CuttingAsk: cuttingAskText(full.GetCuttingWordsAsk())}
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
		}
	}
	for _, cl := range d.Classes {
		switch cl.ClassKey {
		case "class:monk":
			out.MonkLevel = cl.Level
		case "class:bard":
			out.BardLevel = cl.Level
		}
	}
	out.DexMod = abilityMod(d, rules.DEX)
	// Charisma is the spellcasting ability of the Infernal Legacy.
	out.LegacyDC = 8 + d.ProficiencyBonus + abilityMod(d, rules.CHA)
	return out
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
