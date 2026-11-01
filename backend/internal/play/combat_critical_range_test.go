package play

import (
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// The critical range of the table's critical_range modifier, end to end (#573): a weapon attack is a critical
// hit from the lowest natural d20 the character's effects give (the lowest of the SRD's 20, Improved
// Critical's 19 and the table's), and a spell attack keeps the 20 (combat_attack_stroke.go criticalFrom). These
// tests need the database (MEURPG_TEST_DATABASE_URL).

// heroWithFeats is heroWith for a fighter that has feats.
func (u *user) heroWithFeats(t *testing.T, campaignID, name string, cl *charactersv1.ClassLevel, weapon string, feats ...string) *charactersv1.Character {
	t.Helper()
	sheet := &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
		BaseScores: abilities(16, 13, 14, 10, 8), RaceKey: "race:human", Classes: []*charactersv1.ClassLevel{cl},
		WeaponKeys: []string{weapon}, FeatKeys: feats,
	}}}
	res, err := u.characters.CreateCharacter(t.Context(), connect.NewRequest(&charactersv1.CreateCharacterRequest{
		CampaignId: campaignID, Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Name: name, Sheet: sheet,
	}))
	if err != nil {
		t.Fatalf("CreateCharacter(%s) error = %v", name, err)
	}
	return res.Msg.GetCharacter()
}

func TestTableCriticalRangeWidensTheCriticalRangeOfAWeaponAttack(t *testing.T) {
	t.Parallel()
	a := newArmedWith(t, func(a *armed) {
		res, err := a.master.table.CreateTableEntry(t.Context(), connect.NewRequest(&rulesv1.CreateTableEntryRequest{
			CampaignId: a.campaignID, Body: &rulesv1.CreateTableEntryRequest_TableFeat{TableFeat: &rulesv1.TableFeat{
				NamePt: "Olho Afiado", DescPt: []string{"Um talento da mesa."},
				Effects: []*rulesv1.TableEffect{{Type: "modifier", Target: "critical_range", Mode: "set", Value: "18"}},
			}},
		}))
		if err != nil {
			t.Fatalf("CreateTableEntry() error = %v", err)
		}
		feat := res.Msg.GetEntry().GetKey()
		// Toren: table 18 and Improved Critical 19. Pensantus: Improved Critical alone. Brisa: the table's 18 alone.
		a.toren = a.caio.heroWithFeats(t, a.campaignID, "Toren", classLevel("class:fighter", 3, "subclass:champion"), battleaxe, feat)
		a.pens = a.ana.heroWithFeats(t, a.campaignID, "Pensantus", classLevel("class:fighter", 3, "subclass:champion"), battleaxe)
		a.bri = a.bia.heroWithFeats(t, a.campaignID, "Brisa", classLevel("class:fighter", 2, ""), battleaxe, feat)
	})
	e := a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: a.goblin.GetId()}}, npcRolls: []int{3}, reveal: []string{"Goblin"},
		players: map[string]int32{"Toren": 18, "Pensantus": 10, "Brisa": 1},
		at:      map[string][2]int32{"Toren": {3, 3}, "Goblin": {4, 3}, "Pensantus": {5, 3}, "Brisa": {4, 4}},
	})
	for _, tc := range []struct {
		who  string
		user *user
		// face to the outcome, in the order the rounds go
		faces []int32
		want  []playv1.AttackOutcome
	}{
		{"Toren", a.caio, []int32{17, 18}, []playv1.AttackOutcome{hit, crit}},
		{"Pensantus", a.ana, []int32{18, 19}, []playv1.AttackOutcome{hit, crit}},
		{"Brisa", a.bia, []int32{17, 18}, []playv1.AttackOutcome{hit, crit}},
	} {
		for i, face := range tc.faces {
			e = a.turnOf(t, tc.who)
			res := a.mustAttack(t, tc.user, e, tc.who, battleaxe, "Goblin", d20(face))
			if got := res.GetRoll().GetOutcome(); got != tc.want[i] {
				t.Errorf("%s: natural %d is %v, want %v", tc.who, face, got, tc.want[i])
			}
		}
	}
}
