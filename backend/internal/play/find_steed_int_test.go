package play

import (
	"slices"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// A steed has an Intelligence of at least 6 (Find Steed), so Hideous Laughter, which does nothing to a creature of 4 or less, takes hold of
// it although the warhorse's own page says 2.
func TestFindSteedIntelligenceFloorLetsHideousLaughterTakeHold(t *testing.T) {
	t.Parallel()
	a := newArmedWith(t, func(a *armed) {
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 5,
			&rulesv1.AbilityScores{Strength: 16, Dexterity: 13, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}, []string{battleaxe}, nil)
		a.pens = a.ana.caster(t, a.campaignID, "Aurélio", "class:paladin", "race:human", 9,
			&rulesv1.AbilityScores{Strength: 16, Dexterity: 10, Constitution: 14, Intelligence: 10, Wisdom: 12, Charisma: 16}, nil, nil,
			[]string{findSteed, coreLaughter}, []string{findSteed, coreLaughter})
	})
	a.mustCastSteed(t, warhorse, "celestial", "Trovão")
	a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: a.goblin.GetId()}}, npcRolls: []int{3},
		players: map[string]int32{"Aurélio": 18, "Toren": 10, "Trovão": 12},
		reveal:  []string{"Goblin"},
		at:      map[string][2]int32{"Aurélio": {3, 3}, "Toren": {6, 3}, "Goblin": {12, 3}, "Trovão": {4, 3}},
	})
	e := a.turnOf(t, "Aurélio")
	a.h.roller.queue(1) // the steed fails its saving throw
	if _, err := a.cast(t, a.ana, e, "Aurélio", coreLaughter, slotOfLevel(1), a.at(t, "Trovão"), noCastRoll); err != nil {
		t.Fatalf("cast Hideous Laughter on the steed error = %v (%v)", err, connect.CodeOf(err))
	}
	steed := byLabel(t, a.get(t, a.master), "Trovão")
	if !slices.Contains(steed.GetConditions(), "condition:prone") || !slices.Contains(steed.GetConditions(), "condition:incapacitated") {
		t.Errorf("the steed's conditions = %v, want prone and incapacitated: its Intelligence is 6", steed.GetConditions())
	}
}
