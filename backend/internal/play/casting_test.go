package play

import (
	"errors"
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// Casting outside a combat (SRD 5.1, "Spellcasting"): healing between fights, rituals,
// the spells that take minutes or hours, concentration, and what a combat does to them.
// These tests need the database (MEURPG_TEST_DATABASE_URL). The fixture is the party of
// the designs: Pensantus, a level 5 wizard with a spellbook; Brisa, a level 3 cleric of
// the Life Domain; Toren, a level 5 fighter; and the master's Capitão Goblin and Goblin.

const (
	mageArmorKey    = "spell:mage-armor"
	detectMagicKey  = "spell:detect-magic"
	prayerKey       = "spell:prayer-of-healing"
	blessKey        = "spell:bless"
	alarmKey        = "spell:alarm"
	identifyKey     = "spell:identify"
	leatherArmorKey = "equipment:leather-armor"
)

// castingHero creates a player's character with a class, a subclass, spells and armor.
func (u *user) castingHero(t *testing.T, campaignID, name string, cl *charactersv1.ClassLevel, scores *rulesv1.AbilityScores, cantrips, known, prepared []string, armor string) *charactersv1.Character {
	t.Helper()
	sheet := &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
		BaseScores: scores, RaceKey: "race:human", Classes: []*charactersv1.ClassLevel{cl},
		CantripKeys: cantrips, KnownSpellKeys: known, PreparedSpellKeys: prepared, ArmorKey: armor,
	}}}
	res, err := u.characters.CreateCharacter(t.Context(), connect.NewRequest(&charactersv1.CreateCharacterRequest{
		CampaignId: campaignID, Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Name: name, Sheet: sheet,
	}))
	if err != nil {
		t.Fatalf("CreateCharacter(%s) error = %v", name, err)
	}
	return res.Msg.GetCharacter()
}

// newCastingParty is the fixture of this file.
func newCastingParty(t *testing.T) *armed {
	t.Helper()
	return newArmedWith(t, func(a *armed) {
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 5,
			&rulesv1.AbilityScores{Strength: 16, Dexterity: 13, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}, []string{battleaxe}, nil)
		a.pens = a.ana.castingHero(t, a.campaignID, "Pensantus", classLevel("class:wizard", 5, ""),
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 14, Constitution: 12, Intelligence: 16, Wisdom: 10, Charisma: 8}, []string{fireBolt},
			[]string{mageArmorKey, detectMagicKey, alarmKey, identifyKey, magicMissileSpell, shieldSpell, holdPerson},
			[]string{mageArmorKey, magicMissileSpell, shieldSpell}, "")
		a.bri = a.bia.castingHero(t, a.campaignID, "Brisa", classLevel("class:cleric", 3, "subclass:life"),
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 16, Constitution: 14, Intelligence: 10, Wisdom: 16, Charisma: 8}, []string{sacredFlame}, nil,
			[]string{cureWounds, healingWord, prayerKey, detectMagicKey, blessKey, aidSpell}, "")
	})
}

// castOut calls CastSpellOutsideCombat as u, with a key of its own.
func (a *armed) castOut(t *testing.T, u *user, caster *charactersv1.Character, spell string, slot *playv1.SpellSlot, ritual bool, targets []*charactersv1.Character, edit ...func(*playv1.CastSpellOutsideCombatRequest)) (*playv1.CastSpellOutsideCombatResponse, error) {
	t.Helper()
	req := &playv1.CastSpellOutsideCombatRequest{
		CampaignId: a.campaignID, CasterId: caster.GetId(), SpellKey: spell, Slot: slot, Ritual: ritual, IdempotencyKey: newKey(),
		Roll: &playv1.CastSpellOutsideCombatRequest_RollInApp{RollInApp: true},
	}
	for _, c := range targets {
		req.TargetCharacterIds = append(req.TargetCharacterIds, c.GetId())
	}
	for _, e := range edit {
		e(req)
	}
	res, err := u.casting.CastSpellOutsideCombat(t.Context(), connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func (a *armed) mustCastOut(t *testing.T, u *user, caster *charactersv1.Character, spell string, slot *playv1.SpellSlot, ritual bool, targets []*charactersv1.Character, edit ...func(*playv1.CastSpellOutsideCombatRequest)) *playv1.CastSpellOutsideCombatResponse {
	t.Helper()
	res, err := a.castOut(t, u, caster, spell, slot, ritual, targets, edit...)
	if err != nil {
		t.Fatalf("CastSpellOutsideCombat(%s) error = %v", spell, err)
	}
	return res
}

// finish calls FinishCast as u.
func (a *armed) finish(t *testing.T, u *user, castID string, edit ...func(*playv1.FinishCastRequest)) (*playv1.FinishCastResponse, error) {
	t.Helper()
	req := &playv1.FinishCastRequest{CampaignId: a.campaignID, CastId: castID, IdempotencyKey: newKey(), Roll: &playv1.FinishCastRequest_RollInApp{RollInApp: true}}
	for _, e := range edit {
		e(req)
	}
	res, err := u.casting.FinishCast(t.Context(), connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

// casts lists the casts as u.
func (a *armed) casts(t *testing.T, u *user) *playv1.ListSpellCastsResponse {
	t.Helper()
	res, err := u.casting.ListSpellCasts(t.Context(), connect.NewRequest(&playv1.ListSpellCastsRequest{CampaignId: a.campaignID}))
	if err != nil {
		t.Fatalf("ListSpellCasts() error = %v", err)
	}
	return res.Msg
}

// wantCastingBlocked fails unless err is the failed_precondition with this reason.
func wantCastingBlocked(t *testing.T, call string, err error, reason playv1.CastingBlockedReason) *playv1.CastingBlocked {
	t.Helper()
	if err == nil {
		t.Fatalf("%s succeeded, want %v", call, reason)
	}
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("%s error = %v, want failed_precondition", call, err)
	}
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("%s error = %v, want a Connect error", call, err)
	}
	for _, d := range ce.Details() {
		v, derr := d.Value()
		if derr != nil {
			continue
		}
		if b, ok := v.(*playv1.CastingBlocked); ok {
			if b.GetReason() != reason {
				t.Fatalf("%s blocked by %v, want %v", call, b.GetReason(), reason)
			}
			return b
		}
	}
	t.Fatalf("%s error = %v, want a CastingBlocked detail (%v)", call, err, reason)
	return nil
}

// hurt sets a character's hit points as the master.
func (a *armed) hurt(t *testing.T, c *charactersv1.Character, hp int32) {
	t.Helper()
	if _, err := a.master.play.AdjustCharacterVitals(t.Context(), connect.NewRequest(&playv1.AdjustCharacterVitalsRequest{
		CampaignId: a.campaignID, CharacterId: c.GetId(), IdempotencyKey: newKey(), HitPointsCurrent: &hp,
	})); err != nil {
		t.Fatalf("AdjustCharacterVitals() error = %v", err)
	}
}

// TestCastingOutside_HealingBetweenFightsAndTheLifeDomain: Cure Wounds outside a combat
// rolls its dice at the slot's level (1d8 at the 1st, 2d8 at the 2nd) plus the
// spellcasting modifier; a cleric of the Life Domain adds Disciple of Life (2 + the
// spell's level) to the creature it heals (SRD 5.1, Cure Wounds and Cleric: Life Domain).
func TestCastingOutside_HealingBetweenFightsAndTheLifeDomain(t *testing.T) {
	t.Parallel()
	a := newCastingParty(t)
	full := a.vitals(t, a.toren).GetHitPointsMax()
	a.hurt(t, a.toren, 5)
	a.h.roller.queue(6, 3)

	// 2d8 (6 + 3) + WIS 3 = 12, and Disciple of Life at the 2nd level adds 2 + 2 = 4.
	res := a.mustCastOut(t, a.bia, a.bri, cureWounds, slotOfLevel(2), false, []*charactersv1.Character{a.toren})
	cast := res.GetCast()
	if cast.GetStatus() != playv1.OutsideCastStatus_OUTSIDE_CAST_STATUS_ENDED || cast.GetEndReason() != playv1.OutsideCastEnd_OUTSIDE_CAST_END_INSTANT || cast.GetSlotLevel() != 2 {
		t.Fatalf("cast = %v, want an instantaneous cast at the 2nd level", cast)
	}
	if cast.GetDiceCount() != 2 || cast.GetDiceSides() != 8 || len(cast.GetFaces()) != 2 || cast.GetFaces()[0] != 6 || cast.GetFaces()[1] != 3 || cast.GetRollTotal() != 12 {
		t.Errorf("roll = %dd%d %v = %d, want 2d8 [6 3] = 12", cast.GetDiceCount(), cast.GetDiceSides(), cast.GetFaces(), cast.GetRollTotal())
	}
	if got := a.vitals(t, a.toren).GetHitPointsCurrent(); got != 5+12+4 {
		t.Errorf("Toren's hit points = %d, want %d (5 + 12 + Disciple of Life 4)", got, 5+12+4)
	}
	if got := usedSlots(a.vitals(t, a.bri), 2); got != 1 {
		t.Errorf("2nd-level slots used = %d, want 1", got)
	}
	// What Toren regained is for the master and Toren's player (RN-20), not for the caster's.
	if tg := cast.GetTargets()[0]; tg.GetEffect() != playv1.CastEffect_CAST_EFFECT_HEAL || tg.GetAmount() != 0 || tg.GetHitPointsAfter() != 0 {
		t.Errorf("target as the caster's player = %v, want the heal without the numbers", tg)
	}
	for _, who := range []*user{a.master, a.caio} {
		var tg *playv1.OutsideCastTarget
		for _, c := range a.casts(t, who).GetLog() {
			tg = c.GetTargets()[0]
		}
		if tg.GetAmount() != 16 || tg.GetHitPointsBefore() != 5 || tg.GetHitPointsAfter() != 21 {
			t.Errorf("target as %s = %v, want a heal of 16 from 5 to 21", who.id, tg)
		}
	}
	// The heal never passes the maximum (SRD 5.1, "Healing").
	a.hurt(t, a.toren, full-2)
	a.h.roller.queue(8)
	a.mustCastOut(t, a.bia, a.bri, cureWounds, slotOfLevel(1), false, []*charactersv1.Character{a.toren})
	if got := a.vitals(t, a.toren).GetHitPointsCurrent(); got != full {
		t.Errorf("Toren = %d, want the maximum %d", got, full)
	}
	if n := a.eventCount(t, "spell_cast_outside"); n != 2 {
		t.Errorf("spell_cast_outside events = %d, want 2", n)
	}
}
