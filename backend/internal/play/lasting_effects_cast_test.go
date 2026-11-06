package play

import (
	"strings"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// The effect of a table spell when it is cast (RN-22, #579): who gets it, the temporary hit points and the armor
// class it gives, and how it ends. The spell is "Marca de Nanquim" of lasting_effects_tablespell_test.go.

const (
	modAC   = rulesv1.TableEffectModifierKind_TABLE_EFFECT_MODIFIER_KIND_AC_BONUS
	modTemp = rulesv1.TableEffectModifierKind_TABLE_EFFECT_MODIFIER_KIND_TEMP_HP
)

func selfShape(s *rulesv1.TableSpell) {
	s.Range = &rulesv1.TableSpellRange{Kind: rulesv1.SpellRangeKind_SPELL_RANGE_KIND_SELF}
	s.Target = tableTarget(rulesv1.TableSpellTargetKind_TABLE_SPELL_TARGET_KIND_SELF)
}

// newSelfMark is Pensantus with a self spell that gives +2 armor class and 5 temporary hit points for a minute.
func newSelfMark(t *testing.T) *armed {
	t.Helper()
	return newMarkCastersWith(t, selfShape, &tableMod{Kind: modAC, Value: 2}, &tableMod{Kind: modTemp, Value: 5})
}

func (a *armed) selfMarked(t *testing.T) *playv1.Encounter {
	t.Helper()
	e := a.castersFight(t, 1)
	a.mustCast(t, a.ana, e, "Pensantus", tableMark, slotOfLevel(1), nil, noCastRoll)
	return a.get(t, a.master)
}

// wantMarked checks the caster's armor class bonus, temporary hit points and card.
func (a *armed) wantMarked(t *testing.T, label string, marked bool) {
	t.Helper()
	c := byLabel(t, a.get(t, a.master), label)
	bonus, temp, card := c.GetArmorClassBonus(), c.GetHitPointsTemporary(), cardOf(t, c, tableMark) != nil
	if marked && (bonus != 2 || temp != 5 || !card) {
		t.Errorf("%s: AC bonus %d, temp %d, card %v; want 2, 5 and the card", label, bonus, temp, card)
	}
	if !marked && (bonus != 0 || temp != 0 || card) {
		t.Errorf("%s: AC bonus %d, temp %d, card %v; want none of them", label, bonus, temp, card)
	}
}

func TestATableSpellOnSelfPutsItOnTheCasterInCombat(t *testing.T) {
	t.Parallel()
	a := newSelfMark(t)
	a.selfMarked(t)
	a.wantMarked(t, "Pensantus", true)
	card := cardOf(t, byLabel(t, a.get(t, a.master), "Pensantus"), tableMark)
	if !card.GetConcentration() || card.GetRoundsLeft() != 10 {
		t.Errorf("the card = concentration %v, %d rounds left; want a concentration of 10 rounds", card.GetConcentration(), card.GetRoundsLeft())
	}
}

func TestATableSpellOnSelfEndsWithTheConcentration(t *testing.T) {
	t.Parallel()
	a := newSelfMark(t)
	e := a.selfMarked(t)
	if _, err := a.ana.combat.EndConcentration(t.Context(), connect.NewRequest(&playv1.EndConcentrationRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, "Pensantus"), IdempotencyKey: newKey(),
	})); err != nil {
		t.Fatalf("EndConcentration() error = %v", err)
	}
	a.wantMarked(t, "Pensantus", false)
}

// The 1 minute of the spell is 10 rounds: it ends on the caster's turn of the 11th round.
func TestATableSpellOnSelfEndsByTheClockAfterTenRounds(t *testing.T) {
	t.Parallel()
	a := newSelfMark(t)
	e := a.selfMarked(t)
	for range 80 {
		cur := a.get(t, a.master)
		if cur.GetRound() >= 11 {
			break
		}
		if _, err := a.endTurn(t, a.master, e, true); err != nil {
			t.Fatalf("EndTurn() error = %v", err)
		}
	}
	a.wantMarked(t, "Pensantus", false)
	if got := a.get(t, a.master).GetRound(); got < 11 {
		t.Fatalf("the fight is at round %d, want at least 11", got)
	}
}

func TestATableSpellOnSelfIsTakenBackByTheUndo(t *testing.T) {
	t.Parallel()
	a := newSelfMark(t)
	e := a.selfMarked(t)
	if err := a.undo(t, a.master, e, a.log(t, a.master, e).GetUndoableEventId()); err != nil {
		t.Fatalf("UndoLastAction() error = %v", err)
	}
	a.wantMarked(t, "Pensantus", false)
}

// Out of a combat the self spell is a row of the caster that gives the hit points and takes them back with the clock.
func TestATableSpellOnSelfOutsideACombat(t *testing.T) {
	t.Parallel()
	a := newSelfMark(t)
	a.mustCastOut(t, a.ana, a.pens, tableMark, slotOfLevel(1), false, nil)
	if n := a.effectRows(t, a.pens.GetId(), tableMark); n != 1 {
		t.Fatalf("Pensantus has %d rows of the spell, want 1", n)
	}
	if v := a.vitals(t, a.pens); v.GetHitPointsTemporary() != 5 {
		t.Fatalf("temporary hit points = %d, want 5", v.GetHitPointsTemporary())
	}
	a.advanceGameTime(t, 61)
	if n := a.effectRows(t, a.pens.GetId(), tableMark); n != 0 {
		t.Errorf("Pensantus has %d rows after a minute, want none", n)
	}
	if v := a.vitals(t, a.pens); v.GetHitPointsTemporary() != 0 {
		t.Errorf("temporary hit points = %d after the effect ended, want 0", v.GetHitPointsTemporary())
	}
}

// The temporary hit points do not stack: the caster keeps the higher.
func TestATableSpellTempHPKeepsTheHigherPool(t *testing.T) {
	t.Parallel()
	a := newSelfMark(t)
	e := a.castersFight(t, 1)
	a.correct(t, a.pens, func(r *playv1.AdjustCharacterVitalsRequest) { v := int32(8); r.HitPointsTemporary = &v })
	a.mustCast(t, a.ana, e, "Pensantus", tableMark, slotOfLevel(1), nil, noCastRoll)
	if _, temp, _ := a.hp(t, "Pensantus"); temp != 8 {
		t.Errorf("temporary hit points = %d, want 8 (the 5 of the spell do not add)", temp)
	}
}

// failed_save reaches only the targets that fail the save; hit only the target the attack hits.
func TestATableSpellAppliesOnlyToFailedSaves(t *testing.T) {
	t.Parallel()
	a := newMarkCastersWith(t, func(s *rulesv1.TableSpell) {
		s.Target = &rulesv1.TableSpellTarget{Kind: rulesv1.TableSpellTargetKind_TABLE_SPELL_TARGET_KIND_CREATURES, Count: 2}
		s.Save = &rulesv1.SpellSave{Ability: rulesv1.Ability_ABILITY_WISDOM, OnSuccess: rulesv1.SpellSaveSuccess_SPELL_SAVE_SUCCESS_NONE}
		s.Effect.Applies = rulesv1.TableEffectApplies_TABLE_EFFECT_APPLIES_FAILED_SAVE
	}, &tableMod{Kind: modAC, Value: 2})
	e := a.castersFight(t, 1)
	a.h.roller.queue(1, 20)
	a.mustCast(t, a.ana, e, "Pensantus", tableMark, slotOfLevel(1), a.at(t, "Goblin", "Capitão Goblin"), noCastRoll)
	e = a.get(t, a.master)
	failed, saved := 0, 0
	for _, l := range []string{"Goblin", "Capitão Goblin"} {
		if cardOf(t, byLabel(t, e, l), tableMark) != nil {
			failed++
		} else {
			saved++
		}
	}
	if failed != 1 || saved != 1 {
		t.Errorf("%d targets got the effect and %d did not; want one of each", failed, saved)
	}
}

func TestATableSpellAppliesOnlyToAHit(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		face int32
		want bool
	}{{"hit", 20, true}, {"miss", 1, false}} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			a := newMarkCastersWith(t, func(s *rulesv1.TableSpell) {
				s.Attack = "ranged"
				s.Damage = []*rulesv1.TableSpellDamage{{DamageTypeKey: "damage-type:necrotic", Dice: "1d4"}}
				s.Effect.Applies = rulesv1.TableEffectApplies_TABLE_EFFECT_APPLIES_HIT
			}, &tableMod{Kind: modAC, Value: 2})
			e := a.castersFight(t, 1)
			a.mustCast(t, a.ana, e, "Pensantus", tableMark, slotOfLevel(1), a.at(t, "Goblin"), func(r *playv1.CastSpellRequest) { r.D20Faces = []int32{c.face, c.face} })
			if got := cardOf(t, byLabel(t, a.get(t, a.master), "Goblin"), tableMark) != nil; got != c.want {
				t.Errorf("the goblin has the effect = %v, want %v", got, c.want)
			}
		})
	}
}

// RN-10: a player who does not own the creature reads the label of an effect, never its numbers or the spell's name;
// the master and the creature's owner read all of it.
func TestATableSpellEffectDoesNotLeakItsNumbers(t *testing.T) {
	t.Parallel()
	a := newMarkCastersWith(t, func(s *rulesv1.TableSpell) {
		s.Effect.Visibility = rulesv1.TableEffectAudience_TABLE_EFFECT_AUDIENCE_OWNER
		s.Effect.PlayerLabelPt = "Marcado"
	}, &tableMod{Kind: modTemp, Value: 77}, &tableMod{Kind: rulesv1.TableEffectModifierKind_TABLE_EFFECT_MODIFIER_KIND_ROLL_BONUS, Value: 13,
		AppliesTo: rollKinds(rulesv1.TableEffectRollKind_TABLE_EFFECT_ROLL_KIND_ATTACK)})
	e := a.castersFight(t, 1)
	a.mustCast(t, a.ana, e, "Pensantus", tableMark, slotOfLevel(1), a.at(t, "Toren"), noCastRoll)

	// The creature's card, as each reader gets it, with the combatant's own numbers.
	cardFor := func(u *user) (*playv1.LastingEffect, *playv1.Combatant) {
		c := byLabel(t, a.get(t, u), "Toren")
		return cardOf(t, c, tableMark), c
	}
	for name, u := range map[string]*user{"master": a.master, "owner": a.caio} {
		card, c := cardFor(u)
		if card == nil || len(card.GetModifiers()) != 2 || card.GetModifiers()[0].GetValue()+card.GetModifiers()[1].GetValue() != 90 || !strings.Contains(strings.Join(card.GetTagsPt(), " "), "77") {
			t.Errorf("%s reads %v (%v), want the numbers", name, card, c)
		}
	}
	if v := a.vitals(t, a.toren); v.GetHitPointsTemporary() != 77 {
		t.Errorf("Toren has %d temporary hit points, want 77", v.GetHitPointsTemporary())
	}
	card, c := cardFor(a.bia)
	if c.GetHitPointsTemporary() == 77 || strings.Contains(c.String(), "Marca de Nanquim") {
		t.Errorf("another player reads %v", c)
	}
	if card != nil {
		for _, m := range card.GetModifiers() {
			t.Errorf("another player reads the modifier %v", m)
		}
		read := strings.Join(card.GetTagsPt(), " ") + card.GetSourceNamePt() + strings.Join(card.GetChangesPt(), " ")
		if strings.Contains(read, "77") || strings.Contains(read, "13") || strings.Contains(read, "Marca de Nanquim") {
			t.Errorf("another player reads %v", card)
		}
	}
}

func hideEffect(t *testing.T, a *armed, e *playv1.Encounter, label string) {
	t.Helper()
	id := a.rpcEffectID(t, tableMark, label)
	if _, err := a.rpcVisibility(t, a.master, e, id, func(r *playv1.SetLastingEffectVisibilityRequest) { r.PlayerVisible = false }); err != nil {
		t.Fatalf("SetLastingEffectVisibility() error = %v", err)
	}
}

// RN-10: what an effect the master hides adds to an attack is "Outra fonte" to the player and the spell to the master.
func TestATableSpellHiddenBonusIsOutraFonteToThePlayer(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		who   string
		hides bool // whether the roller is a player
	}{{"player", true}, {"master", false}} {
		t.Run(c.who, func(t *testing.T) {
			t.Parallel()
			a := newMarkCasters(t, &tableMod{
				Kind: rulesv1.TableEffectModifierKind_TABLE_EFFECT_MODIFIER_KIND_ROLL_BONUS, Value: 2,
				AppliesTo: rollKinds(rulesv1.TableEffectRollKind_TABLE_EFFECT_ROLL_KIND_ATTACK),
			})
			e := a.markOn(t, "Toren")
			hideEffect(t, a, e, "Toren")
			a.passTo(t, e, "Toren")
			u := a.master
			if c.hides {
				u = a.caio
			}
			res := a.mustAttack(t, u, e, "Toren", battleaxe, "Goblin", func(r *playv1.RollAttackRequest) { r.D20Faces = []int32{12} })
			texts := sourceTextsOf(res.GetRoll().GetSources())
			if got := strings.Contains(texts, "Outra fonte"); got != c.hides {
				t.Errorf("sources = %q, Outra fonte = %v, want %v", texts, got, c.hides)
			}
			if got := strings.Contains(texts, "Marca de Nanquim"); got == c.hides {
				t.Errorf("sources = %q, the spell's name listed = %v, want %v", texts, got, !c.hides)
			}
		})
	}
}

// A negative roll_bonus is a penalty on the roll.
func TestATableSpellNegativeRollBonusLowersTheRoll(t *testing.T) {
	t.Parallel()
	a := newMarkCasters(t, &tableMod{
		Kind: rulesv1.TableEffectModifierKind_TABLE_EFFECT_MODIFIER_KIND_ROLL_BONUS, Value: -3,
		AppliesTo: rollKinds(rulesv1.TableEffectRollKind_TABLE_EFFECT_ROLL_KIND_ATTACK, rulesv1.TableEffectRollKind_TABLE_EFFECT_ROLL_KIND_SAVE),
	})
	e := a.markOn(t, "Toren")
	a.passTo(t, e, "Toren")
	res := a.mustAttack(t, a.caio, e, "Toren", battleaxe, "Goblin", func(r *playv1.RollAttackRequest) { r.D20Faces = []int32{12} })
	if got := res.GetRoll().GetD20().GetModifier(); got != 3 {
		t.Errorf("the attack's modifier = %d, want 3 (+6 and -3)", got)
	}
	if cm := a.checkModeFor(t, a.toren.GetId(), "", "dex", false); bonusTotal(cm.Bonuses) != -3 {
		t.Errorf("a save's bonus = %d, want -3", bonusTotal(cm.Bonuses))
	}
}

// nonmagical_bps halves the slashing damage of a weapon and leaves a spell's damage alone.
func TestATableSpellNonmagicalResistanceHalvesAWeaponOnly(t *testing.T) {
	t.Parallel()
	a := newMarkCasters(t, &tableMod{
		Kind: rulesv1.TableEffectModifierKind_TABLE_EFFECT_MODIFIER_KIND_RESISTANCE, DamageTypes: []string{"nonmagical_bps"},
	})
	e := a.markOn(t, "Goblin")
	a.passTo(t, e, "Toren")
	res := a.mustAttack(t, a.caio, e, "Toren", battleaxe, "Goblin", func(r *playv1.RollAttackRequest) { r.D20Faces = []int32{15} })
	a.mustDamage(t, a.caio, e, res.GetPendingDamage().GetId(), typedDamage(6)) // 9 with the Strength bonus, 4 after the resistance
	if got, _, _ := a.hp(t, "Goblin"); got != 3 {
		t.Errorf("the goblin has %d hit points after a weapon, want 3", got)
	}
}

func TestATableSpellNonmagicalResistanceLeavesASpellAlone(t *testing.T) {
	t.Parallel()
	a := newMarkCasters(t, &tableMod{
		Kind: rulesv1.TableEffectModifierKind_TABLE_EFFECT_MODIFIER_KIND_RESISTANCE, DamageTypes: []string{"nonmagical_bps"},
	})
	e := a.markOn(t, "Goblin")
	a.passTo(t, e, "Toren")
	a.passTo(t, e, "Pensantus") // the next round: the action of the cast is spent
	res := a.mustAttack(t, a.ana, e, "Pensantus", fireBolt, "Goblin", func(r *playv1.RollAttackRequest) { r.D20Faces = []int32{15, 15} })
	a.mustDamage(t, a.ana, e, res.GetPendingDamage().GetId(), typedDamage(6))
	if got, _, _ := a.hp(t, "Goblin"); got != 1 {
		t.Errorf("the goblin has %d hit points after a Fire Bolt of 6, want 1: the clause is for weapons", got)
	}
}

// RN-10: the damage step of a resistance the master hides reads "Outra fonte" to the character's player, in the
// pending damage and in the log, and the spell's name to the master.
func TestATableSpellHiddenResistanceStepIsOutraFonteToThePlayer(t *testing.T) {
	t.Parallel()
	a := newMarkCasters(t, &tableMod{
		Kind: rulesv1.TableEffectModifierKind_TABLE_EFFECT_MODIFIER_KIND_RESISTANCE, DamageTypes: []string{"all"},
	})
	e := a.markOn(t, "Toren")
	hideEffect(t, a, e, "Toren")
	a.passTo(t, e, "Goblin")
	hit := a.mustAttack(t, a.master, e, "Goblin", "basic:0", "Toren", d20(19))
	dmg := a.mustDamage(t, a.master, e, hit.GetPendingDamage().GetId(), typedDamage(6)).GetPendingDamage()
	stepsText := func(steps []*playv1.DamageStep) string {
		var b strings.Builder
		for _, s := range steps {
			b.WriteString(s.GetLabelPt() + " " + strings.Join(s.GetSourceKeys(), ",") + "|")
		}
		return b.String()
	}
	if got := stepsText(dmg.GetSteps()); !strings.Contains(got, "Marca de Nanquim") && !strings.Contains(got, tableMark) {
		t.Fatalf("the master reads the steps %q, want the spell", got)
	}
	var log string
	for _, r := range a.log(t, a.caio, e).GetRounds() {
		for _, en := range r.GetEntries() {
			log += stepsText(en.GetDamage().GetSteps())
		}
	}
	if !strings.Contains(log, "Outra fonte") || strings.Contains(log, "Marca de Nanquim") || strings.Contains(log, tableMark) {
		t.Errorf("the player reads the steps %q, want Outra fonte and no spell", log)
	}
}
