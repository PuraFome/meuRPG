package play

import (
	"slices"
	"strings"
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// Vicious Mockery (SRD 5.1): a creature that fails the Wisdom saving throw takes psychic damage and
// has disadvantage on the next attack roll it makes before the end of its next turn. The effect is
// the rider of an instantaneous spell: it lasts until the end of the target's next turn, the next
// attack roll spends it (hit or miss), and the master's undo of that roll gives it back.
const viciousMockeryKey = "spell:vicious-mockery"

// newMockers is the party of these tests: Orla, a 3rd-level bard with Charisma 16 (save DC 8 + 2 + 3
// = 13) who knows Vicious Mockery, Toren and Pensantus, against the goblin of the fixture (a basic
// sheet: its saving throws have no bonus; its scimitar is +4).
func newMockers(t *testing.T) *armed {
	t.Helper()
	return newArmedWith(t, func(a *armed) {
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 5,
			&rulesv1.AbilityScores{Strength: 16, Dexterity: 13, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}, []string{battleaxe}, nil)
		a.pens = a.ana.hero(t, a.campaignID, "Pensantus", "class:wizard", "race:gnome", 1,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 14, Constitution: 12, Intelligence: 16, Wisdom: 10, Charisma: 8}, nil, []string{fireBolt})
		a.bri = a.bia.caster(t, a.campaignID, "Orla", "class:bard", "race:human", 3,
			&rulesv1.AbilityScores{Strength: 8, Dexterity: 14, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 16}, []string{rapier}, []string{viciousMockeryKey}, nil, nil)
	})
}

// mockFight starts the combat with the goblin next to Orla (Orla's armor class is 12: no armor,
// Dexterity 14). goblinFirst puts the goblin first in the order; else it plays last, after Orla,
// Toren and Pensantus.
func (a *armed) mockFight(t *testing.T, goblinFirst bool) *playv1.Encounter {
	t.Helper()
	roll := 1
	if goblinFirst {
		roll = 20
	}
	return a.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: a.goblin.GetId()}},
		npcRolls: []int{roll},
		players:  map[string]int32{"Orla": 15, "Toren": 12, "Pensantus": 8},
		reveal:   []string{"Goblin"},
		at:       map[string][2]int32{"Orla": {5, 5}, "Goblin": {6, 5}, "Toren": {3, 3}, "Pensantus": {10, 3}},
	})
}

// mock has Orla cast Vicious Mockery at the goblin, whose saving throw rolls face, and returns the
// goblin's save outcome.
func (a *armed) mock(t *testing.T, e *playv1.Encounter, face int) playv1.SaveOutcome {
	t.Helper()
	a.h.roller.queue(face)
	cast := a.mustCast(t, a.bia, e, "Orla", viciousMockeryKey, nil, a.at(t, "Goblin"), noCastRoll)
	targets := cast.GetCast().GetTargets()
	if len(targets) != 1 {
		t.Fatalf("Vicious Mockery's targets = %v, want the goblin", targets)
	}
	return targets[0].GetSave().GetOutcome()
}

// effectAttackSource is the source of the disadvantage an effect gives the attacker, nil when the
// roll has none.
func effectAttackSource(sources []*playv1.AdvantageSource) *playv1.AdvantageSource {
	for _, s := range sources {
		if s.GetKind() == playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_EFFECT_ATTACK {
			return s
		}
	}
	return nil
}

// goblinMockery is the Vicious Mockery card on the goblin, as the master reads it; nil when gone.
func (a *armed) goblinMockery(t *testing.T) *playv1.LastingEffect {
	t.Helper()
	return cardOf(t, byLabel(t, a.get(t, a.master), "Goblin"), viciousMockeryKey)
}

// The goblin fails the save: its next attack roll has disadvantage, named after the spell; that roll
// spends the effect, so its next one has none, and the card says it was used; the effect ends at the
// end of the goblin's turn.
func TestViciousMockeryGivesTheNextAttackRollDisadvantage(t *testing.T) {
	t.Parallel()
	a := newMockers(t)
	e := a.mockFight(t, false)
	if got := a.mock(t, e, 5); got != playv1.SaveOutcome_SAVE_OUTCOME_FAILED {
		t.Fatalf("the goblin's save (5 against DC 13) = %v, want failed", got)
	}
	card := a.goblinMockery(t)
	if card == nil {
		t.Fatal("the goblin has no Vicious Mockery card after failing the save")
	}
	// The goblin has not played this round: the effect ends at the end of its turn in round 1.
	if card.GetDurationKind() != playv1.EffectDurationKind_EFFECT_DURATION_KIND_UNTIL_END_OF_TURN_OF || card.GetEndsAt().GetRound() != 1 ||
		card.GetEndsAt().GetCombatantId() != a.id(t, "Goblin") || card.GetEndsAt().GetPhase() != playv1.EffectPhase_EFFECT_PHASE_END {
		t.Errorf("the card's clock = %v, ends at %v; want the end of the goblin's turn in round 1", card.GetDurationKind(), card.GetEndsAt())
	}
	if slices.Contains(card.GetTagsPt(), spentTagPT) {
		t.Errorf("the card's tags = %v, want it not used yet", card.GetTagsPt())
	}

	e = a.passTo(t, e, "Goblin")
	a.h.roller.queue(3, 2) // two d20 with disadvantage: the 2 counts, 2 + 4 misses Orla
	res := a.mustAttack(t, a.master, e, "Goblin", sword, "Orla", inAppRoll)
	src := effectAttackSource(res.GetRoll().GetSources())
	if res.GetRoll().GetMode() != playv1.RollMode_ROLL_MODE_DISADVANTAGE || src == nil || src.GetEffect() != playv1.RollMode_ROLL_MODE_DISADVANTAGE {
		t.Fatalf("the goblin's attack = mode %v, sources %v; want disadvantage from the effect", res.GetRoll().GetMode(), res.GetRoll().GetSources())
	}
	if !strings.Contains(src.GetTextPt(), "Zombaria Viciosa") {
		t.Errorf("the source's text = %q, want it to name Zombaria Viciosa", src.GetTextPt())
	}

	// The roll spent it: the next attack roll has none, and the card stays, used, until its end.
	a.h.roller.queue(2)
	again := a.mustAttack(t, a.master, e, "Goblin", sword, "Orla", inAppRoll)
	if again.GetRoll().GetMode() != playv1.RollMode_ROLL_MODE_NORMAL || effectAttackSource(again.GetRoll().GetSources()) != nil {
		t.Errorf("the goblin's second attack = mode %v, sources %v; want a normal roll", again.GetRoll().GetMode(), again.GetRoll().GetSources())
	}
	if card := a.goblinMockery(t); card == nil || !slices.Contains(card.GetTagsPt(), spentTagPT) {
		t.Errorf("the card after the attack = %v, want it there and marked used", card)
	}
	a.mustEndTurn(t, a.master, e)
	if card := a.goblinMockery(t); card != nil {
		t.Errorf("the card after the goblin's turn = %v, want it gone", card)
	}
}

// A goblin that passes the save gets nothing: no card, and its attack has no disadvantage.
func TestViciousMockeryLeavesNothingOnAPassedSave(t *testing.T) {
	t.Parallel()
	a := newMockers(t)
	e := a.mockFight(t, false)
	if got := a.mock(t, e, 18); got != playv1.SaveOutcome_SAVE_OUTCOME_SAVED {
		t.Fatalf("the goblin's save (18 against DC 13) = %v, want saved", got)
	}
	if card := a.goblinMockery(t); card != nil {
		t.Fatalf("the goblin has a Vicious Mockery card after passing the save: %v", card)
	}
	e = a.passTo(t, e, "Goblin")
	a.h.roller.queue(2)
	res := a.mustAttack(t, a.master, e, "Goblin", sword, "Orla", inAppRoll)
	if res.GetRoll().GetMode() != playv1.RollMode_ROLL_MODE_NORMAL || effectAttackSource(res.GetRoll().GetSources()) != nil {
		t.Errorf("the goblin's attack = mode %v, sources %v; want a normal roll", res.GetRoll().GetMode(), res.GetRoll().GetSources())
	}
}

// The master's undo of the attack that spent the disadvantage gives it back: the attack made again
// has it.
func TestUndoingTheMockedAttackGivesTheDisadvantageBack(t *testing.T) {
	t.Parallel()
	a := newMockers(t)
	e := a.mockFight(t, false)
	if got := a.mock(t, e, 5); got != playv1.SaveOutcome_SAVE_OUTCOME_FAILED {
		t.Fatalf("the goblin's save = %v, want failed", got)
	}
	e = a.passTo(t, e, "Goblin")
	a.h.roller.queue(3, 2)
	if res := a.mustAttack(t, a.master, e, "Goblin", sword, "Orla", inAppRoll); effectAttackSource(res.GetRoll().GetSources()) == nil {
		t.Fatalf("the first attack's sources = %v, want the effect's disadvantage", res.GetRoll().GetSources())
	}
	a.undoLast(t, e)
	if card := a.goblinMockery(t); card == nil || slices.Contains(card.GetTagsPt(), spentTagPT) {
		t.Fatalf("the card after the undo = %v, want it there and not used", card)
	}
	a.h.roller.queue(4, 3)
	res := a.mustAttack(t, a.master, e, "Goblin", sword, "Orla", inAppRoll)
	if res.GetRoll().GetMode() != playv1.RollMode_ROLL_MODE_DISADVANTAGE || effectAttackSource(res.GetRoll().GetSources()) == nil {
		t.Errorf("the attack made again = mode %v, sources %v; want the disadvantage back", res.GetRoll().GetMode(), res.GetRoll().GetSources())
	}
}

// A goblin that never attacks loses the effect at the end of its next turn. Cast after it played
// this round, the effect waits for its turn in the next round (SRD 5.1: "before the end of its next
// turn").
func TestViciousMockeryEndsAtTheEndOfTheTargetsNextTurn(t *testing.T) {
	t.Parallel()
	a := newMockers(t)
	e := a.mockFight(t, true)
	e = a.passTo(t, e, "Orla") // the goblin played first
	if got := a.mock(t, e, 5); got != playv1.SaveOutcome_SAVE_OUTCOME_FAILED {
		t.Fatalf("the goblin's save = %v, want failed", got)
	}
	card := a.goblinMockery(t)
	if card == nil || card.GetEndsAt().GetRound() != 2 || card.GetEndsAt().GetCombatantId() != a.id(t, "Goblin") {
		t.Fatalf("the card = %v, want it to end at the goblin's turn in round 2", card)
	}
	e = a.passTo(t, e, "Goblin")
	if a.get(t, a.master).GetRound() != 2 || a.goblinMockery(t) == nil {
		t.Fatalf("at the goblin's turn of round %d the card = %v, want it there in round 2", a.get(t, a.master).GetRound(), a.goblinMockery(t))
	}
	a.mustEndTurn(t, a.master, e)
	if card := a.goblinMockery(t); card != nil {
		t.Errorf("the card after the goblin's turn = %v, want it gone", card)
	}
}

// A spell attack is an attack roll too: Brisa, mocked, casts Guiding Bolt with disadvantage, the
// roll spends the effect, and the undo of the cast gives it back.
func TestASpellAttackSpendsTheMockery(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	// Brisa stands away from the Goblin: next to a hostile creature a ranged attack has disadvantage.
	e := a.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: a.goblin.GetId()}},
		npcRolls: []int{3},
		players:  map[string]int32{"Toren": 18, "Pensantus": 10, "Brisa": 1},
		reveal:   []string{"Goblin"},
		at:       map[string][2]int32{"Toren": {3, 3}, "Goblin": {4, 3}, "Pensantus": {10, 3}, "Brisa": {4, 9}},
	})
	a.mustAddEffect(t, e, viciousMockeryKey, []string{"Brisa"}, a.rounds(t, 1, "Brisa"))
	a.passTo(t, e, "Brisa")
	e = a.get(t, a.master)
	brisaMockery := func() *playv1.LastingEffect {
		return cardOf(t, byLabel(t, a.get(t, a.master), "Brisa"), viciousMockeryKey)
	}
	// Two typed d20, as the disadvantage asks: the 3 counts, 3 + 5 misses the Goblin.
	cast := a.mustCast(t, a.bia, e, "Brisa", guidingBolt, slotOfLevel(1), a.at(t, "Goblin"), func(r *playv1.CastSpellRequest) {
		r.D20Faces = []int32{12, 3}
	})
	targets := cast.GetCast().GetTargets()
	if len(targets) != 1 || effectAttackSource(targets[0].GetAttackSources()) == nil {
		t.Fatalf("Guiding Bolt's target = %v, want the attack's disadvantage from the effect", targets)
	}
	if card := brisaMockery(); card == nil || !slices.Contains(card.GetTagsPt(), spentTagPT) {
		t.Fatalf("Brisa's card after the spell attack = %v, want it marked used", card)
	}
	a.undoLast(t, e)
	if card := brisaMockery(); card == nil || slices.Contains(card.GetTagsPt(), spentTagPT) {
		t.Errorf("Brisa's card after the cast's undo = %v, want it there and not used", card)
	}
}
