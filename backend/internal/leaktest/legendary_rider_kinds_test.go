package leaktest

import (
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1/playv1connect"
)

// The other riders held for Legendary Resistance: a creature's rider (the pit fiend's bite against the dragon that fights
// on the party's side), answered by the master alone. The invariants are those of the Stunning Strike rows of
// legendary_rider_test.go (RN-20, RN-29): the window, the offer, the failed roll, the use spent and the uses left are the
// master's; the players read the wait any held action gives and, after, an ordinary save line, the same for a save that
// passed and one that was made a success.

// creatureRiderScene has the master's pit fiend bite the dragon, which fights on the party's side, and returns the open
// offer of the bite's rider. The master gives the answer.
func (w *world) creatureRiderScene() riderScene {
	w.t.Helper()
	ctx := w.t.Context()
	m := w.master
	fiend := w.monsterOnTurn("monster:pit-fiend", w.secrets.public("fiend-name"), false)
	w.seenByAna()
	var dragon *playv1.Combatant
	for _, c := range m.currentEncounter(w).GetCombatants() {
		if c.GetBestiaryCreatureKey() == dragonKey {
			dragon = c
		}
	}
	must(m.combat.MoveCombatant(ctx, rq(&playv1.MoveCombatantRequest{CampaignId: w.campaign, EncounterId: w.encounter.GetId(), CombatantId: dragon.GetId(), IdempotencyKey: newKey(), Col: 13, Row: 13, Forced: true})))
	must(m.combat.SetCombatantSide(ctx, rq(&playv1.SetCombatantSideRequest{CampaignId: w.campaign, EncounterId: w.encounter.GetId(), CombatantId: dragon.GetId(), IdempotencyKey: newKey(), Side: playv1.CombatantSide_COMBATANT_SIDE_PARTY})))
	hit := must(m.combat.RollAttack(ctx, rq(&playv1.RollAttackRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), AttackerId: fiend.GetId(), AttackKey: "basic:0", TargetId: dragon.GetId(),
		IdempotencyKey: newKey(), Roll: &playv1.RollAttackRequest_D20Face{D20Face: 19},
	})))
	must(m.combat.RollDamage(ctx, rq(&playv1.RollDamageRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), PendingDamageId: hit.GetPendingDamage().GetId(), IdempotencyKey: newKey(),
		Roll: &playv1.RollDamageRequest_TypedSum{TypedSum: 5},
	})))
	var rider string
	for _, o := range must(m.combat.GetEncounter(ctx, rq(&playv1.GetEncounterRequest{CampaignId: w.campaign}))).GetEncounter().GetHitRiders() {
		if o.GetKind() == playv1.HitRiderKind_HIT_RIDER_KIND_CREATURE {
			rider = o.GetId()
		}
	}
	if rider == "" {
		w.t.Fatal("the bite left no creature rider")
	}
	answer := func() reply {
		return m.call(playv1connect.CombatServiceUseHitRiderProcedure, &playv1.UseHitRiderRequest{
			CampaignId: w.campaign, EncounterId: w.encounter.GetId(), RiderId: rider, Choice: playv1.HitRiderChoice_HIT_RIDER_CHOICE_APPLY, IdempotencyKey: newKey(),
		})
	}
	return riderScene{dragon: dragon, answer: answer, reader: w.ana}
}

// The pit fiend's bite rider at the dragon fails the save with a 3 and the master turns it into a success: see
// TestARidersLegendaryResistanceIsTheMastersAlone, with the master as the one who answers the rider.
func TestACreatureRidersLegendaryResistanceIsTheMastersAlone(t *testing.T) {
	checkRiderLeak(t, (*world).creatureRiderScene)
}

// A dragon with a use left holds the creature's rider whatever it rolled, and the players read the same either way.
func TestAPassedAndAResistedCreatureRiderSaveReadTheSameToThePlayers(t *testing.T) {
	checkPassedAndResistedRider(t, (*world).creatureRiderScene)
}

// openHandScene has Caio's monk of the Open Hand strike the dragon, then strike again with Flurry of Blows, and returns
// the Open Hand offer to knock it prone (a Dexterity save). Caio answers it.
func (w *world) openHandScene() riderScene {
	w.t.Helper()
	ctx := w.t.Context()
	toren, dragon, _ := w.monkStrikeFight(true)
	extra := must(w.caio.combat.RollAttack(ctx, rq(&playv1.RollAttackRequest{ // the Extra Attack of the Attack action
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), AttackerId: toren.GetId(), AttackKey: "attack:unarmed-strike", TargetId: dragon.GetId(),
		IdempotencyKey: newKey(), Roll: &playv1.RollAttackRequest_D20Face{D20Face: 18},
	})))
	must(w.master.combat.DiscardPendingDamage(ctx, rq(&playv1.DiscardPendingDamageRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), PendingDamageId: extra.GetPendingDamage().GetId(), IdempotencyKey: newKey(),
	})))
	must(w.caio.combat.TakeAction(ctx, rq(&playv1.TakeActionRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), CombatantId: toren.GetId(), ActionKey: "feature:flurry-of-blows", IdempotencyKey: newKey(),
	})))
	strike := must(w.caio.combat.RollAttack(ctx, rq(&playv1.RollAttackRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), AttackerId: toren.GetId(), AttackKey: "attack:unarmed-strike", TargetId: dragon.GetId(),
		IdempotencyKey: newKey(), Roll: &playv1.RollAttackRequest_D20Face{D20Face: 18},
	})))
	must(w.master.combat.DiscardPendingDamage(ctx, rq(&playv1.DiscardPendingDamageRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), PendingDamageId: strike.GetPendingDamage().GetId(), IdempotencyKey: newKey(),
	})))
	rider := ""
	for _, o := range must(w.caio.combat.GetEncounter(ctx, rq(&playv1.GetEncounterRequest{CampaignId: w.campaign}))).GetEncounter().GetHitRiders() {
		if o.GetKind() == playv1.HitRiderKind_HIT_RIDER_KIND_OPEN_HAND {
			rider = o.GetId()
			continue
		}
		// The Stunning Strike offers of the earlier strikes are declined: the offers of one instant read in no fixed order.
		must(w.caio.combat.UseHitRider(ctx, rq(&playv1.UseHitRiderRequest{
			CampaignId: w.campaign, EncounterId: w.encounter.GetId(), RiderId: o.GetId(), Choice: playv1.HitRiderChoice_HIT_RIDER_CHOICE_DECLINE, IdempotencyKey: newKey(),
		})))
	}
	if rider == "" {
		w.t.Fatalf("the flurry strike left no Open Hand offer: %v", must(w.caio.combat.GetEncounter(ctx, rq(&playv1.GetEncounterRequest{CampaignId: w.campaign}))).GetEncounter().GetHitRiders())
	}
	answer := func() reply {
		return w.caio.call(playv1connect.CombatServiceUseHitRiderProcedure, &playv1.UseHitRiderRequest{
			CampaignId: w.campaign, EncounterId: w.encounter.GetId(), RiderId: rider, Choice: playv1.HitRiderChoice_HIT_RIDER_CHOICE_PRONE, IdempotencyKey: newKey(),
		})
	}
	return riderScene{dragon: dragon, answer: answer, owner: w.caio, reader: w.caio}
}

// The Open Hand rider that knocks the dragon prone fails the Dexterity save with a 1 and the master turns it into a
// success: see TestARidersLegendaryResistanceIsTheMastersAlone.
func TestAnOpenHandRidersLegendaryResistanceIsTheMastersAlone(t *testing.T) {
	checkRiderLeak(t, (*world).openHandScene)
}

// The same wait, log and answer for the Open Hand rider whether the save passed or was made a success.
func TestAPassedAndAResistedOpenHandRiderSaveReadTheSameToThePlayers(t *testing.T) {
	checkPassedAndResistedRider(t, (*world).openHandScene)
}
