package play

import (
	"strings"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// A table spell that leaves an effect (RN-22, #579): the modifiers the master wrote count in the
// rolls and the damage of the creature that carries them, in a combat and out of it. These tests
// need the database (MEURPG_TEST_DATABASE_URL). The spell is invented: "Marca de Nanquim", a
// concentration spell on one creature that leaves whatever modifiers the test gives it.

const tableMark = "spell:marca-de-nanquim@mesa"

type tableMod = rulesv1.TableEffectModifier

func rollKinds(kinds ...rulesv1.TableEffectRollKind) []rulesv1.TableEffectRollKind { return kinds }

// newMarkCasters is the party of newTableCasters with the one spell Marca de Nanquim, which leaves the modifiers.
func newMarkCasters(t *testing.T, mods ...*tableMod) *armed {
	t.Helper()
	return newMarkCastersWith(t, nil, mods...)
}

// newMarkCastersWith lets the test reshape the spell (a self target, a save, an attack) before it is written.
func newMarkCastersWith(t *testing.T, shape func(*rulesv1.TableSpell), mods ...*tableMod) *armed {
	t.Helper()
	return newArmedWith(t, func(a *armed) {
		mark := tableSpell("Marca de Nanquim", 1, "class:wizard", rulesv1.SpellRangeKind_SPELL_RANGE_KIND_RANGED, tableTarget(rulesv1.TableSpellTargetKind_TABLE_SPELL_TARGET_KIND_CREATURE))
		mark.Concentration = true
		mark.Duration = &rulesv1.TableSpellDuration{Kind: rulesv1.SpellDurationKind_SPELL_DURATION_KIND_TIMED, Amount: 1, Unit: rulesv1.SpellDurationUnit_SPELL_DURATION_UNIT_MINUTE, UpTo: true}
		mark.Effect = &rulesv1.TableSpellEffect{Modifiers: mods, Visibility: rulesv1.TableEffectAudience_TABLE_EFFECT_AUDIENCE_PUBLIC}
		if shape != nil {
			shape(mark)
		}
		if _, err := a.master.table.CreateTableEntry(t.Context(), connect.NewRequest(&rulesv1.CreateTableEntryRequest{
			CampaignId: a.campaignID, Body: &rulesv1.CreateTableEntryRequest_TableSpell{TableSpell: mark},
		})); err != nil {
			t.Fatalf("CreateTableEntry(Marca de Nanquim) error = %v", err)
		}
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 5,
			&rulesv1.AbilityScores{Strength: 16, Dexterity: 13, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}, []string{battleaxe}, nil)
		book := []string{tableMark}
		a.pens = a.ana.caster(t, a.campaignID, "Pensantus", "class:wizard", "race:gnome", 5,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 14, Constitution: 12, Intelligence: 16, Wisdom: 10, Charisma: 8}, nil, []string{fireBolt}, book, book)
		a.bri = a.bia.caster(t, a.campaignID, "Brisa", "class:cleric", "race:human", 3,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 16, Constitution: 14, Intelligence: 10, Wisdom: 16, Charisma: 8}, []string{maceKey}, []string{sacredFlame}, nil, nil)
	})
}

// markOn casts the spell on the label in the first round (Pensantus goes first) and leaves Toren's turn next.
func (a *armed) markOn(t *testing.T, label string) *playv1.Encounter {
	t.Helper()
	e := a.castersFight(t, 1)
	a.mustCast(t, a.ana, e, "Pensantus", tableMark, slotOfLevel(1), a.at(t, label), noCastRoll)
	return e
}

func sourceTextsOf(sources []*playv1.AdvantageSource) string {
	var out []string
	for _, s := range sources {
		out = append(out, s.GetTextPt())
	}
	return strings.Join(out, " | ")
}

func hasSourceKind(sources []*playv1.AdvantageSource, kind playv1.AdvantageSourceKind, effect playv1.RollMode) bool {
	for _, s := range sources {
		if s.GetKind() == kind && s.GetEffect() == effect {
			return true
		}
	}
	return false
}

// The flat bonus of roll_bonus raises the attack roll, the saving throws and the ability checks of the creature
// that carries it, and is listed as a source of each.
func TestATableSpellRollBonusRaisesAnAttackASaveAndACheck(t *testing.T) {
	t.Parallel()
	a := newMarkCasters(t, &tableMod{
		Kind: rulesv1.TableEffectModifierKind_TABLE_EFFECT_MODIFIER_KIND_ROLL_BONUS, Value: 2,
		AppliesTo: rollKinds(rulesv1.TableEffectRollKind_TABLE_EFFECT_ROLL_KIND_ATTACK, rulesv1.TableEffectRollKind_TABLE_EFFECT_ROLL_KIND_SAVE, rulesv1.TableEffectRollKind_TABLE_EFFECT_ROLL_KIND_CHECK),
	})
	e := a.markOn(t, "Toren")

	// The attack: Toren's battleaxe is +6 (Strength +3, proficiency +3), +8 with the effect.
	a.passTo(t, e, "Toren")
	res := a.mustAttack(t, a.caio, e, "Toren", battleaxe, "Goblin", func(r *playv1.RollAttackRequest) { r.D20Faces = []int32{12} })
	if got := res.GetRoll().GetD20().GetModifier(); got != 8 {
		t.Errorf("the attack's modifier = %d, want 8", got)
	}
	if !strings.Contains(sourceTextsOf(res.GetRoll().GetSources()), "+2") {
		t.Errorf("the attack's sources = %q, want the +2 listed", sourceTextsOf(res.GetRoll().GetSources()))
	}

	// A save of the creature: Brisa's flame asks Toren a Dexterity save (+2 on the sheet) with the +2 on top; DC 13 is saved by an 11.
	a.passTo(t, e, "Brisa")
	a.h.roller.queue(11)
	cast := a.mustCast(t, a.master, e, "Brisa", sacredFlame, nil, a.at(t, "Toren"), noCastRoll)
	sv := cast.GetCast().GetTargets()[0].GetSave()
	if sv.GetRoll().GetModifier() != 4 || sv.GetOutcome() != playv1.SaveOutcome_SAVE_OUTCOME_SAVED {
		t.Errorf("the save = %v, want a modifier of +4 (the sheet's +2, the effect +2) that saves with an 11 against DC 13", sv)
	}
	if !strings.Contains(sourceTextsOf(sv.GetSources()), "+2") {
		t.Errorf("the save's sources = %q, want the +2 listed", sourceTextsOf(sv.GetSources()))
	}

	// A check, and a save out of the spell flow, read the same bonus.
	toren := a.toren.GetId()
	for _, c := range []struct {
		what         string
		skill, abil  string
		check        bool
		wantBonus    int
		wantListedBy string
	}{
		{"a check", "skill:perception", "wis", true, 2, tableMark},
		{"a save", "", "dex", false, 2, tableMark},
	} {
		cm := a.checkModeFor(t, toren, c.skill, c.abil, c.check)
		if len(cm.Bonuses) != 1 || cm.Bonuses[0].Value != c.wantBonus || cm.Bonuses[0].Key != c.wantListedBy {
			t.Errorf("%s: bonuses = %+v, want +%d from the spell", c.what, cm.Bonuses, c.wantBonus)
		}
	}

}

// The bonus only counts for the rolls it names.
func TestATableSpellRollBonusForSavesLeavesTheAttackAlone(t *testing.T) {
	t.Parallel()
	a := newMarkCasters(t, &tableMod{
		Kind: rulesv1.TableEffectModifierKind_TABLE_EFFECT_MODIFIER_KIND_ROLL_BONUS, Value: 1,
		AppliesTo: rollKinds(rulesv1.TableEffectRollKind_TABLE_EFFECT_ROLL_KIND_SAVE),
	})
	e := a.markOn(t, "Toren")
	a.passTo(t, e, "Toren")
	res := a.mustAttack(t, a.caio, e, "Toren", battleaxe, "Goblin", func(r *playv1.RollAttackRequest) { r.D20Faces = []int32{12} })
	if got := res.GetRoll().GetD20().GetModifier(); got != 6 {
		t.Errorf("the attack's modifier = %d, want 6", got)
	}
	if cm := a.checkModeFor(t, a.toren.GetId(), "skill:perception", "wis", true); len(cm.Bonuses) != 0 {
		t.Errorf("a check got %+v from a bonus for saves", cm.Bonuses)
	}
}

// attack_advantage gives the carrier's attack roll advantage, and a disadvantage of the same effect cancels it.
func TestATableSpellAttackAdvantageGivesAnAttackAdvantage(t *testing.T) {
	t.Parallel()
	a := newMarkCasters(t, &tableMod{Kind: rulesv1.TableEffectModifierKind_TABLE_EFFECT_MODIFIER_KIND_ATTACK_ADVANTAGE})
	e := a.markOn(t, "Toren")
	a.passTo(t, e, "Toren")
	res := a.mustAttack(t, a.caio, e, "Toren", battleaxe, "Goblin", func(r *playv1.RollAttackRequest) { r.D20Faces = []int32{12, 3} })
	roll := res.GetRoll()
	if roll.GetMode() != playv1.RollMode_ROLL_MODE_ADVANTAGE || !hasSourceKind(roll.GetSources(), playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_EFFECT_ATTACK, playv1.RollMode_ROLL_MODE_ADVANTAGE) {
		t.Errorf("the attack = mode %v, sources %q; want advantage from the effect", roll.GetMode(), sourceTextsOf(roll.GetSources()))
	}
}

func TestATableSpellAdvantageAndDisadvantageOnAttacksCancel(t *testing.T) {
	t.Parallel()
	a := newMarkCasters(t,
		&tableMod{Kind: rulesv1.TableEffectModifierKind_TABLE_EFFECT_MODIFIER_KIND_ATTACK_ADVANTAGE},
		&tableMod{Kind: rulesv1.TableEffectModifierKind_TABLE_EFFECT_MODIFIER_KIND_ATTACK_DISADVANTAGE},
	)
	e := a.markOn(t, "Toren")
	a.passTo(t, e, "Toren")
	res := a.mustAttack(t, a.caio, e, "Toren", battleaxe, "Goblin", func(r *playv1.RollAttackRequest) { r.D20Faces = []int32{12} })
	if res.GetRoll().GetMode() != playv1.RollMode_ROLL_MODE_NORMAL {
		t.Errorf("the attack's mode = %v, want a normal roll: the advantage and the disadvantage cancel", res.GetRoll().GetMode())
	}
}

// save_disadvantage and check_disadvantage hold for the abilities they name, in a spell's save and in a check.
func TestATableSpellDisadvantageOnSavesAndChecks(t *testing.T) {
	t.Parallel()
	a := newMarkCasters(t,
		&tableMod{Kind: rulesv1.TableEffectModifierKind_TABLE_EFFECT_MODIFIER_KIND_SAVE_DISADVANTAGE, Abilities: []string{"dex"}},
		&tableMod{Kind: rulesv1.TableEffectModifierKind_TABLE_EFFECT_MODIFIER_KIND_CHECK_DISADVANTAGE, Abilities: []string{"str"}},
	)
	e := a.markOn(t, "Toren")

	a.passTo(t, e, "Brisa")
	// The flame's save is rolled with two dice and the lower one counts: the 4 and the sheet's +2 do not reach DC 13.
	a.h.roller.queue(15, 4)
	cast := a.mustCast(t, a.master, e, "Brisa", sacredFlame, nil, a.at(t, "Toren"), noCastRoll)
	sv := cast.GetCast().GetTargets()[0].GetSave()
	if sv.GetMode() != playv1.RollMode_ROLL_MODE_DISADVANTAGE || sv.GetOutcome() != playv1.SaveOutcome_SAVE_OUTCOME_FAILED || !hasSourceKind(sv.GetSources(), playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_EFFECT_SAVE, playv1.RollMode_ROLL_MODE_DISADVANTAGE) {
		t.Errorf("the save = %v, want disadvantage from the effect and a fail", sv)
	}

	toren := a.toren.GetId()
	for _, c := range []struct {
		what        string
		skill, abil string
		check       bool
		want        combat.RollMode
	}{
		{"a Dexterity save", "", "dex", false, combat.ModeDisadvantage},
		{"a Wisdom save", "", "wis", false, combat.ModeNormal},
		{"a Strength check", "skill:athletics", "str", true, combat.ModeDisadvantage},
		{"a Dexterity check", "skill:acrobatics", "dex", true, combat.ModeNormal},
		{"a Strength save", "", "str", false, combat.ModeNormal},
	} {
		if cm := a.checkModeFor(t, toren, c.skill, c.abil, c.check); cm.Mode != c.want {
			t.Errorf("%s: mode = %v (%v), want %v", c.what, cm.Mode, sourceKinds(cm), c.want)
		}
	}
}

// resistance, immunity and vulnerability change the damage the carrier takes, by the same steps as a race's: the 6
// typed is 9 with Toren's Strength bonus, so the goblin (7 hit points) takes 4, none or all of it.
func TestATableSpellResistanceChangesTheDamageTheCreatureTakes(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		kind rulesv1.TableResistanceKind
		want int32 // the goblin's hit points after the damage (it has 7)
	}{
		{"resist", rulesv1.TableResistanceKind_TABLE_RESISTANCE_KIND_RESIST, 3},
		{"immune", rulesv1.TableResistanceKind_TABLE_RESISTANCE_KIND_IMMUNE, 7},
		{"vulnerable", rulesv1.TableResistanceKind_TABLE_RESISTANCE_KIND_VULNERABLE, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			a := newMarkCasters(t, &tableMod{
				Kind: rulesv1.TableEffectModifierKind_TABLE_EFFECT_MODIFIER_KIND_RESISTANCE, ResistanceKind: c.kind, DamageTypes: []string{"damage-type:slashing"},
			})
			e := a.markOn(t, "Goblin")
			if before, _, _ := a.hp(t, "Goblin"); before != 7 {
				t.Fatalf("the goblin has %d hit points, want 7", before)
			}
			a.passTo(t, e, "Toren")
			res := a.mustAttack(t, a.caio, e, "Toren", battleaxe, "Goblin", func(r *playv1.RollAttackRequest) { r.D20Faces = []int32{15} })
			a.mustDamage(t, a.caio, e, res.GetPendingDamage().GetId(), typedDamage(6))
			if got, _, _ := a.hp(t, "Goblin"); got != c.want {
				t.Errorf("the goblin has %d hit points after the hit, want %d", got, c.want)
			}
		})
	}
}

// The resistance of one damage type leaves the others alone.
func TestATableSpellResistanceOnlyCoversItsDamageTypes(t *testing.T) {
	t.Parallel()
	a := newMarkCasters(t, &tableMod{
		Kind: rulesv1.TableEffectModifierKind_TABLE_EFFECT_MODIFIER_KIND_RESISTANCE, DamageTypes: []string{"damage-type:fire"},
	})
	e := a.markOn(t, "Goblin")
	a.passTo(t, e, "Toren")
	res := a.mustAttack(t, a.caio, e, "Toren", battleaxe, "Goblin", func(r *playv1.RollAttackRequest) { r.D20Faces = []int32{15} })
	a.mustDamage(t, a.caio, e, res.GetPendingDamage().GetId(), typedDamage(6))
	if _, _, down := a.hp(t, "Goblin"); !down {
		t.Error("the goblin survived 6 slashing damage and a Strength bonus: the resistance is to fire, it should take all of it")
	}
}

// Out of a combat the effect is a row of the character, and its bonus and disadvantage count in a check and a save.
func TestATableSpellRollBonusAndDisadvantageCountOutsideACombat(t *testing.T) {
	t.Parallel()
	a := newMarkCasters(t,
		&tableMod{
			Kind: rulesv1.TableEffectModifierKind_TABLE_EFFECT_MODIFIER_KIND_ROLL_BONUS, Value: 3,
			AppliesTo: rollKinds(rulesv1.TableEffectRollKind_TABLE_EFFECT_ROLL_KIND_CHECK, rulesv1.TableEffectRollKind_TABLE_EFFECT_ROLL_KIND_SAVE),
		},
		&tableMod{Kind: rulesv1.TableEffectModifierKind_TABLE_EFFECT_MODIFIER_KIND_SAVE_DISADVANTAGE, Abilities: []string{"wis"}},
		&tableMod{Kind: rulesv1.TableEffectModifierKind_TABLE_EFFECT_MODIFIER_KIND_CHECK_DISADVANTAGE, Abilities: []string{"cha"}},
	)
	e := a.markOn(t, "Toren")
	a.endEncounter(t, e)
	toren := a.toren.GetId()
	save := a.checkModeFor(t, toren, "", "wis", false)
	if save.Mode != combat.ModeDisadvantage || bonusTotal(save.Bonuses) != 3 {
		t.Errorf("a Wisdom save: mode %v, bonus %d; want disadvantage and +3", save.Mode, bonusTotal(save.Bonuses))
	}
	check := a.checkModeFor(t, toren, "skill:persuasion", "cha", true)
	if check.Mode != combat.ModeDisadvantage || bonusTotal(check.Bonuses) != 3 {
		t.Errorf("a Charisma check: mode %v, bonus %d; want disadvantage and +3", check.Mode, bonusTotal(check.Bonuses))
	}
	if other := a.checkModeFor(t, toren, "skill:perception", "wis", true); other.Mode != combat.ModeNormal {
		t.Errorf("a Wisdom check: mode %v, want normal: the effect's disadvantage is on the save", other.Mode)
	}
}
