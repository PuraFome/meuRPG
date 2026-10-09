package play

import (
	"strings"
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// Mage Armor (SRD 5.1): the base armor class becomes 13 + Dexterity for a willing creature that
// wears no armor, for 8 hours; it ends if the target dons armor. Cast outside a combat it is an
// effect on the character: the session's panels read its armor class from the vitals, and the
// combat reads it from the combatant.

// mageArmorOnPensantus has the wizard cast Mage Armor on herself (Dexterity 14: 13 + 2 = 15).
func (a *armed) mageArmorOnPensantus(t *testing.T) *playv1.OutsideCast {
	t.Helper()
	return a.mustCastOut(t, a.ana, a.pens, mageArmorKey, slotOfLevel(1), false, []*charactersv1.Character{a.pens}).GetCast()
}

func (a *armed) armorEffectSeconds(t *testing.T, characterID string) (seconds int32, found bool) {
	t.Helper()
	rows, err := a.h.pool.Query(t.Context(), `SELECT seconds_left FROM character_effects WHERE character_id = $1 AND source_key = $2`, characterID, mageArmorKey)
	if err != nil {
		t.Fatalf("read the effect: %v", err)
	}
	defer rows.Close()
	if rows.Next() {
		if err := rows.Scan(&seconds); err != nil {
			t.Fatalf("read the effect: %v", err)
		}
		return seconds, true
	}
	return 0, false
}

func TestMageArmorCastOutsideCombatIsAnEffectThePanelsRead(t *testing.T) {
	t.Parallel()
	a := newCastingParty(t)
	cast := a.mageArmorOnPensantus(t)
	if seconds, ok := a.armorEffectSeconds(t, a.pens.GetId()); !ok || seconds != 8*3600 {
		t.Fatalf("the effect = %d seconds, %v; want 28800 (8 hours)", seconds, ok)
	}
	// The sheet says 12 (10 + Dex); the vitals the session's panels read say 15 (13 + Dex).
	if got := a.ana.character(t, a.pens).GetDerived().GetArmorClass(); got != 12 {
		t.Fatalf("the sheet's armor class = %d, want 12", got)
	}
	if got := a.vitals(t, a.pens).GetArmorClassBase(); got != 15 {
		t.Errorf("the base armor class in the vitals = %d, want 15", got)
	}
	// The player and the master read the same in the session.
	for _, u := range []*user{a.ana, a.master} {
		live := u.liveSession(t, a.campaignID).GetVitals()
		found := false
		for _, v := range live {
			found = found || (v.GetCharacterId() == a.pens.GetId() && v.GetArmorClassBase() == 15)
		}
		if !found {
			t.Errorf("a session read of the vitals does not hold the base armor class 15: %v", live)
		}
	}
	// A casting of the same spell does not stack, and the hours go down with the game time.
	a.advanceGameTime(t, 3600)
	if seconds, _ := a.armorEffectSeconds(t, a.pens.GetId()); seconds != 8*3600-3600 {
		t.Errorf("after an hour the effect has %d seconds, want %d", seconds, 8*3600-3600)
	}
	// The master ends the cast: the panels go back to the sheet's number.
	if _, err := a.endSpell(t, a.master, cast.GetId()); err != nil {
		t.Fatalf("EndActiveSpell() error = %v", err)
	}
	if _, ok := a.armorEffectSeconds(t, a.pens.GetId()); ok {
		t.Error("the effect stayed after the cast ended")
	}
	if got := a.vitals(t, a.pens).GetArmorClassBase(); got != 0 {
		t.Errorf("the base armor class after the end = %d, want 0", got)
	}
}

func TestMageArmorGoesIntoACombatAndBackOutWithItsTime(t *testing.T) {
	t.Parallel()
	a := newCastingParty(t)
	a.mageArmorOnPensantus(t)
	a.advanceGameTime(t, 12) // two rounds of game time before the fight
	e := a.closeFight(t)
	pens := byLabel(t, e, "Pensantus")
	f := cardOf(t, pens, mageArmorKey)
	if f == nil || f.GetRoundsLeft() != (8*3600-12)/6 {
		t.Fatalf("Pensantus's Mage Armor in the combat = %v, want %d rounds left", f, (8*3600-12)/6)
	}
	var ac int32
	if err := a.h.pool.QueryRow(t.Context(), `SELECT mage_armor_ac FROM combatants WHERE id = $1`, pens.GetId()).Scan(&ac); err != nil || ac != 15 {
		t.Errorf("the armor class in the combat = %d (%v), want 15", ac, err)
	}
	if got := a.vitals(t, a.pens).GetArmorClassBase(); got != 15 {
		t.Errorf("the vitals during the combat say %d, want 15", got)
	}
	// An attack against her is compared with 15, not 12: a 13 + modifier that would hit 12 misses 15.
	if _, err := a.master.combat.EndEncounter(t.Context(), connect.NewRequest(&playv1.EndEncounterRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey()})); err != nil {
		t.Fatalf("EndEncounter() error = %v", err)
	}
	seconds, ok := a.armorEffectSeconds(t, a.pens.GetId())
	if !ok || seconds < 8*3600-12-6*2 || seconds > 8*3600-12 || seconds%6 != 0 {
		t.Errorf("after the combat the effect has %d seconds (%v), want about 28788 in whole rounds", seconds, ok)
	}
	if got := a.vitals(t, a.pens).GetArmorClassBase(); got != 15 {
		t.Errorf("the vitals after the combat say %d, want 15", got)
	}
}

func TestMageArmorEndsWithALongRestAndAShortOneOnlyCountsTheHour(t *testing.T) {
	t.Parallel()
	a := newCastingParty(t)
	a.mageArmorOnPensantus(t)
	rest := func(kind playv1.RestKind) {
		t.Helper()
		if _, err := a.master.resource.TakeRest(t.Context(), connect.NewRequest(&playv1.TakeRestRequest{CampaignId: a.campaignID, Kind: kind, IdempotencyKey: newKey()})); err != nil {
			t.Fatalf("TakeRest(%v) error = %v", kind, err)
		}
	}
	rest(playv1.RestKind_REST_KIND_SHORT)
	if seconds, ok := a.armorEffectSeconds(t, a.pens.GetId()); !ok || seconds != 8*3600-3600 {
		t.Fatalf("after a short rest the effect has %d seconds (%v), want %d", seconds, ok, 8*3600-3600)
	}
	if got := a.vitals(t, a.pens).GetArmorClassBase(); got != 15 {
		t.Errorf("after a short rest the vitals say %d, want 15", got)
	}
	rest(playv1.RestKind_REST_KIND_LONG)
	if _, ok := a.armorEffectSeconds(t, a.pens.GetId()); ok {
		t.Error("the effect survived a long rest")
	}
	if got := a.vitals(t, a.pens).GetArmorClassBase(); got != 0 {
		t.Errorf("after a long rest the vitals say %d, want 0", got)
	}
}

func TestMageArmorEndsWhenTheTargetDonsArmor(t *testing.T) {
	t.Parallel()
	a := newCastingParty(t)
	cast := a.mageArmorOnPensantus(t)
	c := a.ana.character(t, a.pens)
	sheet := c.GetSheet()
	sheet.GetFull().ArmorKey = leatherArmorKey
	if _, err := a.master.characters.UpdateCharacter(t.Context(), connect.NewRequest(&charactersv1.UpdateCharacterRequest{
		CampaignId: a.campaignID, CharacterId: c.GetId(), Revision: c.GetRevision(), Name: c.GetName(), Sheet: sheet,
	})); err != nil {
		t.Fatalf("UpdateCharacter() error = %v", err)
	}
	if _, ok := a.armorEffectSeconds(t, a.pens.GetId()); ok {
		t.Error("Mage Armor stayed on a character that donned armor")
	}
	if got := a.vitals(t, a.pens).GetArmorClassBase(); got != 0 {
		t.Errorf("the vitals say %d after the armor, want 0", got)
	}
	if got := a.castByID(t, a.master, cast.GetId()); got == nil || got.GetStatus() != playv1.OutsideCastStatus_OUTSIDE_CAST_STATUS_ENDED {
		t.Errorf("the cast = %v, want ended", got)
	}
}

// Mage Armor cannot be cast on a creature that wears armor (SRD 5.1): no effect is made.
func TestMageArmorOnAnArmoredTargetMakesNoEffect(t *testing.T) {
	t.Parallel()
	a := newCastingParty(t)
	c := a.bia.character(t, a.bri)
	sheet := c.GetSheet()
	sheet.GetFull().ArmorKey = leatherArmorKey
	if _, err := a.master.characters.UpdateCharacter(t.Context(), connect.NewRequest(&charactersv1.UpdateCharacterRequest{
		CampaignId: a.campaignID, CharacterId: c.GetId(), Revision: c.GetRevision(), Name: c.GetName(), Sheet: sheet,
	})); err != nil {
		t.Fatalf("UpdateCharacter() error = %v", err)
	}
	if _, err := a.castOut(t, a.ana, a.pens, mageArmorKey, slotOfLevel(1), false, []*charactersv1.Character{a.bri}); err == nil {
		if _, ok := a.armorEffectSeconds(t, a.bri.GetId()); ok {
			t.Error("Mage Armor took hold of a creature that wears armor")
		}
	}
}

// Mage Armor is 13 + Dexterity (SRD 5.1) and a shield still adds its 2: the base is worked out
// again from the gear the character carries when the sheet changes, not fixed at the cast.
func TestMageArmorFollowsAShieldPickedUpAfterTheCast(t *testing.T) {
	t.Parallel()
	a := newCastingParty(t)
	a.mageArmorOnPensantus(t)
	if got := a.vitals(t, a.pens).GetArmorClassBase(); got != 15 {
		t.Fatalf("the base armor class after the cast = %d, want 15", got)
	}
	c := a.ana.character(t, a.pens)
	sheet := c.GetSheet()
	sheet.GetFull().Shield = true
	if _, err := a.master.characters.UpdateCharacter(t.Context(), connect.NewRequest(&charactersv1.UpdateCharacterRequest{
		CampaignId: a.campaignID, CharacterId: c.GetId(), Revision: c.GetRevision(), Name: c.GetName(), Sheet: sheet,
	})); err != nil {
		t.Fatalf("UpdateCharacter() error = %v", err)
	}
	if got := a.vitals(t, a.pens).GetArmorClassBase(); got != 17 {
		t.Errorf("the base armor class with a shield = %d, want 17 (13 + Dex 2 + shield 2)", got)
	}
	// The combat reads the same number.
	e := a.closeFight(t)
	var ac int32
	if err := a.h.pool.QueryRow(t.Context(), `SELECT mage_armor_ac FROM combatants WHERE id = $1`, byLabel(t, e, "Pensantus").GetId()).Scan(&ac); err != nil || ac != 17 {
		t.Errorf("the combat's base armor class = %d (%v), want 17", ac, err)
	}
	// And the shield put away takes it back down.
	c = a.ana.character(t, a.pens)
	sheet = c.GetSheet()
	sheet.GetFull().Shield = false
	if _, err := a.master.characters.UpdateCharacter(t.Context(), connect.NewRequest(&charactersv1.UpdateCharacterRequest{
		CampaignId: a.campaignID, CharacterId: c.GetId(), Revision: c.GetRevision(), Name: c.GetName(), Sheet: sheet,
	})); err != nil {
		t.Fatalf("UpdateCharacter() error = %v", err)
	}
	if got := a.vitals(t, a.pens).GetArmorClassBase(); got != 15 {
		t.Errorf("the base armor class without the shield = %d, want 15", got)
	}
}

// A player reads the rounds left of an effect the master added: the count is hidden only for a
// caster the player does not see.
func TestAMasterAddedEffectShowsItsRoundsLeftToThePlayer(t *testing.T) {
	t.Parallel()
	a := newCastingParty(t)
	e := a.closeFight(t)
	a.mustAddEffect(t, e, blessKey, []string{"Toren"}, a.rounds(t, 10, "Toren"))
	c := byLabel(t, a.get(t, a.caio), "Toren")
	f := cardOf(t, c, blessKey)
	if f == nil || !strings.Contains(f.GetClockTextPt(), "Restam 10 rodadas") {
		t.Fatalf("Toren's Bless as the player reads it = %v, want the rounds left", f)
	}
}
