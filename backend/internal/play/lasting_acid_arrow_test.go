package play

import (
	"strings"
	"testing"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// Acid Arrow (SRD 5.1): a ranged spell attack. A hit deals the initial acid damage at once and the
// later acid damage at the end of the target's next turn; a miss deals half the initial damage and
// leaves nothing for later. A higher slot adds a die to both parts.
const (
	acidArrowKey = "spell:acid-arrow"
	// acidArrowSlot is the spell's own level.
	acidArrowSlot = 2
	// acidArrowHitFace and acidArrowMissFace are d20 faces against the ogre's armor class 12 with
	// the wizard's +5 to hit.
	acidArrowHitFace  = 15
	acidArrowMissFace = 2
)

// newAcidWizards is the party of these tests: Pensantus, a 5th-level wizard who knows Acid Arrow
// (three 2nd-level and two 3rd-level slots), and an ogre with 60 hit points that plays last, after the party.
func newAcidWizards(t *testing.T) (*armed, *charactersv1.Character) {
	t.Helper()
	var ogre *charactersv1.Character
	a := newArmedWith(t, func(a *armed) {
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 5,
			&rulesv1.AbilityScores{Strength: 16, Dexterity: 13, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}, []string{battleaxe}, nil)
		a.pens = a.ana.caster(t, a.campaignID, "Pensantus", "class:wizard", "race:gnome", 5,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 14, Constitution: 12, Intelligence: 16, Wisdom: 10, Charisma: 8}, nil, []string{fireBolt},
			[]string{acidArrowKey}, []string{acidArrowKey})
		a.bri = a.bia.caster(t, a.campaignID, "Brisa", "class:cleric", "race:human", 3,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 16, Constitution: 14, Intelligence: 10, Wisdom: 16, Charisma: 8}, []string{maceKey}, []string{sacredFlame}, nil,
			[]string{cureWounds})
		ogre = a.master.npc(t, a.campaignID, "Ogre", 60, 12)
	})
	return a, ogre
}

// acidFight starts the combat: Pensantus first, then Toren and Brisa, the ogre last, 30 feet from
// Pensantus (no creature next to the wizard, so the ranged attack rolls straight).
func (a *armed) acidFight(t *testing.T, ogre *charactersv1.Character) *playv1.Encounter {
	t.Helper()
	return a.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: ogre.GetId()}},
		npcRolls: []int{1},
		players:  map[string]int32{"Pensantus": 20, "Toren": 15, "Brisa": 10},
		reveal:   []string{"Ogre"},
		at:       map[string][2]int32{"Pensantus": {5, 5}, "Toren": {6, 5}, "Brisa": {5, 6}, "Ogre": {11, 5}},
	})
}

// arrow has Pensantus cast Acid Arrow at the ogre with the d20 face, at the slot level.
func (a *armed) arrow(t *testing.T, e *playv1.Encounter, slot, face int32) *playv1.CastSpellResponse {
	t.Helper()
	return a.mustCast(t, a.ana, e, "Pensantus", acidArrowKey, slotOfLevel(slot), a.at(t, "Ogre"), d20Face(face))
}

// ogreArrow is the Acid Arrow card on the ogre as the master reads it; nil when there is none.
func (a *armed) ogreArrow(t *testing.T) *playv1.LastingEffect {
	t.Helper()
	return cardOf(t, byLabel(t, a.get(t, a.master), "Ogre"), acidArrowKey)
}

// A miss: the spell deals half the initial 4d4 (rounded down) and leaves no card, so nothing lands
// at the end of the ogre's turn.
func TestAcidArrowMissDealsHalfTheInitialDamageAndSchedulesNothing(t *testing.T) {
	t.Parallel()
	a, ogre := newAcidWizards(t)
	e := a.acidFight(t, ogre)
	cast := a.arrow(t, e, acidArrowSlot, acidArrowMissFace)
	if got := cast.GetCast().GetTargets()[0].GetOutcome(); got != playv1.AttackOutcome_ATTACK_OUTCOME_MISS {
		t.Fatalf("the arrow's outcome = %v, want a miss", got)
	}
	pending := cast.GetCast().GetPendingDamages()
	if len(pending) != 1 || !pending[0].GetHalf() || pending[0].GetDiceCount() != 4 || pending[0].GetDiceSides() != 4 {
		t.Fatalf("the pending damage of the miss = %v, want one half-damage 4d4", pending)
	}
	a.h.roller.queue(3, 3, 3, 3) // 12, half is 6
	if d := a.mustDamage(t, a.ana, e, pending[0].GetId(), inAppDamage).GetPendingDamage(); d.GetAmount() != 6 {
		t.Fatalf("the splash = %d, want 6 (half of 12)", d.GetAmount())
	}
	if cur, _, _ := a.hp(t, "Ogre"); cur != 54 {
		t.Fatalf("the ogre's hit points = %d, want 54", cur)
	}
	if card := a.ogreArrow(t); card != nil {
		t.Fatalf("the ogre has an Acid Arrow card after a miss: %v", card)
	}
	e = a.passTo(t, e, "Ogre")
	a.mustEndTurn(t, a.master, e)
	if cur, _, _ := a.hp(t, "Ogre"); cur != 54 {
		t.Errorf("the ogre's hit points after its turn = %d, want 54: a miss leaves nothing for later", cur)
	}
}

// A hit: the full initial damage at once, a card on the ogre that ends at the end of its next turn
// with the 2d4 acid it brings, and nothing lands at the end of the caster's turn or the start of
// the ogre's.
func TestAcidArrowHitDealsTheRestAtTheEndOfTheTargetsNextTurn(t *testing.T) {
	t.Parallel()
	a, ogre := newAcidWizards(t)
	e := a.acidFight(t, ogre)
	cast := a.arrow(t, e, acidArrowSlot, acidArrowHitFace)
	a.h.roller.queue(4, 4, 4, 4) // 16
	if d := a.mustDamage(t, a.ana, e, cast.GetCast().GetPendingDamages()[0].GetId(), inAppDamage).GetPendingDamage(); d.GetAmount() != 16 {
		t.Fatalf("the initial damage = %d, want 16", d.GetAmount())
	}
	card := a.ogreArrow(t)
	if card == nil {
		t.Fatal("the ogre has no Acid Arrow card after a hit")
	}
	if tr := card.GetTurnTrigger(); tr.GetDice() != "2d4" || tr.GetDamageTypeKey() != "damage-type:acid" || tr.GetPhase() != playv1.EffectPhase_EFFECT_PHASE_END {
		t.Errorf("the card's trigger = %v, want 2d4 acid at the end of the turn", tr)
	}
	if at := card.GetEndsAt(); at.GetCombatantId() != a.id(t, "Ogre") || at.GetPhase() != playv1.EffectPhase_EFFECT_PHASE_END || at.GetRound() != 1 {
		t.Errorf("the card ends at %v, want the end of the ogre's turn in round 1", at)
	}

	// The caster's turn ends, then Toren's and Brisa's, then the ogre's turn starts: nothing lands.
	e = a.passTo(t, e, "Ogre")
	if cur, _, _ := a.hp(t, "Ogre"); cur != 44 {
		t.Fatalf("the ogre's hit points at the start of its turn = %d, want 44", cur)
	}
	a.h.roller.queue(3, 4) // 7
	a.mustEndTurn(t, a.master, e)
	if cur, _, _ := a.hp(t, "Ogre"); cur != 37 {
		t.Errorf("the ogre's hit points after its turn = %d, want 37 (44 - 7)", cur)
	}
	if card := a.ogreArrow(t); card != nil {
		t.Errorf("the card after the ogre's turn = %v, want it gone", card)
	}
	// One hit, one later damage: the next full round brings nothing.
	e = a.passTo(t, a.get(t, a.master), "Ogre")
	a.mustEndTurn(t, a.master, e)
	if cur, _, _ := a.hp(t, "Ogre"); cur != 37 {
		t.Errorf("the ogre's hit points a round later = %d, want 37", cur)
	}
}

// A 3rd-level slot adds a die to both parts: 5d4 now and 3d4 at the end of the target's turn.
func TestAcidArrowUpcastAddsADieToBothParts(t *testing.T) {
	t.Parallel()
	a, ogre := newAcidWizards(t)
	e := a.acidFight(t, ogre)
	cast := a.arrow(t, e, acidArrowSlot+1, acidArrowHitFace)
	if p := cast.GetCast().GetPendingDamages()[0]; p.GetDiceCount() != 5 || p.GetDiceSides() != 4 {
		t.Fatalf("the initial damage = %dd%d, want 5d4", p.GetDiceCount(), p.GetDiceSides())
	}
	card := a.ogreArrow(t)
	if card == nil || card.GetTurnTrigger().GetDice() != "3d4" {
		t.Fatalf("the card = %v, want a trigger of 3d4", card)
	}
}

// The master ending the effect, or the target going down, leaves nothing to land.
func TestAcidArrowLaterDamageDoesNotLandOnAnEndedEffect(t *testing.T) {
	t.Parallel()
	a, ogre := newAcidWizards(t)
	e := a.acidFight(t, ogre)
	a.arrow(t, e, acidArrowSlot, acidArrowHitFace)
	if _, err := a.adjustHP(t, e, "Ogre", damageHP(60)); err != nil {
		t.Fatalf("AdjustCombatantHitPoints() error = %v", err)
	}
	if card := a.ogreArrow(t); card != nil {
		t.Fatalf("the card on a defeated ogre = %v, want it gone", card)
	}
	before := a.events(t)
	a.passTo(t, a.get(t, a.master), "Pensantus")
	for _, line := range a.eventTrace(t, a.get(t, a.master), before) {
		if strings.Contains(line, eventLastingTriggered) {
			t.Errorf("event after the ogre went down = %v, want no later damage", line)
		}
	}
}

// The undo of the cast takes the card back, and the pending damage with it.
func TestAcidArrowUndoTakesTheScheduledDamageBack(t *testing.T) {
	t.Parallel()
	a, ogre := newAcidWizards(t)
	e := a.acidFight(t, ogre)
	a.arrow(t, e, acidArrowSlot, acidArrowHitFace)
	if a.ogreArrow(t) == nil {
		t.Fatal("the ogre has no card after a hit")
	}
	a.undoLast(t, e)
	if card := a.ogreArrow(t); card != nil {
		t.Fatalf("the card after the undo = %v, want it gone", card)
	}
	e = a.passTo(t, a.get(t, a.master), "Ogre")
	a.mustEndTurn(t, a.master, e)
	if cur, _, _ := a.hp(t, "Ogre"); cur != 60 {
		t.Errorf("the ogre's hit points = %d, want 60", cur)
	}
}
