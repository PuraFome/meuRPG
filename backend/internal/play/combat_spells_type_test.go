package play

import (
	"strings"
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// The creature type in the spells (SRD 5.1): Cure Wounds, Healing Word, Heal and Mass Cure Wounds have no effect on undead or
// constructs, Spare the Dying stabilizes none, Blight skips both and a plant saves against it with disadvantage and takes the
// most the dice can give. The slot is spent; a player reads only that nothing happened and the master reads why (RN-02, RN-10).
// A creature with no type (a character) is as it always was. These tests need the database (MEURPG_TEST_DATABASE_URL).

const (
	blightSpell     = "spell:blight"
	regenerateSpell = "spell:regenerate"
	massHealSpell   = "spell:mass-heal"
)

// typeTable is a cleric of level 11 (Brisa: Wisdom 16, the 1st to the 6th circle) and a wizard of level 17 (Pensantus: Intelligence
// 20, so the save DC is 19) with a Zumbi (undead), an Armadura (construct) and an Arbusto (plant) next to each, and Toren, a
// character with no type. Brisa has the turn.
func typeTable(t *testing.T) (*armed, *playv1.Encounter) {
	t.Helper()
	return typeTableFor(t, false)
}

// typeTableFor is typeTable; with high, Brisa is a cleric of level 17 who prepares Regenerate and Mass Heal (the 7th and 9th
// circles) instead of the level 11 cleric.
func typeTableFor(t *testing.T, high bool) (*armed, *playv1.Encounter) {
	t.Helper()
	a := newArmedWith(t, func(a *armed) {
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 5,
			&rulesv1.AbilityScores{Strength: 16, Dexterity: 13, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}, []string{battleaxe}, nil)
		a.pens = a.ana.caster(t, a.campaignID, "Pensantus", "class:wizard", "race:gnome", 17,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 14, Constitution: 12, Intelligence: 20, Wisdom: 10, Charisma: 8}, nil, nil, []string{blightSpell}, []string{blightSpell})
		if high {
			a.bri = a.bia.castingHero(t, a.campaignID, "Brisa", classLevel("class:cleric", 17, "subclass:life"),
				&rulesv1.AbilityScores{Strength: 10, Dexterity: 16, Constitution: 14, Intelligence: 10, Wisdom: 20, Charisma: 8}, nil, nil,
				[]string{cureWounds, regenerateSpell, massHealSpell}, "")
		} else {
			a.bri = a.bia.caster(t, a.campaignID, "Brisa", "class:cleric", "race:human", 11,
				&rulesv1.AbilityScores{Strength: 10, Dexterity: 16, Constitution: 14, Intelligence: 10, Wisdom: 16, Charisma: 8}, []string{maceKey}, []string{spareDying}, nil,
				[]string{cureWounds, healingWord, massCureWounds, healSpell})
		}
	})
	e := a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: a.goblin.GetId()}}, npcRolls: []int{3},
		players: map[string]int32{"Brisa": 18, "Pensantus": 16, "Toren": 14}, reveal: []string{"Goblin"}, setup: true,
		at: map[string][2]int32{"Brisa": {3, 3}, "Pensantus": {3, 6}, "Toren": {2, 3}, "Goblin": {19, 9}},
	})
	creatures := map[string][2]string{"Zumbi": {"monster:zombie", ""}, "Armadura": {"monster:animated-armor", ""}, "Arbusto": {"monster:awakened-shrub", ""}}
	for name, c := range creatures {
		a.h.roller.queue(10)
		e = a.mustAddMonsters(t, e, func(r *playv1.AddMonstersRequest) {
			r.CreatureKey, r.Count, r.Name, r.Hidden = c[0], 1, name, new(false)
		}).GetEncounter()
	}
	for name, sq := range map[string][2]int32{"Zumbi": {4, 3}, "Armadura": {3, 4}, "Arbusto": {3, 5}} {
		a.put(t, name, sq[0], sq[1])
	}
	e = a.begin(t, e)
	e = a.passTo(t, e, "Brisa")
	return a, e
}

// targetOf is the result of the cast for the target with the label.
func (a *armed) targetOf(t *testing.T, e *playv1.Encounter, res *playv1.CastSpellResponse, label string) *playv1.SpellTargetResult {
	t.Helper()
	id := byLabel(t, e, label).GetId()
	for _, r := range res.GetCast().GetTargets() {
		if r.GetCombatantId() == id {
			return r
		}
	}
	t.Fatalf("%s is not in the cast", label)
	return nil
}

// A healing on an undead or a construct: the slot is spent, no hit points come back, the damage the heal would open is
// not opened, and the player of the caster reads "nothing happened" with no reason (the same as for a creature of any
// other type that nothing refused); the master reads why, in the app's words.
func TestHealingSpellsHaveNoEffectOnUndeadAndConstructs(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, spell string
		slot        int32
		heal        bool // the spell reads its hit points (Heal): the answer is an effect, not a lasting effect
		targets     [][]string
	}{
		{"Cure Wounds", cureWounds, 1, false, [][]string{{"Zumbi"}, {"Armadura"}}},
		{"Healing Word", healingWord, 1, false, [][]string{{"Zumbi"}, {"Armadura"}}},
		{"Mass Cure Wounds", massCureWounds, 5, false, [][]string{{"Zumbi", "Armadura"}}},
		{"Heal", healSpell, 6, true, [][]string{{"Zumbi"}, {"Armadura"}}},
	} {
		for _, targets := range tc.targets {
			t.Run(tc.name+" on "+strings.Join(targets, " and "), func(t *testing.T) {
				t.Parallel()
				a, e := typeTable(t)
				before := map[string]int32{}
				for _, target := range targets {
					before[target] = byLabel(t, e, target).GetHitPointsCurrent()
				}
				res := a.mustCast(t, a.bia, e, "Brisa", tc.spell, slotOfLevel(tc.slot), a.at(t, targets...), noCastRoll)
				for _, target := range targets {
					got := a.targetOf(t, e, res, target)
					if tc.heal {
						if o := got.GetEffect().GetOutcome(); o != playv1.SpellEffectOutcome_SPELL_EFFECT_OUTCOME_NOT_AFFECTED || got.GetEffect().Healed != nil {
							t.Errorf("%s: the player reads the effect %v, want not affected and no amount", target, got.GetEffect())
						}
					} else if got.GetLastingEffect() != playv1.LastingEffectOutcome_LASTING_EFFECT_OUTCOME_NO_EFFECT {
						t.Errorf("%s: the player reads %v, want no effect", target, got.GetLastingEffect())
					}
					if got.GetNoEffectReasonMaster() != "" || got.GetPendingDamageId() != "" || len(got.GetMorePendingDamageIds()) != 0 {
						t.Errorf("%s: the player reads the reason %q and the pending heals %q %v, want none", target, got.GetNoEffectReasonMaster(), got.GetPendingDamageId(), got.GetMorePendingDamageIds())
					}
					if now := byLabel(t, a.get(t, a.master), target).GetHitPointsCurrent(); now != before[target] {
						t.Errorf("%s has %d PV, want the %d it had", target, now, before[target])
					}
				}
				if used := usedSlots(a.vitals(t, a.bri), tc.slot); used != 1 {
					t.Errorf("slots of level %d used = %d, want 1: the slot is spent", tc.slot, used)
				}
				// The master, who casts it again for Brisa, reads why.
				if err := a.undo(t, a.master, e, a.log(t, a.master, e).GetUndoableEventId()); err != nil {
					t.Fatalf("UndoLastAction() error = %v", err)
				}
				master := a.mustCast(t, a.master, e, "Brisa", tc.spell, slotOfLevel(tc.slot), a.at(t, targets...), noCastRoll)
				for _, target := range targets {
					if why := a.targetOf(t, e, master, target).GetNoEffectReasonMaster(); why != whyNoLifeMagic {
						t.Errorf("%s: the master reads %q, want %q", target, why, whyNoLifeMagic)
					}
				}
			})
		}
	}
}

// A creature with no type, a character, is healed as ever: Cure Wounds opens its heal and Heal heals it.
func TestHealingSpellsStillHealACreatureWithNoType(t *testing.T) {
	t.Parallel()
	a, e := typeTable(t)
	a.hurt(t, a.toren, 5)
	a.put(t, "Toren", 2, 3)
	cure := a.targetOf(t, e, a.mustCast(t, a.bia, e, "Brisa", cureWounds, slotOfLevel(1), a.at(t, "Toren"), noCastRoll), "Toren")
	if cure.GetPendingDamageId() == "" || cure.GetLastingEffect() != playv1.LastingEffectOutcome_LASTING_EFFECT_OUTCOME_UNSPECIFIED {
		t.Errorf("Cure Wounds on a character = %v, want its heal opened", cure)
	}
	if err := a.undo(t, a.master, e, a.log(t, a.master, e).GetUndoableEventId()); err != nil {
		t.Fatalf("UndoLastAction() error = %v", err)
	}
	heal := a.targetOf(t, e, a.mustCast(t, a.bia, e, "Brisa", healSpell, slotOfLevel(6), a.at(t, "Toren"), noCastRoll), "Toren")
	if heal.GetEffect().GetOutcome() != playv1.SpellEffectOutcome_SPELL_EFFECT_OUTCOME_AFFECTED {
		t.Errorf("Heal on a character = %v, want affected", heal.GetEffect())
	}
}

// Spare the Dying stabilizes no undead and no construct: the player reads that it was not affected, as for any creature that
// is not at 0 hit points, and the master reads the type's reason instead of the one of the hit points.
func TestSpareTheDyingStabilisesNoUndeadAndNoConstruct(t *testing.T) {
	t.Parallel()
	for _, target := range []string{"Zumbi", "Armadura"} {
		t.Run(target, func(t *testing.T) {
			t.Parallel()
			a, e := typeTable(t)
			res := a.mustCast(t, a.bia, e, "Brisa", spareDying, nil, a.at(t, target), noCastRoll)
			got := a.targetOf(t, e, res, target)
			if got.GetEffect().GetOutcome() != playv1.SpellEffectOutcome_SPELL_EFFECT_OUTCOME_NOT_AFFECTED || got.GetNoEffectReasonMaster() != "" || got.GetEffect().GetReason() != playv1.SpellEffectReason_SPELL_EFFECT_REASON_UNSPECIFIED {
				t.Errorf("the player reads %v, want not affected with no reason", got)
			}
			if err := a.undo(t, a.master, e, a.log(t, a.master, e).GetUndoableEventId()); err != nil {
				t.Fatalf("UndoLastAction() error = %v", err)
			}
			master := a.targetOf(t, e, a.mustCast(t, a.master, e, "Brisa", spareDying, nil, a.at(t, target), noCastRoll), target)
			if master.GetNoEffectReasonMaster() != whyNoLifeMagic || master.GetEffect().GetOutcome() != playv1.SpellEffectOutcome_SPELL_EFFECT_OUTCOME_NOT_AFFECTED {
				t.Errorf("the master reads %v, want not affected: %q", master, whyNoLifeMagic)
			}
		})
	}
}

// Blight skips an undead and a construct: no save, no damage, the slot spent; the player reads that it had no effect, the
// master reads why.
func TestBlightSkipsUndeadAndConstructs(t *testing.T) {
	t.Parallel()
	for _, target := range []string{"Zumbi", "Armadura"} {
		t.Run(target, func(t *testing.T) {
			t.Parallel()
			a, e := typeTable(t)
			e = a.passTo(t, e, "Pensantus")
			res := a.mustCast(t, a.ana, e, "Pensantus", blightSpell, slotOfLevel(4), a.at(t, target), noCastRoll)
			got := a.targetOf(t, e, res, target)
			if got.GetLastingEffect() != playv1.LastingEffectOutcome_LASTING_EFFECT_OUTCOME_NO_EFFECT || got.GetNoEffectReasonMaster() != "" {
				t.Errorf("the player reads %v, want no effect and no reason", got)
			}
			if got.GetSave() != nil || got.GetPendingDamageId() != "" {
				t.Errorf("the cast rolled a save or opened a damage: %v", got)
			}
			if used := usedSlots(a.vitals(t, a.pens), 4); used != 1 {
				t.Errorf("4th-level slots used = %d, want 1", used)
			}
			if err := a.undo(t, a.master, e, a.log(t, a.master, e).GetUndoableEventId()); err != nil {
				t.Fatalf("UndoLastAction() error = %v", err)
			}
			master := a.targetOf(t, e, a.mustCast(t, a.master, e, "Pensantus", blightSpell, slotOfLevel(4), a.at(t, target), noCastRoll), target)
			if master.GetNoEffectReasonMaster() != whyNoLifeMagic || master.GetSave() != nil {
				t.Errorf("the master reads %v, want no save and %q", master, whyNoLifeMagic)
			}
		})
	}
}

// A plant saves against Blight with disadvantage and takes the most the dice can give (8d8 at the 4th level: 64), halved when it
// saves. Only the master reads the roll and why it had disadvantage; the sentence is the app's, and a player is given none.
func TestBlightOnAPlantSavesWithDisadvantageAndTakesTheMaximum(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		faces []int
		saved bool
		want  int32
	}{
		{"failed", []int{19, 2}, false, 64}, // the lower d20 counts: 2 - 1 = 1
		{"passed", []int{20, 20}, true, 32},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a, e := typeTable(t)
			e = a.passTo(t, e, "Pensantus")
			a.h.roller.queue(tc.faces...)
			res := a.mustCast(t, a.master, e, "Pensantus", blightSpell, slotOfLevel(4), a.at(t, "Arbusto"), noCastRoll)
			got := a.targetOf(t, e, res, "Arbusto")
			if got.GetSave().GetMode() != playv1.RollMode_ROLL_MODE_DISADVANTAGE || len(got.GetSave().GetRoll().GetFaces()) != 2 {
				t.Errorf("the master reads the save %v, want two d20 with disadvantage", got.GetSave())
			}
			var source *playv1.AdvantageSource
			for _, s := range got.GetSave().GetSources() {
				if s.GetKind() == playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_TARGET_CREATURE_TYPE {
					source = s
				}
			}
			if source == nil || source.GetEffect() != playv1.RollMode_ROLL_MODE_DISADVANTAGE || source.GetTextPt() != "Criatura do tipo planta: desvantagem no teste de resistência" {
				t.Errorf("the master reads the source %v, want the plant's disadvantage", source)
			}
			if (got.GetSave().GetOutcome() == playv1.SaveOutcome_SAVE_OUTCOME_SAVED) != tc.saved {
				t.Fatalf("the save = %v, want saved %v", got.GetSave().GetOutcome(), tc.saved)
			}
			a.h.roller.queue(1, 1, 1, 1, 1, 1, 1, 1)
			dmg := a.mustDamage(t, a.master, e, got.GetPendingDamageId(), inAppDamage).GetPendingDamage()
			if dmg.GetDiceCount() != 8 || dmg.GetDiceSides() != 8 || dmg.GetHalf() != tc.saved {
				t.Errorf("the pending damage = %dd%d (half %v), want the spell's 8d8 as on any target (half %v)", dmg.GetDiceCount(), dmg.GetDiceSides(), dmg.GetHalf(), tc.saved)
			}
			// The server takes the most the dice can give at landing, halved on a passed save; the master's line reads it, a player's
			// reads the damage as rolled (8, halved: 4).
			applied, asRolled := a.castDamage(t, a.master, e).GetAmount(), a.castDamage(t, a.ana, e).GetAmount()
			if applied != tc.want {
				t.Errorf("the master reads the damage %d, want %d applied", applied, tc.want)
			}
			if wantRolled := map[bool]int32{false: 8, true: 4}[tc.saved]; asRolled != wantRolled {
				t.Errorf("the caster's player reads the damage %d, want %d as rolled", asRolled, wantRolled)
			}
		})
	}
}

// A creature with no type (a character) saves against Blight as ever: no disadvantage, the dice rolled.
func TestBlightOnACreatureWithNoTypeIsAsItWas(t *testing.T) {
	t.Parallel()
	a, e := typeTable(t)
	e = a.passTo(t, e, "Pensantus")
	a.put(t, "Toren", 3, 7)
	a.h.roller.queue(1)
	got := a.targetOf(t, e, a.mustCast(t, a.master, e, "Pensantus", blightSpell, slotOfLevel(4), a.at(t, "Toren"), noCastRoll), "Toren")
	if got.GetSave().GetMode() != playv1.RollMode_ROLL_MODE_NORMAL || got.GetLastingEffect() != playv1.LastingEffectOutcome_LASTING_EFFECT_OUTCOME_UNSPECIFIED {
		t.Errorf("the save of a character = %v, want a normal roll and no refusal", got)
	}
	a.h.roller.queue(1, 1, 1, 1, 1, 1, 1, 1)
	dmg := a.mustDamage(t, a.master, e, got.GetPendingDamageId(), inAppDamage).GetPendingDamage()
	if dmg.GetDiceCount() != 8 || dmg.GetAmount() != 8 {
		t.Errorf("the damage = %d dice, %d, want 8d8 rolled (8)", dmg.GetDiceCount(), dmg.GetAmount())
	}
}

// castDamage is the damage the latest cast of the log opened for its target, as the user reads it.
func (a *armed) castDamage(t *testing.T, u *user, e *playv1.Encounter) *playv1.CombatLogDamage {
	t.Helper()
	targets := spellEntry(t, a.log(t, u, e)).GetSpell().GetTargets()
	if len(targets) != 1 || targets[0].GetDamage() == nil {
		t.Fatalf("the cast's targets = %v, want one with its damage", targets)
	}
	return targets[0].GetDamage()
}

// The caster's player rolls Blight's damage on a plant and reads it as rolled, as on a creature that is not a plant, whether the
// target failed the save (amount 8, total 8) or passed it (amount 4, the half, and the whole roll's total 8): in the answer to the
// roll and in a retry of the cast, never the maximum the plant took. The options of the turn list no pending damage of an NPC
// target once it is rolled (it is applied at once), so the view they would use is the one of the answer (pendingView).
func TestBlightOnAPlantReadsAsRolledToTheCastersPlayer(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name         string
		target       string
		save         []int
		half         bool
		amount, roll int32
	}{
		{"the shrub fails", "Arbusto", []int{1, 1}, false, 8, 8},
		{"the goblin fails", "Goblin", []int{1}, false, 8, 8},
		{"the shrub saves", "Arbusto", []int{20, 20}, true, 4, 8},
		{"the goblin saves", "Goblin", []int{20}, true, 4, 8},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a, e := typeTable(t)
			e = a.passTo(t, e, "Pensantus")
			a.put(t, "Goblin", 4, 6)
			a.h.roller.queue(tc.save...)
			key := newKey()
			res, err := a.castKey(t, a.ana, e, "Pensantus", blightSpell, slotOfLevel(4), a.at(t, tc.target), noCastRoll, key)
			if err != nil {
				t.Fatalf("CastSpell() error = %v", err)
			}
			got := a.targetOf(t, e, res, tc.target)
			a.h.roller.queue(1, 1, 1, 1, 1, 1, 1, 1)
			dmg := a.mustDamage(t, a.ana, e, got.GetPendingDamageId(), inAppDamage).GetPendingDamage()
			check := func(what string, d *playv1.PendingDamage) {
				t.Helper()
				if d.GetHalf() != tc.half || d.GetAmount() != tc.amount || d.GetRoll().GetTotal() != tc.roll {
					t.Errorf("%s reads half %v, amount %d and total %d, want %v, %d and %d as rolled", what, d.GetHalf(), d.GetAmount(), d.GetRoll().GetTotal(), tc.half, tc.amount, tc.roll)
				}
			}
			check("the answer to the roll", dmg)
			// The same cast asked again after the roll (a retry) holds the damage as rolled too.
			again, err := a.castKey(t, a.ana, e, "Pensantus", blightSpell, slotOfLevel(4), a.at(t, tc.target), noCastRoll, key)
			if err != nil {
				t.Fatalf("the retry of the cast: %v", err)
			}
			if pd := again.GetCast().GetPendingDamages(); len(pd) != 1 {
				t.Errorf("the retry holds %d pending damages, want 1", len(pd))
			} else {
				check("the retry of the cast", pd[0])
			}
		})
	}
}

// Spare the Dying on a creature with no type works as it always did.
func TestSpareTheDyingStillStabilizesACreatureWithNoType(t *testing.T) {
	t.Parallel()
	a, e := typeTable(t)
	a.correct(t, a.toren, hpIs(0))
	a.put(t, "Toren", 2, 3)
	got := a.targetOf(t, e, a.mustCast(t, a.bia, e, "Brisa", spareDying, nil, a.at(t, "Toren"), noCastRoll), "Toren")
	if got.GetEffect().GetOutcome() != playv1.SpellEffectOutcome_SPELL_EFFECT_OUTCOME_AFFECTED {
		t.Errorf("Spare the Dying on Toren = %v, want affected", got.GetEffect())
	}
	if c := byLabel(t, a.get(t, a.master), "Toren"); c.GetState() != playv1.CombatantState_COMBATANT_STATE_STABLE {
		t.Errorf("Toren = %v, want stable", c.GetState())
	}
}

// wound is the master's damage to a combatant of the encounter (an NPC, which a.hurt cannot reach).
func (a *armed) wound(t *testing.T, e *playv1.Encounter, label string, damage int32) {
	t.Helper()
	if _, err := a.adjustHP(t, e, label, func(r *playv1.AdjustCombatantHitPointsRequest) {
		r.Change = &playv1.AdjustCombatantHitPointsRequest_Damage{Damage: damage}
	}); err != nil {
		t.Fatalf("AdjustCombatantHitPoints(%s) error = %v", label, err)
	}
}

// Heal ends Blinded and Deafened on a creature it heals, but on an undead or a construct it does nothing at all: the
// conditions stay, the hit points stay, the slot is spent.
func TestHealLeavesABlindedUndeadOrConstructBlind(t *testing.T) {
	t.Parallel()
	for _, target := range []string{"Zumbi", "Armadura"} {
		t.Run(target, func(t *testing.T) {
			t.Parallel()
			a, e := typeTable(t)
			a.setConditions(t, e, target, blindedC)
			before := byLabel(t, a.get(t, a.master), target).GetHitPointsCurrent()
			a.mustCast(t, a.bia, e, "Brisa", healSpell, slotOfLevel(6), a.at(t, target), noCastRoll)
			now := byLabel(t, a.get(t, a.master), target)
			if now.GetHitPointsCurrent() != before {
				t.Errorf("%s has %d PV, want the %d it had", target, now.GetHitPointsCurrent(), before)
			}
			if !hasCondition(a.get(t, a.master), t, target, blindedC) {
				t.Errorf("%s conditions = %v, want still blinded", target, now.GetConditions())
			}
			if used := usedSlots(a.vitals(t, a.bri), 6); used != 1 {
				t.Errorf("6th-level slots used = %d, want 1: the slot is spent", used)
			}
		})
	}
}

// Mass Cure Wounds picks its targets one by one: the Zumbi gets nothing and the plant and the character get their heal.
func TestMassCureWoundsSkipsTheUndeadAndHealsTheOthers(t *testing.T) {
	t.Parallel()
	a, e := typeTable(t)
	a.wound(t, e, "Arbusto", 5)
	a.hurt(t, a.toren, 5)
	a.put(t, "Toren", 2, 3)
	zombieBefore := byLabel(t, a.get(t, a.master), "Zumbi").GetHitPointsCurrent()
	res := a.mustCast(t, a.bia, e, "Brisa", massCureWounds, slotOfLevel(5), a.at(t, "Zumbi", "Arbusto", "Toren"), noCastRoll)
	zombie := a.targetOf(t, e, res, "Zumbi")
	if zombie.GetLastingEffect() != playv1.LastingEffectOutcome_LASTING_EFFECT_OUTCOME_NO_EFFECT || zombie.GetPendingDamageId() != "" {
		t.Errorf("the Zumbi reads %v, want no effect and no heal", zombie)
	}
	if now := byLabel(t, a.get(t, a.master), "Zumbi").GetHitPointsCurrent(); now != zombieBefore {
		t.Errorf("the Zumbi has %d PV, want the %d it had", now, zombieBefore)
	}
	for _, name := range []string{"Arbusto", "Toren"} {
		got := a.targetOf(t, e, res, name)
		if got.GetPendingDamageId() == "" || got.GetLastingEffect() != playv1.LastingEffectOutcome_LASTING_EFFECT_OUTCOME_UNSPECIFIED {
			t.Errorf("Mass Cure Wounds on %s = %v, want its heal opened", name, got)
		}
	}
}

// A plant is a creature that life magic heals: Cure Wounds and Healing Word open its heal, and Heal heals it.
func TestHealingSpellsHealAPlantCreature(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, spell string
		slot        int32
	}{
		{"Cure Wounds", cureWounds, 1},
		{"Healing Word", healingWord, 1},
		{"Heal", healSpell, 6},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a, e := typeTable(t)
			max := byLabel(t, e, "Arbusto").GetHitPointsCurrent()
			a.wound(t, e, "Arbusto", 5)
			a.put(t, "Arbusto", 4, 4) // beside Brisa: Cure Wounds is a touch
			got := a.targetOf(t, e, a.mustCast(t, a.bia, e, "Brisa", tc.spell, slotOfLevel(tc.slot), a.at(t, "Arbusto"), noCastRoll), "Arbusto")
			if got.GetLastingEffect() != playv1.LastingEffectOutcome_LASTING_EFFECT_OUTCOME_UNSPECIFIED {
				t.Errorf("%s on the Arbusto reads %v, want no refusal", tc.name, got)
			}
			if tc.spell == healSpell {
				if got.GetEffect().GetOutcome() != playv1.SpellEffectOutcome_SPELL_EFFECT_OUTCOME_AFFECTED {
					t.Errorf("Heal on the Arbusto = %v, want affected", got.GetEffect())
				}
				if now := byLabel(t, a.get(t, a.master), "Arbusto").GetHitPointsCurrent(); now != max {
					t.Errorf("the Arbusto has %d PV after Heal, want its %d", now, max)
				}
			} else if got.GetPendingDamageId() == "" {
				t.Errorf("%s on the Arbusto = %v, want its heal opened", tc.name, got)
			}
		})
	}
}

// Mass Heal is cast in a fight like Cure Wounds (its "up to 700" is a heal the cast opens): it has no effect on an undead or a
// construct and heals a creature with no type. Regenerate takes a minute to cast, so a fight refuses it before the type is read.
func TestMassHealHasNoEffectOnUndeadAndConstructs(t *testing.T) {
	t.Parallel()
	a, e := typeTableFor(t, true)
	a.hurt(t, a.toren, 5)
	a.put(t, "Toren", 2, 3)
	res := a.mustCast(t, a.bia, e, "Brisa", massHealSpell, slotOfLevel(9), a.at(t, "Zumbi", "Armadura", "Toren"), noCastRoll)
	for _, target := range []string{"Zumbi", "Armadura"} {
		got := a.targetOf(t, e, res, target)
		if got.GetLastingEffect() != playv1.LastingEffectOutcome_LASTING_EFFECT_OUTCOME_NO_EFFECT || got.GetPendingDamageId() != "" {
			t.Errorf("Mass Heal on %s = %v, want no effect and no heal opened", target, got)
		}
	}
	if got := a.targetOf(t, e, res, "Toren"); got.GetPendingDamageId() == "" {
		t.Errorf("Mass Heal on a character = %v, want its heal opened", got)
	}
}
