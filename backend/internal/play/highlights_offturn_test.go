package play

import (
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules/reaction"
)

// Damage a creature deals outside its own turn (a reaction, an opportunity attack, a zone's
// or an effect's later damage) counts for the creature that dealt it, not for whoever's turn
// it is. Fabricated events, no database; the integration test is at the end of the file.

// rebuked is the event a Hellish Rebuke leaves when its fire landed on an NPC.
func rebuked(actor string, before, lost int32) actionEvent {
	after := hpState{HP: before - lost, Defeated: before-lost == 0}
	return actionEvent{
		Actor: actor, Target: "c-gob", Applied: true, Before: &hpState{HP: before}, After: &after,
		Reaction: &reactionEvent{Kind: string(reaction.HellishRebukeKind), Used: true, Fire: lost},
	}
}

func TestTallyHighlights_ReactionDamageCountsForTheReactor(t *testing.T) {
	t.Parallel()
	f := &fabric{}
	f.add(eventDamageRolled, land("c-toren", 30, 5))         // the cantrip, on his turn
	f.add(eventReactionAnswered, rebuked("c-toren", 25, 10)) // Hellish Rebuke, on the goblin's turn
	got := tallyOf(f)["Toren"]
	if got.damage != 15 {
		t.Errorf("damage dealt = %d, want 15 (the cantrip's 5 and the Rebuke's 10)", got.damage)
	}
}

func TestTallyHighlights_ReactionFinalBlowAndTheFirstStepOfTheRebuke(t *testing.T) {
	t.Parallel()
	f := &fabric{}
	// The first step (the NPC's save is asked) carries no damage; the second does.
	f.add(eventReactionAnswered, actionEvent{Actor: "c-toren", Target: "c-gob", Reaction: &reactionEvent{Kind: string(reaction.HellishRebukeKind), Used: true}})
	f.add(eventReactionAnswered, rebuked("c-toren", 7, 7))
	got := tallyOf(f)["Toren"]
	if got.damage != 7 || got.finalBlows != 1 {
		t.Errorf("Toren = %+v, want 7 damage and a final blow", got)
	}
}

func TestTallyHighlights_RebukeOnAnAllyAndWithoutASnapshotCountForNothing(t *testing.T) {
	t.Parallel()
	f := &fabric{}
	ally := rebuked("c-toren", 20, 4)
	ally.Target = "c-bri" // friendly fire
	f.add(eventReactionAnswered, ally)
	// An event written before the snapshot existed: the fire is not the hit points lost.
	old := rebuked("c-toren", 20, 4)
	old.Before, old.After, old.Applied = nil, nil, false
	f.add(eventReactionAnswered, old)
	if got := tallyOf(f)["Toren"]; got.damage != 0 || got.finalBlows != 0 {
		t.Errorf("Toren = %+v, want nothing", got)
	}
}

func TestTallyHighlights_OpportunityAttackCountsForTheReactor(t *testing.T) {
	t.Parallel()
	f := &fabric{}
	// Brisa's attack and its damage, made on the goblin's turn (its move provoked it).
	f.add(eventAttackRolled, actionEvent{Actor: "c-bri", Target: "c-gob", Outcome: outcomeCrit, OfferID: "offer-1"})
	f.add(eventDamageRolled, land("c-bri", 12, 8))
	f.add(eventDamageRolled, land("c-toren", 4, 4)) // Toren, on his own turn
	got := tallyOf(f)
	if got["Brisa"].damage != 8 || got["Brisa"].crits != 1 || got["Toren"].damage != 4 || got["Toren"].finalBlows != 1 {
		t.Errorf("Brisa = %+v, Toren = %+v; want 8 damage and a crit, and 4 damage and a final blow", got["Brisa"], got["Toren"])
	}
}

func TestTallyHighlights_ZoneDamageCountsForTheCaster(t *testing.T) {
	t.Parallel()
	f := &fabric{}
	zone := land("c-bri", 20, 6) // Brisa's zone caught the goblin on its turn
	f.add(eventZoneTriggered, zone)
	friendly := land("c-bri", 20, 6)
	friendly.Target = "c-toren"
	f.add(eventZoneTriggered, friendly)
	if got := tallyOf(f)["Brisa"]; got.damage != 6 {
		t.Errorf("Brisa damage = %d, want 6 (the goblin's; the ally's is friendly fire)", got.damage)
	}
}

func TestTallyHighlights_LastingEffectDamageCountsForTheCaster(t *testing.T) {
	t.Parallel()
	f := &fabric{}
	tick := land("c-gob", 20, 5) // the effect's event names the victim as the actor
	tick.Lasting = &lastingEvent{Key: "web", Change: "damage", Caster: "c-bri", Targets: []string{"c-gob"}, Amount: 5}
	f.add(eventLastingTriggered, tick)
	older := land("c-gob", 20, 5) // before the caster was written: nobody's
	older.Lasting = &lastingEvent{Key: "web", Change: "damage", Targets: []string{"c-gob"}, Amount: 5}
	f.add(eventLastingTriggered, older)
	got := tallyOf(f)
	if got["Brisa"].damage != 5 || got["Toren"].damage != 0 {
		t.Errorf("Brisa = %d, Toren = %d; want 5 and 0", got["Brisa"].damage, got["Toren"].damage)
	}
}

// The Infernal Legacy's Hellish Rebuke through the real flow: the fire that downs the goblin
// is in the Reator's "Dano causado" at the end of the combat.
func TestHighlights_HellishRebukeIsInTheCastersDamageDealt(t *testing.T) {
	t.Parallel()
	a, e := reactorFight(t, tieflingFighter)
	hit := a.mustAttack(t, a.master, e, "Goblin", sword, "Reator", d20(15))
	a.mustDamage(t, a.master, e, hit.GetPendingDamage().GetId(), typedDamage(3))
	if _, err := a.settle(t, a.master, e, hit.GetPendingDamage().GetId(), true); err != nil {
		t.Fatalf("ApplyPendingDamage() error = %v", err)
	}
	w := a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_HELLISH_REBUKE)
	if w == nil {
		t.Fatal("the tiefling has no Hellish Rebuke window after being hurt")
	}
	a.mustAnswer(t, a.ana, e, w.GetId(), func(r *playv1.AnswerReactionRequest) {
		r.Answer, r.UseRacial = playv1.ReactionChoice_REACTION_CHOICE_USE, true
	})
	a.h.roller.queue(8, 9, 7) // the fire: 3d10
	save := a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_HELLISH_REBUKE)
	if save == nil {
		t.Fatal("the master has no second step")
	}
	goblinHP, _, _ := a.hp(t, "Goblin")
	done := a.mustAnswer(t, a.master, e, save.GetId(), func(r *playv1.AnswerReactionRequest) {
		r.Answer, r.Roll = playv1.ReactionChoice_REACTION_CHOICE_USE, &playv1.AnswerReactionRequest_Typed{Typed: 3}
	})
	a.endEncounter(t, done.GetEncounter())
	res := a.mustHighlights(t, a.master, done.GetEncounter())
	for _, c := range res.GetCharacters() {
		if c.GetName() == "Reator" {
			if c.GetDamageDealt() != goblinHP || c.GetFinalBlows() != 1 {
				t.Errorf("Reator = %v, want %d damage dealt and a final blow", c, goblinHP)
			}
			return
		}
	}
	t.Fatalf("characters = %v, want Reator's row", res.GetCharacters())
}
