package play

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// The card of a weapon-hit-damage effect (#577): the caster's player and the master read it whole, with the dice
// worked out for the slot of the cast and whether it was used; every other player reads the name of the mark when
// it is visible and nothing of the dice (RN-10).

const (
	canaryType = "damage-type:necrotic"
	canaryCond = "condition:frightened"
)

// canaryMod is a hit of seven d12 (cap 13, one more for each slot above the first) of necrotic damage with a Wisdom
// save that frightens until the end of the spell: nothing else of the fight has these.
func canaryMod(trigger rulesv1.TableHitTrigger) *tableMod {
	return hitMod(trigger, func(m *tableMod) {
		m.Die, m.Dice, m.PerSlotDice, m.MaxDice, m.DamageType = 12, 7, 1, 13, canaryType
		m.Save, m.OnFailConditions, m.ConditionDuration = "wis", []string{canaryCond}, spellEnd
	})
}

// hitCardOf is the card of the spell's effect on the combatant, as u reads the combat; nil when there is none.
func (a *armed) hitCardOf(t *testing.T, u *user, label string) *playv1.LastingEffect {
	t.Helper()
	for _, ef := range byLabel(t, a.get(t, u), label).GetEffects() {
		if ef.GetSourceKey() == tableMark && len(ef.GetModifiers()) > 0 {
			return ef
		}
	}
	return nil
}

func TestAHitEffectCardTheCastersPlayerAndTheMasterReadItWhole(t *testing.T) {
	t.Parallel()
	a := newHitCasters(t, markShape, canaryMod(trigMarked))
	e := a.hitFight(t)
	a.castMark(t, e, 2, "Goblin")
	for name, u := range map[string]*user{"Ana": a.ana, "the master": a.master} {
		card := a.hitCardOf(t, u, "Pensantus")
		if card == nil || len(card.GetModifiers()) != 1 {
			t.Fatalf("%s reads the card %v, want one hit modifier", name, card)
		}
		m, hit := card.GetModifiers()[0], card.GetModifiers()[0].GetHit()
		if m.GetKind() != playv1.EffectModifierKind_EFFECT_MODIFIER_KIND_WEAPON_HIT_DAMAGE || hit == nil {
			t.Fatalf("%s reads the modifier %v", name, m)
		}
		if hit.GetTrigger() != "marked_target" || hit.GetDie() != 12 || hit.GetDice() != 8 || hit.GetPerSlotDice() != 0 || hit.GetMaxDice() != 13 ||
			hit.GetDamageTypeKey() != canaryType || hit.GetDamageTypePt() == "" || hit.GetSave() != "wis" ||
			len(hit.GetConditionKeys()) != 1 || hit.GetConditionKeys()[0] != canaryCond || hit.GetConditionNamesPt()[0] == "" || hit.GetConditionDuration() != "spell_end" {
			t.Errorf("%s reads the hit %v, want 8d12 (7 and one slot above) of necrotic damage, save wis, frightened to the end of the spell", name, hit)
		}
		if card.GetSpent() {
			t.Errorf("%s reads a pending effect as spent", name)
		}
	}
}

func TestAHitEffectCardSaysWhenItIsSpent(t *testing.T) {
	t.Parallel()
	a := newHitCasters(t, nil, hitMod(trigNext))
	e := a.hitFight(t)
	a.castMark(t, e, 1)
	if card := a.hitCardOf(t, a.ana, "Pensantus"); card == nil || card.GetSpent() {
		t.Fatalf("before the hit the card is %v, want a pending one", card)
	}
	hit := a.strike(t, e, battleaxe, "Goblin", d20(19))
	a.rollDamage(t, hit, 1, 1)
	for name, u := range map[string]*user{"Ana": a.ana, "the master": a.master} {
		if card := a.hitCardOf(t, u, "Pensantus"); card == nil || !card.GetSpent() {
			t.Errorf("%s reads the card %v after the hit, want it spent", name, card)
		}
	}
}

// RN-10: a spent mark is the caster's player's and the master's to read; no other player reads an effect of the combat as
// spent, on any combatant, nor its tag.
func TestRN10_ASpentHitEffectIsNotReadAsSpentByOtherPlayers(t *testing.T) {
	t.Parallel()
	a := newHitCasters(t, nil, hitMod(trigNext))
	e := a.hitFight(t)
	a.castMark(t, e, 1)
	a.rollDamage(t, a.strike(t, e, battleaxe, "Goblin", d20(19)), 1, 1)
	for name, u := range map[string]*user{"Caio": a.caio, "Bia": a.bia} {
		for _, c := range a.get(t, u).GetCombatants() {
			for _, ef := range c.GetEffects() {
				if ef.GetSpent() || slices.Contains(ef.GetTagsPt(), spentTagPT) {
					t.Errorf("%s reads %v of %s as spent", name, ef, c.GetLabel())
				}
			}
		}
	}
	for name, u := range map[string]*user{"Ana": a.ana, "the master": a.master} {
		if card := a.hitCardOf(t, u, "Pensantus"); card == nil || !card.GetSpent() {
			t.Errorf("%s reads the card %v, want it spent: the control is unreachable", name, card)
		}
	}
}

// RN-10 at the API: every other player reads no part of the card (the die, the dice, the trigger, the save, the damage
// type, the condition), in the combat, the log and the turn options. Ana (the caster's player) and the master are the
// positive controls. Caio plays Toren, the creature that carries the mark, and reads the mark's name when it is visible.
func TestRN10_AHitEffectCardIsNotReadByOtherPlayers(t *testing.T) {
	t.Parallel()
	canaries := regexp.MustCompile(`"die":\s*12|"dice":\s*[78]\b|"maxDice"|"hit"|marked_target|each_hit|next_hit|next_melee_hit|necrotic|"save":\s*"wis"|condition:frightened|"diceSides":\s*12|"diceCount":\s*[78]\b`)
	for _, tc := range []struct {
		name       string
		visibility rulesv1.TableEffectAudience
		trigger    rulesv1.TableHitTrigger
		target     string
		markSeen   string
	}{
		{"a public mark on another player", rulesv1.TableEffectAudience_TABLE_EFFECT_AUDIENCE_PUBLIC, trigMarked, "Toren", "Marca de Nanquim"},
		{"a public mark on a monster", rulesv1.TableEffectAudience_TABLE_EFFECT_AUDIENCE_PUBLIC, trigMarked, "Goblin", "Marca de Nanquim"},
		{"an owner's mark on another player", rulesv1.TableEffectAudience_TABLE_EFFECT_AUDIENCE_OWNER, trigMarked, "Toren", "Marca de Nanquim"},
		{"an effect on the caster alone", rulesv1.TableEffectAudience_TABLE_EFFECT_AUDIENCE_PUBLIC, trigEach, "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := newHitCasters(t, func(s *rulesv1.TableSpell) {
				if tc.target != "" {
					markShape(s)
				}
				s.Effect.Visibility = tc.visibility
			}, canaryMod(tc.trigger))
			e := a.hitFight(t)
			if tc.target == "" {
				a.castMark(t, e, 2)
			} else {
				a.castMark(t, e, 2, tc.target)
			}
			a.strike(t, e, battleaxe, "Goblin", d20(19))
			for _, p := range []struct {
				name string
				u    *user
			}{{"Caio", a.caio}, {"Bia", a.bia}} {
				if text := a.readsOf(t, p.u, e, "Brisa") + a.readsOf(t, p.u, e, ""); canaries.MatchString(text) {
					t.Errorf("%s reads %q of the card", p.name, canaries.FindString(text))
				}
				if card := a.hitCardOf(t, p.u, "Pensantus"); card != nil {
					t.Errorf("%s reads the caster's card %v", p.name, card)
				}
			}
			if tc.target == "Toren" {
				if tc.visibility == rulesv1.TableEffectAudience_TABLE_EFFECT_AUDIENCE_PUBLIC {
					var seen string
					for _, l := range byLabel(t, a.get(t, a.caio), "Toren").GetEffectLabels() {
						seen = l.GetTextPt()
					}
					if seen != tc.markSeen {
						t.Errorf("the marked player reads the label %q, want %q", seen, tc.markSeen)
					}
				}
				for _, ef := range byLabel(t, a.get(t, a.caio), "Toren").GetEffects() {
					if len(ef.GetModifiers()) != 0 || len(ef.GetConditionKeys()) != 0 {
						t.Errorf("the marked player reads the mark %v, want the name alone", ef)
					}
				}
			}
			for name, u := range map[string]*user{"Ana": a.ana, "the master": a.master} {
				if !canaries.MatchString(a.readsOf(t, u, e, "Pensantus")) {
					t.Errorf("%s does not read the card: the canary is unreachable", name)
				}
			}
		})
	}
}

// Both blocks in one spell: the target reads what the spell does to it (the condition, the label), the caster's card
// the hit; the other players read neither the dice nor the target's block's source beyond its label.
func TestRN10_ABothBlocksSpellKeepsTheHitCardOffTheTarget(t *testing.T) {
	t.Parallel()
	canaries := regexp.MustCompile(`"die":\s*12|"dice":\s*7\b|"maxDice"|"hit"|marked_target|necrotic`)
	a := newHitCasters(t, markShape, canaryMod(trigMarked), &tableMod{
		Kind: rulesv1.TableEffectModifierKind_TABLE_EFFECT_MODIFIER_KIND_ROLL_BONUS, Value: -2,
		AppliesTo: rollKinds(rulesv1.TableEffectRollKind_TABLE_EFFECT_ROLL_KIND_ATTACK),
	})
	e := a.hitFight(t)
	a.castMark(t, e, 1, "Toren")
	for name, u := range map[string]*user{"Caio": a.caio, "Bia": a.bia} {
		if text := a.readsOf(t, u, e, "Brisa") + a.readsOf(t, u, e, ""); canaries.MatchString(text) {
			t.Errorf("%s reads %q", name, canaries.FindString(text))
		}
	}
	for _, ef := range byLabel(t, a.get(t, a.caio), "Toren").GetEffects() {
		for _, m := range ef.GetModifiers() {
			if m.GetHit() != nil || m.GetKind() == playv1.EffectModifierKind_EFFECT_MODIFIER_KIND_WEAPON_HIT_DAMAGE {
				t.Errorf("the target reads the hit on its own card: %v", m)
			}
		}
	}
	if card := a.hitCardOf(t, a.ana, "Pensantus"); card == nil || card.GetModifiers()[0].GetHit().GetDice() != 7 {
		t.Errorf("the caster's player reads %v, want the hit", card)
	}
}

// A spell that adds damage to weapon hits has its place in a combat: cast out of one it is refused before a slot is
// spent, and leaves no record of the character.
func TestAHitSpellCastOutsideACombatIsRefusedBeforeSpendingAnything(t *testing.T) {
	t.Parallel()
	for name, mods := range map[string][]*tableMod{
		"next hit":    {hitMod(trigNext)},
		"marked":      {hitMod(trigMarked)},
		"both blocks": {hitMod(trigEach), {Kind: rulesv1.TableEffectModifierKind_TABLE_EFFECT_MODIFIER_KIND_ROLL_BONUS, Value: 2, AppliesTo: rollKinds(rulesv1.TableEffectRollKind_TABLE_EFFECT_ROLL_KIND_ATTACK)}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			a := newHitCasters(t, func(s *rulesv1.TableSpell) {
				if name == "marked" {
					markShape(s)
				}
			}, mods...)
			var targets []*charactersv1.Character
			if name == "marked" {
				targets = []*charactersv1.Character{a.toren}
			}
			_, err := a.castOut(t, a.ana, a.pens, tableMark, slotOfLevel(1), false, targets)
			wantCode(t, "CastSpellOutsideCombat", err, connect.CodeFailedPrecondition)
			if !strings.Contains(err.Error(), "only in a combat") {
				t.Errorf("error = %v, want it to say the spell works only in a combat", err)
			}
			if n := a.count(t, `SELECT count(*) FROM character_effects`); n != 0 {
				t.Errorf("%d character effects, want none", n)
			}
			if n := a.count(t, `SELECT count(*) FROM spell_casts`); n != 0 {
				t.Errorf("%d casts, want none", n)
			}
		})
	}
}
