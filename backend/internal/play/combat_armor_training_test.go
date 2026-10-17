package play

import (
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// Armor or a shield the character is not proficient with (SRD 5.1, "Armor"): disadvantage on
// every ability check, saving throw and attack roll that uses Strength or Dexterity, and no
// spell can be cast. These tests need the database (MEURPG_TEST_DATABASE_URL).

const quarterstaffKey = "equipment:quarterstaff"

// armorHero creates a player's character wearing armor and, maybe, a shield.
func (u *user) armorHero(t *testing.T, campaignID, name, class string, level int32, armor string, shield bool) *charactersv1.Character {
	t.Helper()
	sheet := &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
		BaseScores: &rulesv1.AbilityScores{Strength: 14, Dexterity: 14, Constitution: 12, Intelligence: 14, Wisdom: 10, Charisma: 8},
		RaceKey:    "race:human", Classes: []*charactersv1.ClassLevel{{ClassKey: class, Level: level}},
		WeaponKeys: []string{quarterstaffKey, shortbowKey}, ArmorKey: armor, Shield: shield,
		CantripKeys: []string{fireBolt}, KnownSpellKeys: []string{magicMissileSpell}, PreparedSpellKeys: []string{magicMissileSpell},
	}}}
	res, err := u.characters.CreateCharacter(t.Context(), connect.NewRequest(&charactersv1.CreateCharacterRequest{
		CampaignId: campaignID, Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Name: name, Sheet: sheet,
	}))
	if err != nil {
		t.Fatalf("CreateCharacter(%s) error = %v", name, err)
	}
	return res.Msg.GetCharacter()
}

// newUntrainedTable is the party: Toren, a wizard in chain mail (no proficiency in armor);
// Pensantus, a wizard with a shield alone; Brisa, a fighter in chain mail with a shield (trained).
func newUntrainedTable(t *testing.T) *armed {
	t.Helper()
	return newArmedWith(t, func(a *armed) {
		a.toren = a.caio.armorHero(t, a.campaignID, "Toren", "class:wizard", 3, "equipment:chain-mail", false)
		a.pens = a.ana.armorHero(t, a.campaignID, "Pensantus", "class:wizard", 3, "", true)
		a.bri = a.bia.armorHero(t, a.campaignID, "Brisa", "class:fighter", 3, "equipment:chain-mail", true)
	})
}

// untrainedFight starts the combat with the goblin next to Toren.
func (a *armed) untrainedFight(t *testing.T) *playv1.Encounter {
	t.Helper()
	return a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: a.goblin.GetId()}}, npcRolls: []int{3},
		players: map[string]int32{"Toren": 18, "Pensantus": 10, "Brisa": 1}, reveal: []string{"Goblin"},
		at: map[string][2]int32{"Toren": {3, 3}, "Goblin": {4, 3}, "Pensantus": {3, 4}, "Brisa": {4, 4}},
	})
}

// wantArmorSource checks the target's mode is disadvantage and one of its sources is the armor.
func wantArmorSource(t *testing.T, what string, tg *playv1.TargetInReach, want bool) {
	t.Helper()
	if tg == nil {
		t.Fatalf("%s: no such target", what)
	}
	has := false
	for _, s := range tg.GetSources() {
		has = has || s.GetKind() == playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_ARMOR_NOT_PROFICIENT
	}
	if has != want {
		t.Errorf("%s: sources %v, armor source = %v, want %v", what, tg.GetSources(), has, want)
	}
	if want && tg.GetRollMode() != playv1.RollMode_ROLL_MODE_DISADVANTAGE {
		t.Errorf("%s: mode = %v, want disadvantage", what, tg.GetRollMode())
	}
}

func TestUntrainedArmorGivesDisadvantageOnStrengthAndDexterityAttacks(t *testing.T) {
	t.Parallel()
	a := newUntrainedTable(t)
	e := a.untrainedFight(t)
	toren := a.mustOptions(t, a.caio, e, "Toren")
	wantArmorSource(t, "Strength melee attack", targetOf(toren, quarterstaffKey, "Goblin"), true)
	wantArmorSource(t, "Dexterity ranged attack", targetOf(toren, shortbowKey, "Goblin"), true)
	// A spell attack uses the casting ability, and the spell is refused anyway.
	if tg := targetOf(toren, fireBolt, "Goblin"); tg != nil {
		wantArmorSource(t, "spell attack", tg, false)
	}
	// The roll itself takes two dice: a single typed d20 is not the mode the server asks for.
	if _, err := a.attack(t, a.caio, e, "Toren", quarterstaffKey, "Goblin", d20(12)); connect.CodeOf(err) != connect.CodeAborted {
		t.Fatalf("RollAttack() with one die error = %v, want aborted (disadvantage asks for two)", err)
	}
	res := a.mustAttack(t, a.caio, e, "Toren", quarterstaffKey, "Goblin", disadvantage(12))
	if res.GetRoll().GetMode() != playv1.RollMode_ROLL_MODE_DISADVANTAGE {
		t.Errorf("the roll mode = %v, want disadvantage", res.GetRoll().GetMode())
	}
}

func TestATrainedWearerAttacksNormally(t *testing.T) {
	t.Parallel()
	a := newUntrainedTable(t)
	e := a.untrainedFight(t)
	brisa := a.mustOptions(t, a.bia, e, "Brisa")
	wantArmorSource(t, "fighter in chain mail with a shield", targetOf(brisa, quarterstaffKey, "Goblin"), false)
	if tg := targetOf(brisa, quarterstaffKey, "Goblin"); tg.GetRollMode() != playv1.RollMode_ROLL_MODE_NORMAL {
		t.Errorf("mode = %v, want normal", tg.GetRollMode())
	}
}

func TestAShieldAloneBreaksTheProficiency(t *testing.T) {
	t.Parallel()
	a := newUntrainedTable(t)
	e := a.untrainedFight(t)
	pens := a.mustOptions(t, a.ana, e, "Pensantus")
	wantArmorSource(t, "wizard with a shield", targetOf(pens, quarterstaffKey, "Goblin"), true)
	_, err := a.cast(t, a.ana, e, "Pensantus", magicMissileSpell, slotOfLevel(1), []*playv1.SpellTarget{{CombatantId: a.id(t, "Goblin")}}, noCastRoll)
	wantEncounterBlocked(t, err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_ARMOR_NOT_PROFICIENT)
}

func TestUntrainedArmorGivesDisadvantageOnStrengthAndDexterityChecksAndSaves(t *testing.T) {
	t.Parallel()
	a := newUntrainedTable(t)
	toren, brisa := a.toren.GetId(), a.bri.GetId()
	armor := []string{"armor_not_proficient/disadvantage"}
	// Out of a combat.
	wantCheckMode(t, "STR check", a.checkModeFor(t, toren, "skill:athletics", "str", true), combat.ModeDisadvantage, armor, nil, 0, 0)
	wantCheckMode(t, "DEX check", a.checkModeFor(t, toren, "skill:acrobatics", "dex", true), combat.ModeDisadvantage, armor, nil, 0, 0)
	wantCheckMode(t, "DEX save", a.checkModeFor(t, toren, "", "dex", false), combat.ModeDisadvantage, armor, nil, 0, 0)
	wantCheckMode(t, "STR save", a.checkModeFor(t, toren, "", "str", false), combat.ModeDisadvantage, armor, nil, 0, 0)
	wantCheckMode(t, "WIS save", a.checkModeFor(t, toren, "", "wis", false), combat.ModeNormal, nil, nil, 0, 0)
	wantCheckMode(t, "WIS check", a.checkModeFor(t, toren, "skill:perception", "wis", true), combat.ModeNormal, nil, nil, 0, 0)
	wantCheckMode(t, "CON save", a.checkModeFor(t, toren, "", "con", false), combat.ModeNormal, nil, nil, 0, 0)
	wantCheckMode(t, "trained STR check", a.checkModeFor(t, brisa, "skill:athletics", "str", true), combat.ModeNormal, nil, nil, 0, 0)
	// In a combat.
	a.untrainedFight(t)
	wantCheckMode(t, "STR check in combat", a.checkModeFor(t, toren, "skill:athletics", "str", true), combat.ModeDisadvantage, armor, nil, 0, 0)
	wantCheckMode(t, "DEX save in combat", a.checkModeFor(t, toren, "", "dex", false), combat.ModeDisadvantage, armor, nil, 0, 0)
	wantCheckMode(t, "WIS save in combat", a.checkModeFor(t, toren, "", "wis", false), combat.ModeNormal, nil, nil, 0, 0)
	wantCheckMode(t, "trained DEX save in combat", a.checkModeFor(t, brisa, "", "dex", false), combat.ModeNormal, nil, nil, 0, 0)
}

func TestUntrainedArmorRefusesEverySpellInACombat(t *testing.T) {
	t.Parallel()
	a := newUntrainedTable(t)
	e := a.untrainedFight(t)
	goblin := []*playv1.SpellTarget{{CombatantId: a.id(t, "Goblin")}}
	_, err := a.cast(t, a.caio, e, "Toren", magicMissileSpell, slotOfLevel(1), goblin, noCastRoll)
	wantEncounterBlocked(t, err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_ARMOR_NOT_PROFICIENT)
	// A cantrip with an attack roll is a spell too.
	_, err = a.attack(t, a.caio, e, "Toren", fireBolt, "Goblin", inAppRoll)
	wantEncounterBlocked(t, err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_ARMOR_NOT_PROFICIENT)
	// The turn options say so.
	o := a.mustOptions(t, a.caio, e, "Toren")
	for _, sp := range o.GetOptions().GetSpells() {
		if sp.GetEnabled() || sp.GetReason().GetCode() != rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_ARMOR_NOT_PROFICIENT {
			t.Errorf("spell %s = enabled %v, reason %v; want disabled by the armor", sp.GetSpell().GetKey(), sp.GetEnabled(), sp.GetReason())
		}
	}
	for _, at := range o.GetOptions().GetAttacks() {
		if at.GetAttack().GetKind() == rulesv1.AttackKind_ATTACK_KIND_SPELL && (at.GetEnabled() || at.GetReason().GetCode() != rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_ARMOR_NOT_PROFICIENT) {
			t.Errorf("cantrip %s = enabled %v, reason %v; want disabled by the armor", at.GetAttack().GetKey(), at.GetEnabled(), at.GetReason())
		}
	}
}

func TestUntrainedArmorRefusesASpellOutsideACombat(t *testing.T) {
	t.Parallel()
	a := newUntrainedTable(t)
	_, err := a.castOut(t, a.caio, a.toren, magicMissileSpell, slotOfLevel(1), false, nil)
	wantCastingBlocked(t, "CastSpellOutsideCombat(armor)", err, playv1.CastingBlockedReason_CASTING_BLOCKED_REASON_ARMOR_NOT_PROFICIENT)
	o, err := a.casting(t, a.caio, a.toren)
	if err != nil {
		t.Fatalf("GetCastOptions() error = %v", err)
	}
	if !o.GetArmorNotProficient() {
		t.Error("GetCastOptions().armor_not_proficient = false, want true")
	}
	if o, err := a.casting(t, a.bia, a.bri); err != nil || o.GetArmorNotProficient() {
		t.Errorf("trained wearer: armor_not_proficient = %v, error = %v; want false", o.GetArmorNotProficient(), err)
	}
}
