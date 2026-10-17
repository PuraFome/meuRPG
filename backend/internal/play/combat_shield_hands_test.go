package play

import (
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
)

// The shield and the hands: the armor class the combat reads and compares an attack with
// leaves the shield's bonus out while a two-handed weapon is on the sheet.

const (
	chainMailKey  = "equipment:chain-mail"
	greatswordKey = "equipment:greatsword"
	longswordKey  = "equipment:longsword"
)

// shieldBearer creates Ana's character with chain mail, a shield, the choices and the
// weapons given (STR 16: chain mail is 16, the shield makes it 18).
func (a *armed) shieldBearer(t *testing.T, name string, choices, weapons []string) *charactersv1.Character {
	t.Helper()
	sheet := &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
		BaseScores: abilities(16, 13, 14, 10, 8), RaceKey: "race:human", Classes: []*charactersv1.ClassLevel{classLevel("class:fighter", 5, "")},
		ArmorKey: chainMailKey, Shield: true, WeaponKeys: weapons, FeatureChoiceKeys: choices,
	}}}
	res, err := a.ana.characters.CreateCharacter(t.Context(), connect.NewRequest(&charactersv1.CreateCharacterRequest{
		CampaignId: a.campaignID, Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Name: name, Sheet: sheet,
	}))
	if err != nil {
		t.Fatalf("CreateCharacter(%s) error = %v", name, err)
	}
	return res.Msg.GetCharacter()
}

// TestTheCombatReadsNoShieldBonusBesideATwoHandedWeapon: the master's and the owner's
// read of the armor class, which every attack on the character is compared with.
func TestTheCombatReadsNoShieldBonusBesideATwoHandedWeapon(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name    string
		weapons []string
		want    int32
	}{
		{"a one-handed weapon", []string{longswordKey}, 18},
		{"a two-handed weapon", []string{greatswordKey}, 16},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a, e := reactorFight(t, func(t *testing.T, a *armed) *charactersv1.Character {
				return a.shieldBearer(t, "Reator", nil, tt.weapons)
			})
			if got := byLabel(t, e, "Reator").GetArmorClass(); got != tt.want {
				t.Errorf("the encounter at the start reads AC %d, want %d", got, tt.want)
			}
			if got := byLabel(t, a.get(t, a.master), "Reator").GetArmorClass(); got != tt.want {
				t.Errorf("the master reads AC %d, want %d", got, tt.want)
			}
		})
	}
}
