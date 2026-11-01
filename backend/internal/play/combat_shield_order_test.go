package play

import (
	"strings"
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// The Shield prompt comes after the reactions that lower the roll (#643). The attack roll's windows
// (Cutting Words, Protection, Counterspell) open before the d20 and hold the action; the Shield window
// is opened by the replay of the held action, from the hit that is left. These tests freeze that order.
// They need the database (MEURPG_TEST_DATABASE_URL).

// cutAndShield is the Goblin (first) against Pensantus, a level 5 wizard with Shield and Counterspell,
// with a lore bard, Reator, at her side.
func cutAndShield(t *testing.T) (*armed, *playv1.Encounter) {
	t.Helper()
	a := newArmedWith(t, func(a *armed) {
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 5, abilities(16, 13, 14, 10, 8), []string{battleaxe}, nil)
		a.pens = a.ana.caster(t, a.campaignID, "Pensantus", "class:wizard", "race:gnome", 5,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 14, Constitution: 12, Intelligence: 16, Wisdom: 10, Charisma: 8}, nil, []string{fireBolt},
			[]string{magicMissileSpell, shieldSpell, counterspellSpell}, []string{magicMissileSpell, shieldSpell, counterspellSpell})
		a.bri = a.bia.heroWith(t, a.campaignID, "Reator", classLevel("class:bard", 3, "subclass:lore"), abilities(10, 14, 12, 10, 16), []string{rapier}, nil, nil)
	})
	e := a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: a.goblin.GetId()}}, npcRolls: []int{20}, reveal: []string{"Goblin"},
		players: map[string]int32{"Pensantus": 10, "Toren": 9, "Reator": 8},
		at:      map[string][2]int32{"Goblin": {4, 4}, "Pensantus": {5, 5}, "Toren": {6, 5}, "Reator": {5, 6}},
	})
	return a, e
}

// cut answers the bard's Cutting Words with the die typed.
func (a *armed) cut(t *testing.T, e *playv1.Encounter, die int32) *playv1.AnswerReactionResponse {
	t.Helper()
	w := a.windowOf(t, a.bia, playv1.ReactionKind_REACTION_KIND_CUTTING_WORDS)
	if w == nil {
		t.Fatal("the bard has no Cutting Words window")
	}
	return a.mustAnswer(t, a.bia, e, w.GetId(), func(r *playv1.AnswerReactionRequest) {
		r.Answer, r.Roll = playv1.ReactionChoice_REACTION_CHOICE_USE, &playv1.AnswerReactionRequest_Typed{Typed: die}
	})
}

// wantNoShield fails when anyone is asked for a Shield now.
func (a *armed) wantNoShield(t *testing.T, when string) {
	t.Helper()
	for who, u := range map[string]*user{"the master": a.master, "Pensantus's player": a.ana} {
		if w := a.windowOf(t, u, playv1.ReactionKind_REACTION_KIND_SHIELD); w != nil {
			t.Errorf("%s has a Shield window %s: %v", when, who, w)
		}
		if p := a.get(t, u).GetReactionPrompts(); len(p) != 0 {
			t.Errorf("%s has Shield prompts %s: %v", when, who, p)
		}
	}
}

// TestShieldOpensAfterCuttingWords: the Goblin's attack waits for the bard's Cutting Words with no Shield
// window yet; the cut leaves a hit, so the Shield window opens after it, for the total the cut left, and
// the target's player reads neither that total nor the die (RN-20).
func TestShieldOpensAfterCuttingWords(t *testing.T) {
	t.Parallel()
	a, e := cutAndShield(t)
	held := a.mustAttack(t, a.master, e, "Goblin", sword, "Pensantus", d20(12)) // 12 + 4 = 16 against 12
	if !held.GetRoll().GetHeldForReaction() || held.GetPendingDamage() != nil {
		t.Fatalf("the attack = %v, want it held for the bard", held)
	}
	a.wantNoShield(t, "while Cutting Words waits")

	res := a.cut(t, e, 3) // 16 - 3 = 13: still a hit on 12
	if cw := res.GetResult().GetCuttingWords(); !cw.GetEffective() || cw.GetOutcome() != playv1.AttackOutcome_ATTACK_OUTCOME_HIT {
		t.Fatalf("the cut = %v, want a hit that is left", res.GetResult())
	}
	if w := a.windowOf(t, a.bia, playv1.ReactionKind_REACTION_KIND_CUTTING_WORDS); w != nil {
		t.Errorf("Cutting Words stays open after its answer: %v", w)
	}

	// Positive control: the master reads the window, with the total after the cut.
	mw := a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_SHIELD)
	if mw == nil {
		t.Fatal("the cut left a hit and no Shield window opened")
	}
	if tr := mw.GetTrigger(); tr.GetAttackTotal() != 13 || tr.GetTargetArmorClass() != 12 || tr.GetArmorClassWithShield() != 17 {
		t.Errorf("the master's Shield trigger = %v, want the cut total 13 against 12 (17 with Shield)", tr)
	}
	if js := asJSON(t, mw); !strings.Contains(js, "attackTotal") || !strings.Contains(js, "targetArmorClass") {
		t.Errorf("the master's window %s lacks the keys the player's canaries look for", js)
	}
	// The target's player is asked, and reads no total, no armor class and no die.
	pw := a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_SHIELD)
	if pw == nil || !pw.GetForYou() || pw.GetTrigger() != nil || pw.GetShield() == nil {
		t.Fatalf("Pensantus's Shield window = %v, want her prompt with no trigger", pw)
	}
	for what, js := range map[string]string{"the combat": asJSON(t, a.get(t, a.ana)), "the answer's window": asJSON(t, pw)} {
		for _, leak := range []string{"attackTotal", "targetArmorClass", "armorClassWithShield", "cuttingWords", "dieSides", "reduction"} {
			if strings.Contains(js, leak) {
				t.Errorf("Pensantus's player reads %s with %q in it: %s", what, leak, js)
			}
		}
	}
	if p := a.get(t, a.ana).GetReactionPrompts(); len(p) != 1 || p[0].GetSpellKey() != shieldSpell {
		t.Errorf("Pensantus's prompts = %v, want the Shield one", p)
	}
	// Toren's player is not the target: nothing of it.
	if got := a.get(t, a.caio); len(got.GetReactionPrompts()) != 0 || len(got.GetReactionWindows()) != 0 {
		t.Errorf("Toren's player reads the prompts %v / windows %v of Pensantus's Shield", got.GetReactionPrompts(), got.GetReactionWindows())
	}
}

// TestACutThatTurnsTheHitIntoAMissOpensNoShield: the cut takes the hit away, so no Shield window opens, no
// slot is offered and none is spent, and Pensantus keeps her reaction.
func TestACutThatTurnsTheHitIntoAMissOpensNoShield(t *testing.T) {
	t.Parallel()
	a, e := cutAndShield(t)
	a.mustAttack(t, a.master, e, "Goblin", sword, "Pensantus", d20(9)) // 9 + 4 = 13 against 12
	a.wantNoShield(t, "while Cutting Words waits")
	res := a.cut(t, e, 5) // 13 - 5 = 8: a miss
	if cw := res.GetResult().GetCuttingWords(); cw.GetOutcome() != playv1.AttackOutcome_ATTACK_OUTCOME_MISS {
		t.Fatalf("the cut = %v, want a miss", res.GetResult())
	}
	a.wantNoShield(t, "after the cut made a miss")
	if got := res.GetEncounter().GetReactionPrompts(); len(got) != 0 {
		t.Errorf("the answer carries the prompts %v", got)
	}
	if pens := byLabel(t, a.get(t, a.master), "Pensantus"); pens.GetReactionUsed() {
		t.Error("Pensantus's reaction is spent by a Shield nobody asked")
	}
	if used := usedSlots(a.vitals(t, a.pens), 1); used != 0 {
		t.Errorf("Pensantus spent %d 1st-level slots on a miss", used)
	}
}

// TestCounterspellComesBeforeMagicMissileAndItsShield: the Mago Sombrio's Magic Missile at Pensantus waits
// for her Counterspell with no Shield window; once it goes through, her Shield is asked; once it is
// countered, it is not.
func TestCounterspellComesBeforeMagicMissileAndItsShield(t *testing.T) {
	t.Parallel()
	missile := func(t *testing.T) (*armed, *playv1.Encounter) {
		t.Helper()
		a, e := counterFight(t)
		if _, err := a.cast(t, a.master, e, "Mago Sombrio", magicMissileSpell, slotOfLevel(1), []*playv1.SpellTarget{darts(a, t, "Pensantus", 3)}, noCastRoll); err != nil {
			t.Fatalf("CastSpell(Magic Missile) error = %v", err)
		}
		a.wantNoShield(t, "while Counterspell waits")
		if a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_COUNTERSPELL) == nil {
			t.Fatal("Pensantus has no Counterspell window")
		}
		return a, e
	}
	t.Run("let through", func(t *testing.T) {
		t.Parallel()
		a, e := missile(t)
		w := a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_COUNTERSPELL)
		a.mustAnswer(t, a.ana, e, w.GetId(), passAnswer)
		pw := a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_SHIELD)
		if pw == nil || !pw.GetForYou() || pw.GetTrigger() != nil {
			t.Fatalf("Pensantus's Shield window = %v, want it asked once the spell went through", pw)
		}
		if a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_SHIELD) == nil {
			t.Error("the master has no Shield window to read")
		}
		if a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_COUNTERSPELL) != nil {
			t.Error("the Counterspell window is still open")
		}
	})
	t.Run("countered", func(t *testing.T) {
		t.Parallel()
		a, e := missile(t)
		w := a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_COUNTERSPELL)
		a.mustAnswer(t, a.ana, e, w.GetId(), useAnswer(3))
		a.wantNoShield(t, "after the spell was countered")
		if !byLabel(t, a.get(t, a.master), "Pensantus").GetReactionUsed() {
			t.Error("Pensantus's Counterspell did not spend her reaction")
		}
	})
}
