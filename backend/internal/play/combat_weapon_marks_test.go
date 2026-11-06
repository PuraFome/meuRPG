package play

import (
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// markedHero creates a fighter that carries a rapier with the given mark (full.weapon_marks).
func (u *user) markedHero(t *testing.T, campaignID, name string, mark *charactersv1.WeaponMark) *charactersv1.Character {
	t.Helper()
	full := &charactersv1.FullSheet{
		BaseScores: abilities(16, 12, 14, 10, 8), RaceKey: "race:human",
		Classes:    []*charactersv1.ClassLevel{{ClassKey: "class:fighter", Level: 2}},
		WeaponKeys: []string{rapier},
	}
	if mark != nil {
		full.WeaponMarks = []*charactersv1.WeaponMark{mark}
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

// hitFor rolls a rapier hit with 6 piercing (9 with the modifier) on the target and returns
// its hit points afterwards.
func hitFor(t *testing.T, mark *charactersv1.WeaponMark, monster string) int32 {
	t.Helper()
	a, e, target := partsFight(t, func(a *armed) {
		a.toren = a.caio.markedHero(t, a.campaignID, "Toren", mark)
	}, monster)
	hit := a.mustAttack(t, a.caio, e, "Toren", rapier, target, d20(15))
	a.mustDamage(t, a.caio, e, hit.GetPendingDamage().GetId(), typedDamage(6))
	hp, _, _ := a.hp(t, target)
	return hp
}

// The sheet's mark on a weapon reaches the damage: a silvered rapier is not halved by a wight
// (it loses the whole 9), an unmarked one is (it loses 4), an adamantine mark
// does not help against the silver exemption, and a magical mark beats the plain resistance of
// a grick.
func TestWeaponMarkOfTheSheetDecidesTheNonmagicalResistance(t *testing.T) {
	t.Parallel()
	const wightHP, grickHP = 45, 27
	tests := []struct {
		name    string
		monster string
		mark    *charactersv1.WeaponMark
		want    int32
	}{
		{"wight, unmarked", "monster:wight", nil, wightHP - 4},
		{"wight, silvered", "monster:wight", &charactersv1.WeaponMark{WeaponKey: rapier, Silvered: true}, wightHP - 9},
		{"wight, adamantine only", "monster:wight", &charactersv1.WeaponMark{WeaponKey: rapier, Adamantine: true}, wightHP - 4},
		{"wight, magical", "monster:wight", &charactersv1.WeaponMark{WeaponKey: rapier, Magical: true}, wightHP - 9},
		{"grick, unmarked", "monster:grick", nil, grickHP - 4},
		{"grick, silvered", "monster:grick", &charactersv1.WeaponMark{WeaponKey: rapier, Silvered: true}, grickHP - 4},
		{"grick, magical", "monster:grick", &charactersv1.WeaponMark{WeaponKey: rapier, Magical: true}, grickHP - 9},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			start := int32(grickHP)
			if tt.monster == "monster:wight" {
				start = wightHP
			}
			if got := hitFor(t, tt.mark, tt.monster); got != tt.want {
				t.Errorf("hit points after the blow = %d (from %d), want %d", got, start, tt.want)
			}
		})
	}
}

// A weapon_rule of an active table feature that marks the weapons silvered reaches the damage like the
// sheet's own mark does: the rapier is not halved by a wight (it loses the whole 9, not 4).
func TestWeaponRuleMarkMakesTheRapierSilveredAgainstAWight(t *testing.T) {
	t.Parallel()
	const wightHP = 45
	hitWith := func(t *testing.T, rule *rulesv1.TableEffect) int32 {
		t.Helper()
		a, e, target := partsFight(t, func(a *armed) {
			effects := []*rulesv1.TableEffect{}
			if rule != nil {
				effects = append(effects, rule)
			}
			res, err := a.master.table.CreateTableEntry(t.Context(), connect.NewRequest(&rulesv1.CreateTableEntryRequest{
				CampaignId: a.campaignID, Body: &rulesv1.CreateTableEntryRequest_TableSubclass{TableSubclass: &rulesv1.TableSubclass{
					NamePt: "Lâmina de Prata de Teste", ClassKey: "class:fighter", DescPt: []string{"Armas prateadas."},
					Levels: []*rulesv1.TableSubclassLevel{{Level: 3, Features: []*rulesv1.TableFeature{{NamePt: "Prata Viva", DescPt: []string{"Prata."}, Effects: effects}}}},
				}},
			}))
			if err != nil {
				t.Fatalf("CreateTableEntry() error = %v", err)
			}
			a.toren = a.caio.heroWith(t, a.campaignID, "Toren", classLevel("class:fighter", 5, res.Msg.GetEntry().GetKey()),
				abilities(16, 12, 14, 10, 8), []string{rapier}, nil, nil)
		}, "monster:wight")
		hit := a.mustAttack(t, a.caio, e, "Toren", rapier, target, d20(15))
		a.mustDamage(t, a.caio, e, hit.GetPendingDamage().GetId(), typedDamage(6))
		hp, _, _ := a.hp(t, target)
		return hp
	}
	if got := hitWith(t, &rulesv1.TableEffect{Type: "weapon_rule", Weapon: "any", AttackBonus: 1}); got != wightHP-4 {
		t.Errorf("a rule that marks nothing: hit points = %d, want %d (halved)", got, wightHP-4)
	}
	if got := hitWith(t, &rulesv1.TableEffect{Type: "weapon_rule", Weapon: "any", Silvered: true}); got != wightHP-9 {
		t.Errorf("a silvering rule: hit points = %d, want %d (not halved)", got, wightHP-9)
	}
}
