package play

import (
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
)

// plusHero creates a fighter that carries a rapier with the given magic bonus (full.item_bonuses).
func (u *user) plusHero(t *testing.T, campaignID, name string, bonus *charactersv1.ItemBonus) *charactersv1.Character {
	t.Helper()
	full := &charactersv1.FullSheet{
		BaseScores: abilities(16, 12, 14, 10, 8), RaceKey: "race:human",
		Classes:    []*charactersv1.ClassLevel{{ClassKey: "class:fighter", Level: 2}},
		WeaponKeys: []string{rapier}, ItemBonuses: []*charactersv1.ItemBonus{bonus},
	}
	res, err := u.characters.CreateCharacter(t.Context(), connect.NewRequest(&charactersv1.CreateCharacterRequest{
		CampaignId: campaignID, Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Name: name,
		Sheet: &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: full}},
	}))
	if err != nil {
		t.Fatalf("CreateCharacter(%s) error = %v", name, err)
	}
	return res.Msg.GetCharacter()
}

// A +1 rapier reaches the combat through the same derived attack line as the sheet: its blow adds 1 to the damage
// and is magical, so a wight does not halve it (9 with the modifier, 10 with the +1, all of it); a +1 that asks for
// attunement the character lacks adds nothing, and the wight halves the plain 9.
func TestMagicBonusOfTheSheetReachesTheCombatDamage(t *testing.T) {
	t.Parallel()
	const wightHP = 45
	for name, tt := range map[string]struct {
		bonus *charactersv1.ItemBonus
		want  int32
	}{
		"plus one":            {&charactersv1.ItemBonus{Key: rapier, Bonus: 1}, wightHP - 10},
		"plus three":          {&charactersv1.ItemBonus{Key: rapier, Bonus: 3}, wightHP - 12},
		"needs attunement":    {&charactersv1.ItemBonus{Key: rapier, Bonus: 1, RequiresAttunement: true}, wightHP - 4},
		"needs it and has it": {&charactersv1.ItemBonus{Key: rapier, Bonus: 1, RequiresAttunement: true, Attuned: true}, wightHP - 10},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := hitFor(t, func(a *armed) { a.toren = a.caio.plusHero(t, a.campaignID, "Toren", tt.bonus) }, "monster:wight")
			if got != tt.want {
				t.Errorf("hit points after the blow = %d, want %d", got, tt.want)
			}
		})
	}
}
