package play

import (
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// RN-10, MR-031: an NPC on the stage hidden from the players is, for the casts outside a combat, an NPC
// that is not on the stage: its casts are secret, and a player cannot aim at it, not even by its place
// on the stage. Shown, it is the table's again.
func TestRN10_AHiddenStageNPCCastsInSecretAndIsNoTargetForPlayers(t *testing.T) {
	t.Parallel()
	a := newCastingParty(t)
	sheet := &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
		BaseScores: &rulesv1.AbilityScores{Strength: 10, Dexterity: 12, Constitution: 12, Intelligence: 16, Wisdom: 10, Charisma: 10}, RaceKey: "race:human",
		Classes: []*charactersv1.ClassLevel{classLevel("class:wizard", 5, "")}, KnownSpellKeys: []string{mageArmorKey, magicMissileSpell}, PreparedSpellKeys: []string{mageArmorKey, magicMissileSpell},
	}}}
	res, err := a.master.characters.CreateCharacter(t.Context(), connect.NewRequest(&charactersv1.CreateCharacterRequest{
		CampaignId: a.campaignID, Kind: charactersv1.CharacterKind_CHARACTER_KIND_ENEMY, Name: "Zqvxorth-Kwjl-7731", Sheet: sheet,
	}))
	if err != nil {
		t.Fatalf("CreateCharacter(NPC) error = %v", err)
	}
	mage := res.Msg.GetCharacter()
	a.onStage(t, mage)
	setHidden := func(hidden bool) {
		t.Helper()
		if _, err := a.h.pool.Exec(t.Context(), `UPDATE stage_npcs SET hidden = $2 WHERE character_id = $1`, mage.GetId(), hidden); err != nil {
			t.Fatalf("set the NPC hidden = %v: %v", hidden, err)
		}
	}
	var place string
	if err := a.h.pool.QueryRow(t.Context(), `SELECT id FROM stage_npcs WHERE character_id = $1`, mage.GetId()).Scan(&place); err != nil {
		t.Fatal(err)
	}

	setHidden(true)
	cast := a.mustCastOut(t, a.master, mage, mageArmorKey, slotOfLevel(1), false, []*charactersv1.Character{a.toren}).GetCast()
	if !cast.GetSecret() {
		t.Errorf("cast by a hidden stage NPC = %v, want a secret one", cast)
	}
	if c := a.castByID(t, a.master, cast.GetId()); c == nil || c.GetCasterName() != "Zqvxorth-Kwjl-7731" {
		t.Errorf("the master's read of the cast = %v (the positive control)", c)
	}
	if c := a.castByID(t, a.ana, cast.GetId()); c != nil {
		t.Errorf("a player reads %v of the hidden NPC's cast", c)
	}
	_, err = a.castOut(t, a.ana, a.pens, magicMissileSpell, slotOfLevel(1), false, nil, func(r *playv1.CastSpellOutsideCombatRequest) { r.TargetIds = []string{place} })
	wantCode(t, "aiming at a hidden stage NPC by its place", err, connect.CodeNotFound)
	if shown := a.casts(t, a.ana); len(shown.GetActive())+len(shown.GetLog()) != 0 {
		t.Errorf("Ana reads %v, want no cast", shown)
	}

	setHidden(false)
	public := a.mustCastOut(t, a.master, mage, magicMissileSpell, slotOfLevel(1), false, []*charactersv1.Character{a.toren}).GetCast()
	if public.GetSecret() {
		t.Errorf("cast by the NPC once shown = %v, want a public one", public)
	}
	aimed, err := a.castOut(t, a.ana, a.pens, magicMissileSpell, slotOfLevel(1), false, nil, func(r *playv1.CastSpellOutsideCombatRequest) { r.TargetIds = []string{place} })
	if err != nil {
		t.Fatalf("aiming at the NPC once shown error = %v", err)
	}
	if c := a.castByID(t, a.ana, public.GetId()); c == nil || c.GetCasterName() != "Zqvxorth-Kwjl-7731" {
		t.Errorf("Ana reads %v of the cast by the NPC in plain sight, want it", c)
	}

	// Hidden again, the casts it made in plain sight are gone from the players' list, and so is its name as a target.
	setHidden(true)
	if c := a.castByID(t, a.ana, public.GetId()); c != nil {
		t.Errorf("Ana reads %v of a cast by the NPC hidden again, want none", c)
	}
	own := a.castByID(t, a.ana, aimed.GetCast().GetId())
	if own == nil || len(own.GetTargets()) != 0 {
		t.Errorf("Ana's own cast aimed at the hidden NPC = %v, want her cast without the NPC among its targets", own)
	}
	if c := a.castByID(t, a.master, public.GetId()); c == nil {
		t.Error("the master no longer reads the cast by the hidden NPC")
	}
	setHidden(false)
	if c := a.castByID(t, a.ana, public.GetId()); c == nil {
		t.Error("Ana does not read the cast again once the NPC is shown")
	}
}
