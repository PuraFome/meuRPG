package play

import (
	"context"
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// The branches of the effects' core that the calls reach only in odd states: they are called
// here with the state they read. See lasting_core_frozen_test.go.

// An effect with nothing to say about it in the catalog (a condition the master put on) leaves
// nothing behind when it ends; the other ways an effect ends leave their own line.
func TestLastingCoreAConditionTheMasterPutOnEndsWithoutFollowUps(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.closeFight(t)
	var l coreLog
	a.mustAddEffect(t, e, "condition:poisoned", []string{"Goblin"}, nil)
	a.mustAddEffect(t, e, "condition:frightened", []string{"Toren"}, a.rounds(t, 1, "Toren"))
	if err := a.coreEndEffect(t, e, "Goblin", "condition:poisoned", playv1.EffectEndScope_EFFECT_END_SCOPE_THIS); err != nil {
		t.Fatalf("EndLastingEffect() error = %v", err)
	}
	l.say("after the master ended one:\n%s%s", a.coreEffects(t, e.GetId()), a.coreCombatants(t, e.GetId(), "Goblin", "Toren"))
	a.turnOf(t, "Toren")
	l.say("after its round:\n%s%s", a.coreEffects(t, e.GetId()), a.coreCombatants(t, e.GetId(), "Goblin", "Toren"))
	l.say("events:\n%s", a.coreEvents(t, eventLastingAdded, eventLastingEnded))
	wantLastingGolden(t, "lasting_core", "gaps_condition", a.coreNames(t, l.String()))
}

// Mage Armor is not made for a target that wears armor, and Heroism asks the caster's
// spellcasting modifier of a caster that has none (an NPC with a basic sheet).
func TestLastingCoreWhoTakesHoldOfAnEffectThatNeedsTheSheet(t *testing.T) {
	t.Parallel()
	a := newCastingParty(t)
	var l coreLog
	c := a.bia.character(t, a.bri)
	sheet := c.GetSheet()
	sheet.GetFull().ArmorKey = leatherArmorKey
	if _, err := a.master.characters.UpdateCharacter(t.Context(), connect.NewRequest(&charactersv1.UpdateCharacterRequest{
		CampaignId: a.campaignID, CharacterId: c.GetId(), Revision: c.GetRevision(), Name: c.GetName(), Sheet: sheet,
	})); err != nil {
		t.Fatalf("UpdateCharacter() error = %v", err)
	}
	e := a.closeFight(t)
	res := a.mustAddEffect(t, e, mageArmorKey, []string{"Brisa", "Pensantus", "Toren"}, nil)
	l.say("mage armor on Brisa (armored), Pensantus and Toren: %d effects", len(res.GetEffects()))
	// Finding fix/lasting-effects-outside-combat (d): Mage Armor on the armored one alone is refused.
	_, err := a.addEffect(t, e, mageArmorKey, []string{"Brisa"}, nil)
	wantRefusal(t, "Mage Armor on the armored one alone", err, connect.CodeFailedPrecondition, "the effect takes no hold of these targets")
	a.mustAddEffect(t, e, heroismKey, []string{"Toren"}, withCaster(t, a, "Goblin"))
	l.say("%s%s", a.coreEffects(t, e.GetId()), a.coreCombatants(t, e.GetId(), "Brisa", "Pensantus", "Toren"))
	l.say("events:\n%s", a.coreEvents(t, eventLastingAdded))
	wantLastingGolden(t, "lasting_core", "gaps_sheet", a.coreNames(t, l.String()))
}

// An effect the target is immune to takes no hold: the spell reads "no effect" for the master.
func TestLastingCoreAnImmunityOfAnEffectStopsTheSpell(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.closeFight(t)
	var l coreLog
	a.mustAddEffect(t, e, heroismKey, []string{"Goblin"}, nil)
	a.execSQL(t, `UPDATE combatant_states SET modifiers = '[{"kind":"condition_immunity","conditions":["condition:paralyzed"]}]' WHERE source_key = 'spell:heroism'`)
	e = a.passTo(t, e, "Pensantus")
	a.h.roller.queue(1)
	res, err := a.cast(t, a.ana, e, "Pensantus", holdPerson, slotOfLevel(2), a.at(t, "Goblin"), noCastRoll)
	if err != nil {
		t.Fatalf("CastSpell() error = %v", err)
	}
	for _, h := range res.GetCast().GetTargets() {
		l.say("hold person on the Goblin -> lasting %v why %q save %v", h.GetLastingEffect(), h.GetNoEffectReasonMaster(), h.GetSave().GetOutcome())
	}
	l.say("%s%s", a.coreEffects(t, e.GetId()), a.coreCombatants(t, e.GetId(), "Goblin"))
	wantLastingGolden(t, "lasting_core", "gaps_immunity", a.coreNames(t, l.String()))
}

// The functions the calls reach only in states the database does not hold today are called
// here with the state they read.
func TestLastingCoreTheCoreInOddStates(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.closeFight(t)
	var l coreLog
	a.mustAddEffect(t, e, heroismKey, []string{"Toren", "Goblin"}, withCaster(t, a, "Pensantus"))
	a.mustAddEffect(t, e, "effect:web-fire", []string{"Goblin"}, nil)
	a.mustAddEffect(t, e, webSpell, []string{"Goblin"}, nil)
	a.execSQL(t, `UPDATE combatants SET defeated = true WHERE id = $1`, a.id(t, "Goblin"))
	goblin, toren := a.id(t, "Goblin"), a.id(t, "Toren")
	a.coreInTx(t, e.GetId(), func(ctx context.Context, c *combatTx) error {
		cs, err := c.q.ListCombatants(ctx, c.enc.ID)
		if err != nil {
			return err
		}
		// No target: nothing is made.
		rows, err := a.h.svc.addEffects(ctx, c, cs, effectSpec{key: "spell:bless"})
		l.say("addEffects with no target -> %d rows, %v", len(rows), err)
		// No rows: no line.
		id, err := a.h.svc.addedEventID(ctx, c, cs, nil, "spell:bless")
		l.say("addedEventID with no rows -> %q, %v", id, err)
		// An id that is not in the combat is skipped, the others are worked out.
		l.say("refreshCombatants with an unknown id -> %v", a.h.svc.refreshCombatants(ctx, c, "00000000-0000-4000-8000-000000000009", toren))
		// An id with no character, and an NPC's: there is no base to write.
		l.say("syncArmorBase with no id -> %v", a.h.svc.syncArmorBase(ctx, c, "", a.goblin.GetId()))
		// The start of a turn of a defeated creature and of one that is not named.
		l.say("effectsAfterTurnStart on a defeated goblin and Toren -> %v", a.h.svc.effectsAfterTurnStart(ctx, c, []string{goblin, toren}, 2))
		// Damage that cannot be rolled is not rolled.
		for _, dice := range []string{"not dice", "0d4"} {
			l.say("effectDamage with %q -> %v", dice, a.h.svc.effectDamage(ctx, c, playdb.Combatant{ID: toren}, playdb.CombatantState{TriggerDice: &dice}, 2))
		}
		return nil
	})
	l.say("%s%s", a.coreEffects(t, e.GetId()), a.coreCombatants(t, e.GetId(), "Goblin", "Toren"))
	l.say("events:\n%s", a.coreEvents(t, eventLastingAdded, eventLastingEnded, eventLastingTriggered))
	wantLastingGolden(t, "lasting_core", "gaps_odd_states", a.coreNames(t, l.String()))
}
