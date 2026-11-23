package play

import (
	"testing"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// bardTableWith is bardTable with Pipo (Caio's) made by pipo, to give him a feature that asks about the roll.
func bardTableWith(t *testing.T, pipo func(a *armed) *charactersv1.Character) (*armed, *playv1.Encounter) {
	t.Helper()
	fighter := &rulesv1.AbilityScores{Strength: 16, Dexterity: 12, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 10}
	var bard *charactersv1.Character
	a := newArmedWith(t, func(a *armed) {
		a.toren = pipo(a)
		a.pens = a.ana.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 2, fighter, []string{battleaxe}, nil)
		a.bri = a.bia.hero(t, a.campaignID, "Orla", "class:bard", "race:human", 5,
			&rulesv1.AbilityScores{Strength: 8, Dexterity: 14, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 16}, nil, nil)
		bard = loreBardEnemy(t, a)
	})
	e := a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: bard.GetId()}}, npcRolls: []int{3},
		players: map[string]int32{"Pipo": 20, "Toren": 14, "Orla": 10}, reveal: []string{"Bardo"},
		at: map[string][2]int32{"Pipo": {4, 3}, "Toren": {3, 4}, "Orla": {5, 3}, "Bardo": {4, 4}},
	})
	return a, e
}

// A rogue of level 20 whose replayed attack misses is asked about Stroke of Luck: the roll is kept, so the cut line waits for the answer
// and completes on the attack that answer writes (a miss that became a hit, with the die back on the total).
func TestACutLineWaitsForTheStrokeOfLuckAnswerOfAHeldAttack(t *testing.T) {
	t.Parallel()
	a, e := bardTableWith(t, func(a *armed) *charactersv1.Character {
		rogue := &rulesv1.AbilityScores{Strength: 16, Dexterity: 16, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 10}
		return a.caio.hero(t, a.campaignID, "Pipo", "class:rogue", "race:human", 20, rogue, []string{battleaxe}, nil)
	})
	offer := heldWeaponCut(t, a, e, d20(10))
	if offer.GetStrokeOfLuck() == nil {
		t.Fatalf("Pipo's question = %v, want Stroke of Luck's", offer)
	}
	a.wantLines(t, e, a.bardTableReaders(), cutPendingMaster, cutPendingPlayers)
	if _, err := a.answerHeldRoll(t, a.caio, offer.GetHoldId(), strokeAnswer(true)); err != nil {
		t.Fatalf("AnswerHeldRoll() error = %v", err)
	}
	a.wantLines(t, e, a.bardTableReaders(), strokeMaster, strokePlayers)
}

// A reroll rule on the d20 asks the same way: the line waits for the answer, then completes on the rerolled attack.
func TestACutLineWaitsForTheRerollAnswerOfAHeldAttack(t *testing.T) {
	t.Parallel()
	r := &rerollTable{}
	var e *playv1.Encounter
	r.armed, e = bardTableWith(t, func(a *armed) *charactersv1.Character {
		r.feat = a.rerollFeat(t, d20RerollEffects("2", 2, "1")...)
		pipo := a.caio.hero(t, a.campaignID, "Pipo", "class:fighter", "race:human", 2,
			&rulesv1.AbilityScores{Strength: 16, Dexterity: 12, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 10}, []string{battleaxe}, nil)
		a.giveFeat(t, pipo, r.feat)
		return pipo
	})
	r.h.svc.d20Rerolls = true
	offer := heldWeaponCut(t, r.armed, e, d20(2))
	if offer.GetReroll() == nil {
		t.Fatalf("Pipo's question = %v, want the reroll's", offer)
	}
	r.wantLines(t, e, r.bardTableReaders(), cutPendingMaster, cutPendingPlayers)
	r.h.roller.queue(17)
	if _, err := r.answerHeldRoll(t, r.caio, offer.GetHoldId(), rerollAnswer(true)); err != nil {
		t.Fatalf("AnswerHeldRoll() error = %v", err)
	}
	r.wantLines(t, e, r.bardTableReaders(), rerollMaster, rerollPlayers)
}

const (
	strokeMaster  = "Bardo usou Palavras de Interrupção: d6 (5) subtraído do ataque de Pipo: 13 → 8 contra CA 12: acertou."
	strokePlayers = "Bardo usou Palavras de Interrupção: o ataque de Pipo ainda acertou."
	rerollMaster  = "Bardo usou Palavras de Interrupção: d6 (5) subtraído do ataque de Pipo: 22 → 17 contra CA 12: acertou."
	rerollPlayers = strokePlayers
)
