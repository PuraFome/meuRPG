package play

import (
	"context"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/conv"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// addLegendaryProfiles reads the stat block of each monster NPC in npcs for the
// master's card: its legendary actions and Legendary Resistance. The rules content
// is read only when some NPC is a monster; a content that can't be read leaves the
// cards without them, as a sheet that can't be read leaves a card without a portrait.
func (s *Service) addLegendaryProfiles(ctx context.Context, campaignID string, npcs map[string]masterNPC) {
	hasMonster := false
	for _, n := range npcs {
		hasMonster = hasMonster || n.monsterKey != ""
	}
	if !hasMonster {
		return
	}
	content, err := s.roster.RulesContent(ctx, nil, campaignID)
	if err != nil {
		return
	}
	for id, n := range npcs {
		if p, ok := content.LegendaryProfile(n.monsterKey); ok && p.IsLegendary() {
			n.legendary = p
			npcs[id] = n
		}
	}
}

// legendaryView is the master's copy of what a legendary creature has left (RN-20,
// RN-29), from its stat block's profile and the creature_spent it keeps. nil for a
// creature without legendary actions or resistance: the field is then absent.
func legendaryView(p rules.LegendaryProfile, who playdb.Combatant, enc playdb.Encounter) *playv1.LegendaryState {
	if !p.IsLegendary() {
		return nil
	}
	spent := creatureSpentOf(who.CreatureSpent)
	out := &playv1.LegendaryState{
		ActionsPerRound:  conv.Int32(p.PerRound),
		ActionsLeft:      conv.Int32(usesLeft(p.PerRound, spent[spentLegendaryActions])),
		ResistancePerDay: conv.Int32(p.ResistancePerDay),
		ResistanceLeft:   conv.Int32(usesLeft(p.ResistancePerDay, spent[spentLegendaryResistance])),
		UsableNow:        legendaryNow(enc, who, p) == nil,
	}
	for _, o := range p.Options {
		out.Options = append(out.Options, &playv1.LegendaryOption{
			Key: o.Key, Name: o.Name, Cost: conv.Int32(o.Cost), Kind: o.Kind, Text: o.Text,
			Affordable: o.Cost <= int(out.ActionsLeft),
		})
	}
	return out
}
