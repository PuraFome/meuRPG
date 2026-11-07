package play

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/conv"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// addStatBlockProfiles reads the stat block of each monster NPC in npcs for the
// master's card: its legendary actions, Legendary Resistance and limited actions. The rules content
// is read only when some NPC is a monster; a content that can't be read leaves the
// cards without them, as a sheet that can't be read leaves a card without a portrait.
func (s *Service) addStatBlockProfiles(ctx context.Context, campaignID string, npcs map[string]masterNPC) {
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
		if p, ok := content.LegendaryProfile(n.monsterKey); ok {
			if p.IsLegendary() {
				n.legendary = p
			}
			n.parry = slices.ContainsFunc(p.Reactions, func(r rules.MonsterReaction) bool { return r.Kind == rules.ReactionKindParry && r.ACBonus > 0 })
		}
		if plan, ok := content.MonsterPlan(n.monsterKey); ok {
			n.limited = limitedActionsOf(plan)
			n.legendaryDetail = legendaryAttackDetails(n.legendary, plan)
		}
		npcs[id] = n
	}
}

// legendaryView is the master's copy of what a legendary creature has left (RN-20,
// RN-29), from its stat block's profile and the creature_spent it keeps. nil for a
// creature without legendary actions or resistance: the field is then absent.
func legendaryView(p rules.LegendaryProfile, details map[string]string, who playdb.Combatant, enc playdb.Encounter) *playv1.LegendaryState {
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
			Key: o.Key, Name: o.Name, NamePt: o.NamePT, AttackKey: o.ActionKey, Cost: conv.Int32(o.Cost), Kind: o.Kind, Text: o.Text,
			Affordable: o.Cost <= int(out.ActionsLeft), AttackDetailPt: details[o.Key],
		})
	}
	return out
}

// legendaryAttackDetails is, by option key, the numbers of the attack each "attack" option makes,
// in Portuguese: "Cauda: +14 · 2d8 + 8 de concussão · alcance 4,5 m". It is the master's
// (LegendaryOption.attack_detail_pt). An option of another kind, or whose action has no
// attack roll in the plan, has none.
func legendaryAttackDetails(p rules.LegendaryProfile, plan *rules.MonsterPlan) map[string]string {
	var out map[string]string
	for _, o := range p.Options {
		if o.Kind != "attack" || o.ActionKey == "" {
			continue
		}
		a, ok := plan.Action(o.ActionKey)
		if !ok || a.Attack == nil {
			continue
		}
		var dmg []string
		for _, d := range a.Damage {
			part := fmt.Sprintf("%dd%d", d.Dice.Count, d.Dice.Sides)
			if d.Dice.Count == 0 {
				part = strconv.Itoa(d.Dice.Bonus)
			} else if d.Dice.Bonus > 0 {
				part += fmt.Sprintf(" + %d", d.Dice.Bonus)
			} else if d.Dice.Bonus < 0 {
				part += fmt.Sprintf(" - %d", -d.Dice.Bonus)
			}
			dmg = append(dmg, part+" de "+d.TypeNamePT)
		}
		reach := "alcance " + metersPT(int32(a.Attack.ReachFt))
		if !a.Attack.Melee {
			reach = "alcance " + metersPT(int32(a.Attack.RangeFt))
			if a.Attack.LongRangeFt > 0 {
				reach += " / " + metersPT(int32(a.Attack.LongRangeFt))
			}
		}
		name := a.NamePT
		if name == "" {
			name = a.Name
		}
		parts := []string{fmt.Sprintf("%s: %+d", name, a.Attack.Bonus)}
		if len(dmg) > 0 {
			parts = append(parts, strings.Join(dmg, " + "))
		}
		parts = append(parts, reach)
		if out == nil {
			out = map[string]string{}
		}
		out[o.Key] = strings.Join(parts, " · ")
	}
	return out
}

// creatureLimits are the limited actions of the creatures of a stat block that the viewer reads: all of them for the
// master, the ones the player controls for a player. It holds nothing else of the master's NPC data (RN-29).
func (s *Service) creatureLimits(ctx context.Context, campaignID string, d *encounterData, v combatViewer) map[string]masterNPC {
	var mine []playdb.Combatant
	for _, c := range d.cs {
		if isCreature(c) && (v.master || v.owns(c)) && conv.OrZero(c.MonsterKey) != "" {
			mine = append(mine, c)
		}
	}
	if len(mine) == 0 {
		return nil
	}
	content, err := s.roster.RulesContent(ctx, nil, campaignID)
	if err != nil {
		return nil
	}
	out := make(map[string]masterNPC, len(mine))
	for _, c := range mine {
		if plan, ok := content.MonsterPlan(conv.OrZero(c.MonsterKey)); ok {
			out[sheetKey(c)] = masterNPC{limited: limitedActionsOf(plan)}
		}
	}
	return out
}
