package play

import (
	"testing"

	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// newCastersWithMageArmor is newCasters with Mage Armor on Pensantus's list.
func newCastersWithMageArmor(t *testing.T) *armed {
	t.Helper()
	return newArmedWith(t, func(a *armed) {
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 5,
			&rulesv1.AbilityScores{Strength: 16, Dexterity: 13, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}, []string{battleaxe}, nil)
		a.pens = a.ana.caster(t, a.campaignID, "Pensantus", "class:wizard", "race:gnome", 3,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 14, Constitution: 12, Intelligence: 16, Wisdom: 10, Charisma: 8}, nil, []string{fireBolt},
			[]string{mageArmorKey, magicMissileSpell}, []string{mageArmorKey, magicMissileSpell})
		a.bri = a.bia.caster(t, a.campaignID, "Brisa", "class:cleric", "race:human", 3,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 16, Constitution: 14, Intelligence: 10, Wisdom: 16, Charisma: 8}, []string{maceKey}, []string{sacredFlame}, nil,
			[]string{cureWounds})
	})
}

func (a *armed) mageArmorAC(t *testing.T, label string) (ac int32, set bool) {
	t.Helper()
	var v *int32
	if err := a.h.pool.QueryRow(t.Context(), `SELECT mage_armor_ac FROM combatants WHERE id = $1`, a.id(t, label)).Scan(&v); err != nil {
		t.Fatalf("read the armor class of %s: %v", label, err)
	}
	if v == nil {
		return 0, false
	}
	return *v, true
}

// Mage Armor cast in a combat (SRD 5.1): the target's base AC becomes 13 + its Dexterity modifier,
// and the slot is spent. It used to fail with not_found because the base was worked out for no one.
func TestMageArmorCastInACombatSetsThirteenPlusDexterityAndSpendsTheSlot(t *testing.T) {
	t.Parallel()
	a := newCastersWithMageArmor(t)
	e := a.castersFight(t, 1)
	before := spellOption(a.mustOptions(t, a.ana, e, "Pensantus"), mageArmorKey).GetSlots()[0].GetFree()
	a.mustCast(t, a.ana, e, "Pensantus", mageArmorKey, slotOfLevel(1), a.at(t, "Toren"), noCastRoll)
	if ac, ok := a.mageArmorAC(t, "Toren"); !ok || ac != 15 { // human Dex 13 + 1 is 14: +2
		t.Errorf("Toren's base armor class = %d (%v), want 15 (13 + 2: a human's Dexterity 14)", ac, ok)
	}
	if f := cardOf(t, byLabel(t, a.get(t, a.master), "Toren"), mageArmorKey); f == nil {
		t.Error("Toren has no Mage Armor card")
	}
	if after := spellOption(a.mustOptions(t, a.ana, e, "Pensantus"), mageArmorKey).GetSlots()[0].GetFree(); after != before-1 {
		t.Errorf("free 1st-level slots = %d, want %d", after, before-1)
	}
}

// The master's "Adicionar efeito" of Mage Armor takes the same path.
func TestMageArmorAddedByTheMasterInACombatWorksOutTheBase(t *testing.T) {
	t.Parallel()
	a := newCastersWithMageArmor(t)
	e := a.castersFight(t, 1)
	a.mustAddEffect(t, e, mageArmorKey, []string{"Pensantus", "Toren"}, a.rounds(t, 10, "Pensantus"))
	if ac, ok := a.mageArmorAC(t, "Pensantus"); !ok || ac != 15 {
		t.Errorf("Pensantus's base armor class = %d (%v), want 15 (13 + 2)", ac, ok)
	}
	if ac, ok := a.mageArmorAC(t, "Toren"); !ok || ac != 15 {
		t.Errorf("Toren's base armor class = %d (%v), want 15 (13 + 2: a human's Dexterity 14)", ac, ok)
	}
}
