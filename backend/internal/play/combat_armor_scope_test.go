package play

import (
	"slices"
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// Who the armor rule reaches (SRD 5.1, "Armor"): a player's character in armor or with a shield it
// is not proficient with. An NPC or a creature with a stat block is proficient with what it wears,
// so nothing of this ever comes from a stat block. These tests need the database.

// armorNPC is an enemy the master built on a full sheet: a wizard in chain mail whose proficiencies
// do not list armor. As an NPC it is proficient with what it wears.
func (u *user) armorNPC(t *testing.T, campaignID, name string) *charactersv1.Character {
	t.Helper()
	sheet := &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
		BaseScores: &rulesv1.AbilityScores{Strength: 14, Dexterity: 14, Constitution: 12, Intelligence: 14, Wisdom: 10, Charisma: 8},
		RaceKey:    "race:human", Classes: []*charactersv1.ClassLevel{{ClassKey: "class:wizard", Level: 3}},
		WeaponKeys: []string{quarterstaffKey}, ArmorKey: "equipment:chain-mail",
		CantripKeys: []string{fireBolt}, KnownSpellKeys: []string{magicMissileSpell}, PreparedSpellKeys: []string{magicMissileSpell},
	}}}
	res, err := u.characters.CreateCharacter(t.Context(), connect.NewRequest(&charactersv1.CreateCharacterRequest{
		CampaignId: campaignID, Kind: charactersv1.CharacterKind_CHARACTER_KIND_ENEMY, Name: name, Sheet: sheet,
	}))
	if err != nil {
		t.Fatalf("CreateCharacter(%s) error = %v", name, err)
	}
	return res.Msg.GetCharacter()
}

func TestAnNPCInArmorItsSheetDoesNotListIsNeverPenalized(t *testing.T) {
	t.Parallel()
	a := newUntrainedTable(t)
	npc := a.master.armorNPC(t, a.campaignID, "Garrick")
	e := a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: npc.GetId()}}, npcRolls: []int{20},
		players: map[string]int32{"Toren": 10, "Pensantus": 10, "Brisa": 1}, reveal: []string{"Garrick"},
		at: map[string][2]int32{"Toren": {3, 3}, "Garrick": {4, 3}, "Pensantus": {3, 4}, "Brisa": {4, 4}},
	})
	id := npc.GetId()
	for _, c := range []struct {
		what, skill, ability string
		check                bool
	}{
		{"STR check", "skill:athletics", "str", true}, {"DEX check", "skill:acrobatics", "dex", true},
		{"DEX save", "", "dex", false}, {"STR save", "", "str", false},
	} {
		wantCheckMode(t, "NPC "+c.what, a.checkModeFor(t, id, c.skill, c.ability, c.check), combat.ModeNormal, nil, nil, 0, 0)
	}
	a.turnOf(t, "Garrick")
	// A weapon attack with Strength rolls with one die: with disadvantage it would ask for two.
	if _, err := a.attack(t, a.master, e, "Garrick", quarterstaffKey, "Toren", d20(12)); err != nil {
		t.Fatalf("the NPC's weapon attack error = %v, want none", err)
	}
	// And it casts.
	goal := []*playv1.SpellTarget{{CombatantId: a.id(t, "Toren"), Darts: 3}}
	if _, err := a.cast(t, a.master, e, "Garrick", magicMissileSpell, slotOfLevel(1), goal, noCastRoll); err != nil {
		t.Fatalf("the NPC's cast error = %v, want none", err)
	}
	for _, sp := range a.mustOptions(t, a.master, e, "Garrick").GetOptions().GetSpells() {
		if sp.GetReason().GetCode() == rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_ARMOR_NOT_PROFICIENT {
			t.Errorf("the NPC's spell %s is disabled by armor", sp.GetSpell().GetKey())
		}
	}
}

// The concentration save is Constitution: armor without proficiency never touches it.
func TestAConcentrationSaveInUntrainedArmorHasNoDisadvantage(t *testing.T) {
	t.Parallel()
	if got := combat.Resolve(concentrationSources(combat.Creature{ArmorUntrained: true})); got != combat.ModeNormal {
		t.Errorf("a concentration save in untrained armor = %v, want normal", got)
	}
	a := newUntrainedTable(t)
	a.untrainedFight(t)
	wantCheckMode(t, "CON save in combat", a.checkModeFor(t, a.toren.GetId(), "", "con", false), combat.ModeNormal, nil, nil, 0, 0)
}

// Strength and Dexterity checks and saves of a character, in a combat, with a contest skill or Hide.
func TestUntrainedArmorSpoilsTheContestAndHideChecksOfACharacterOnly(t *testing.T) {
	t.Parallel()
	a := newUntrainedTable(t)
	a.untrainedFight(t)
	toren, brisa := a.toren.GetId(), a.bri.GetId()
	armor := []string{"armor_untrained/disadvantage"}
	got := sourceKinds(a.checkModeFor(t, toren, "skill:acrobatics", "dex", true))
	if !slices.Contains(got, armor[0]) {
		t.Errorf("Acrobatics (a contest) sources = %v, want the armor", got)
	}
	got = sourceKinds(a.checkModeFor(t, toren, "skill:stealth", "dex", true))
	if !slices.Contains(got, armor[0]) {
		t.Errorf("Stealth (Hide) sources = %v, want the armor", got)
	}
	wantCheckMode(t, "trained Hide", a.checkModeFor(t, brisa, "skill:stealth", "dex", true), combat.ModeDisadvantage, []string{"stealth_armor/disadvantage"}, nil, 0, 0)
}
