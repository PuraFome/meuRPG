package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// Shield of Faith (SRD 5.1): a bonus action, a creature within 60 feet, +2 to its armor class while
// the caster concentrates (up to 10 minutes).
const shieldOfFaithKey = "spell:shield-of-faith"

// newCastersWithShieldOfFaith is newCasters with Shield of Faith among Brisa's prepared spells.
func newCastersWithShieldOfFaith(t *testing.T) *armed {
	t.Helper()
	return newArmedWith(t, func(a *armed) {
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 5,
			&rulesv1.AbilityScores{Strength: 16, Dexterity: 13, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}, []string{battleaxe}, nil)
		a.pens = a.ana.caster(t, a.campaignID, "Pensantus", "class:wizard", "race:gnome", 3,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 14, Constitution: 12, Intelligence: 16, Wisdom: 10, Charisma: 8}, nil, []string{fireBolt},
			[]string{magicMissileSpell}, []string{magicMissileSpell})
		a.bri = a.bia.caster(t, a.campaignID, "Brisa", "class:cleric", "race:human", 3,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 16, Constitution: 14, Intelligence: 10, Wisdom: 16, Charisma: 8}, []string{maceKey}, []string{sacredFlame}, nil,
			[]string{cureWounds, shieldOfFaithKey})
	})
}

// The cast spends the bonus action and a slot, Toren's armor class goes up by 2 with the card on
// him, and ending Brisa's concentration takes both away (the effect lasts while she concentrates).
func TestShieldOfFaithGivesTwoArmorClassWhileTheCasterConcentrates(t *testing.T) {
	t.Parallel()
	a := newCastersWithShieldOfFaith(t)
	a.castersFight(t, 1)
	e := a.turnOf(t, "Brisa")
	before := byLabel(t, e, "Toren").GetArmorClassBonus()
	res := a.mustCast(t, a.bia, e, "Brisa", shieldOfFaithKey, slotOfLevel(1), a.at(t, "Toren"), noCastRoll)
	if c := byLabel(t, res.GetEncounter(), "Brisa"); c.GetActionUsed() || !c.GetBonusActionUsed() {
		t.Errorf("economy after Shield of Faith = action %v, bonus %v; want only the bonus action spent", c.GetActionUsed(), c.GetBonusActionUsed())
	}
	toren := byLabel(t, a.get(t, a.master), "Toren")
	if got := toren.GetArmorClassBonus(); got != before+2 {
		t.Errorf("Toren's armor class bonus = %d, want %d (+2)", got, before+2)
	}
	if f := cardOf(t, toren, shieldOfFaithKey); f == nil {
		t.Fatal("Toren has no Shield of Faith card")
	}
	if _, err := a.bia.combat.EndConcentration(t.Context(), connect.NewRequest(&playv1.EndConcentrationRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, "Brisa"), IdempotencyKey: newKey(),
	})); err != nil {
		t.Fatalf("EndConcentration() error = %v", err)
	}
	toren = byLabel(t, a.get(t, a.master), "Toren")
	if got := toren.GetArmorClassBonus(); got != before {
		t.Errorf("Toren's armor class bonus after the concentration ended = %d, want %d", got, before)
	}
	if f := cardOf(t, toren, shieldOfFaithKey); f != nil {
		t.Errorf("the Shield of Faith card is still on Toren: %v", f)
	}
}
