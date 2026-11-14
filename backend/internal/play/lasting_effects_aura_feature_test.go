package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// The aura of a feature (#579, slice 3): Toren's table subclass carries Aura de Proteção, the shape of the paladin's: a
// bonus on saves and checks of his Charisma modifier (at least +1), 10 feet around him, to him and his side. The rows are
// written when he joins the combat; nothing about who is inside is. These tests need the database.

// protectionAura is the aura effect of the feature: the radius and the side, and the bonus from the bearer's ability.
func protectionAura(radius int32) []*rulesv1.TableEffect {
	return []*rulesv1.TableEffect{{Type: "aura", Aura: &rulesv1.TableSpellEffect{
		Aura:       &rulesv1.TableEffectAura{RadiusFt: radius, Affects: rulesv1.TableAuraAffects_TABLE_AURA_AFFECTS_ALLIES},
		Visibility: publicAura,
		Modifiers: []*tableMod{{
			Kind: rulesv1.TableEffectModifierKind_TABLE_EFFECT_MODIFIER_KIND_ROLL_BONUS, ValueAbility: "cha", ValueMin: 1,
			AppliesTo: rollKinds(rulesv1.TableEffectRollKind_TABLE_EFFECT_ROLL_KIND_SAVE, rulesv1.TableEffectRollKind_TABLE_EFFECT_ROLL_KIND_CHECK),
		}},
	}}}
}

// auraRig is the party of the flag tests with Toren's charisma set: the master gives Toren the feature.
func newAuraRig(t *testing.T, cha int32, effects []*rulesv1.TableEffect) *armed {
	t.Helper()
	return newArmedWith(t, func(a *armed) {
		res, err := a.master.table.CreateTableEntry(t.Context(), connect.NewRequest(&rulesv1.CreateTableEntryRequest{
			CampaignId: a.campaignID, Body: &rulesv1.CreateTableEntryRequest_TableSubclass{TableSubclass: flagSubclass("class:fighter", effects, "")},
		}))
		if err != nil {
			t.Fatalf("CreateTableEntry() error = %v", err)
		}
		a.toren = a.caio.heroWith(t, a.campaignID, "Toren", classLevel("class:fighter", 5, res.Msg.GetEntry().GetKey()),
			abilities(16, 13, 14, 10, cha), []string{longsword}, nil, []string{"feature:fighter-fighting-style-defense"})
		a.pens = a.ana.hero(t, a.campaignID, "Pensantus", "class:wizard", "race:gnome", 3, scores16(), nil, []string{fireBolt})
		a.bri = a.bia.hero(t, a.campaignID, "Brisa", "class:fighter", "race:human", 2, scores16(), []string{rapier}, nil)
	})
}

func (a *armed) bonusOf(t *testing.T, id string) int {
	t.Helper()
	n := 0
	for _, b := range a.checkModeFor(t, id, "skill:perception", "wis", true).Bonuses {
		n += b.Value
	}
	return n
}

// The bonus is the bearer's Charisma modifier (+3 for 16), read where each creature stands: Brisa on the edge of the 10 feet
// has it, Pensantus at 15 feet has not, and Toren, the bearer, has it once.
func TestAFeatureAuraGivesTheBearersAbilityModifierInsideTheRadius(t *testing.T) {
	t.Parallel()
	a := newAuraRig(t, 16, protectionAura(10))
	a.arena(t, arenaPlan{at: map[string][2]int32{"Brisa": {5, 3}}})
	for label, want := range map[string]int{"Toren": 3, "Brisa": 3, "Pensantus": 0} {
		id := map[string]string{"Toren": a.toren.GetId(), "Brisa": a.bri.GetId(), "Pensantus": a.pens.GetId()}[label]
		if got := a.bonusOf(t, id); got != want {
			t.Errorf("%s: bonus = %d, want %d", label, got, want)
		}
	}
}

// A modifier of 0 or less gives the least the feature says: +1 (Aura of Protection).
func TestAFeatureAuraNeverGivesLessThanItsLeast(t *testing.T) {
	t.Parallel()
	a := newAuraRig(t, 8, protectionAura(10)) // Charisma 8: -1
	a.arena(t, arenaPlan{at: map[string][2]int32{"Brisa": {5, 3}}})
	if got := a.bonusOf(t, a.bri.GetId()); got != 1 {
		t.Errorf("Brisa: bonus = %d, want 1", got)
	}
}

// The aura is the bearer's while it is up: not at 0 hit points, and not incapacitated; it is back when he stands again.
func TestAFeatureAuraEndsWhileItsBearerIsDownOrIncapacitated(t *testing.T) {
	t.Parallel()
	a := newAuraRig(t, 16, protectionAura(10))
	c := a.arena(t, arenaPlan{at: map[string][2]int32{"Brisa": {5, 3}}})
	if got := a.bonusOf(t, a.bri.GetId()); got != 3 {
		t.Fatalf("Brisa with the paladin up: bonus = %d, want 3", got)
	}
	a.setConditions(t, c.e, "Toren", "condition:unconscious")
	if got := a.bonusOf(t, a.bri.GetId()); got != 0 {
		t.Errorf("Brisa with the bearer unconscious: bonus = %d, want 0", got)
	}
	if got := a.bonusOf(t, a.toren.GetId()); got != 0 {
		t.Errorf("the bearer unconscious: bonus = %d, want 0", got)
	}
	a.setConditions(t, c.e, "Toren")
	if got := a.bonusOf(t, a.bri.GetId()); got != 3 {
		t.Errorf("Brisa with the bearer up again: bonus = %d, want 3", got)
	}
	a.hurt(t, a.toren, 0)
	if got := a.bonusOf(t, a.bri.GetId()); got != 0 {
		t.Errorf("Brisa with the bearer at 0 hit points: bonus = %d, want 0", got)
	}
	a.hurt(t, a.toren, 4)
	if got := a.bonusOf(t, a.bri.GetId()); got != 3 {
		t.Errorf("Brisa with the bearer healed to 4: bonus = %d, want 3", got)
	}
}

// The allies of the bearer include the bearer: he reads the bonus once, not twice, and the effect is one row on him that
// the combat's end takes away: the next combat makes it again from the sheet.
func TestAFeatureAuraIsOneRowOnItsBearerThatTheCombatEndTakesAway(t *testing.T) {
	t.Parallel()
	a := newAuraRig(t, 16, protectionAura(10))
	c := a.arena(t, arenaPlan{at: map[string][2]int32{"Brisa": {5, 3}}})
	count := func() int {
		var n int
		if err := a.h.pool.QueryRow(t.Context(), `SELECT count(*) FROM combatant_states s JOIN combatants c ON c.id = s.combatant_id WHERE c.encounter_id = $1 AND s.source_kind = 'feature'`, c.e.GetId()).Scan(&n); err != nil {
			t.Fatalf("count the rows: %v", err)
		}
		return n
	}
	if n := count(); n != 1 {
		t.Errorf("rows of the aura = %d, want 1", n)
	}
	if got := a.bonusOf(t, a.toren.GetId()); got != 3 {
		t.Errorf("the bearer: bonus = %d, want 3, once", got)
	}
	if _, err := a.master.combat.EndEncounter(t.Context(), connect.NewRequest(&playv1.EndEncounterRequest{CampaignId: a.campaignID, EncounterId: c.e.GetId(), IdempotencyKey: newKey()})); err != nil {
		t.Fatalf("EndEncounter() error = %v", err)
	}
	var records int
	if err := a.h.pool.QueryRow(t.Context(), `SELECT count(*) FROM character_effects WHERE character_id = $1`, a.toren.GetId()).Scan(&records); err != nil {
		t.Fatalf("count the records: %v", err)
	}
	if records != 0 {
		t.Errorf("the character's records = %d after the combat, want 0: the aura is made again from the sheet", records)
	}
}
