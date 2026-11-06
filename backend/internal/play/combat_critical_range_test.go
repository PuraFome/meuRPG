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
// Critical's 19 and the table's), and a spell attack keeps the 20 unless the entry's scope (applies) names spells (combat_attack_stroke.go
// criticalFrom). All tests need the database (MEURPG_TEST_DATABASE_URL).

// giveFeat is the master adding a feat to a sheet, which only he may do.
func (a *armed) giveFeat(t *testing.T, c *charactersv1.Character, feat string) {
	t.Helper()
	cur := a.master.character(t, c)
	sheet := cur.GetSheet()
	sheet.GetFull().FeatKeys = append(sheet.GetFull().GetFeatKeys(), feat)
	if _, err := a.master.characters.UpdateCharacter(t.Context(), connect.NewRequest(&charactersv1.UpdateCharacterRequest{
		CampaignId: a.campaignID, CharacterId: cur.GetId(), Revision: cur.GetRevision(), Name: cur.GetName(), Sheet: sheet,
	})); err != nil {
		t.Fatalf("UpdateCharacter(%s gets %s) error = %v", cur.GetName(), feat, err)
	}
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
		a.toren = a.caio.heroWith(t, a.campaignID, "Toren", classLevel("class:fighter", 3, "subclass:champion"), abilities(16, 13, 14, 10, 8), []string{battleaxe}, nil, nil)
		a.pens = a.ana.heroWith(t, a.campaignID, "Pensantus", classLevel("class:fighter", 3, "subclass:champion"), abilities(16, 13, 14, 10, 8), []string{battleaxe}, nil, nil)
		a.bri = a.bia.heroWith(t, a.campaignID, "Brisa", classLevel("class:fighter", 2, ""), abilities(16, 13, 14, 10, 8), []string{battleaxe}, nil, nil)
		a.giveFeat(t, a.toren, feat)
		a.giveFeat(t, a.bri, feat)
	})
	a.start(t, plan{
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
			e := a.turnOf(t, tc.who)
			res := a.mustAttack(t, tc.user, e, tc.who, battleaxe, "Goblin", d20(face))
			if got := res.GetRoll().GetOutcome(); got != tc.want[i] {
				t.Errorf("%s: natural %d is %v, want %v", tc.who, face, got, tc.want[i])
			}
		}
	}
}

// An entry without a scope is for weapon attacks: the same feat that makes Toren's weapon attack critical on an 18
// does nothing for a spell attack (combat_attack_stroke.go criticalFrom).
func TestTableCriticalRangeDoesNotWidenASpellAttack(t *testing.T) {
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
		a.pens = a.ana.hero(t, a.campaignID, "Pensantus", "class:wizard", "race:gnome", 1, abilities(10, 14, 12, 16, 8), nil, []string{fireBolt})
		a.giveFeat(t, a.pens, res.Msg.GetEntry().GetKey())
	})
	a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: a.capitao.GetId()}}, npcRolls: []int{3}, reveal: []string{"Capitão Goblin"},
		players: map[string]int32{"Pensantus": 18},
		at:      map[string][2]int32{"Pensantus": {3, 3}, "Capitão Goblin": {9, 3}},
	})
	for i, tc := range []struct {
		face int32
		want playv1.AttackOutcome
	}{{18, hit}, {19, hit}, {20, crit}} {
		e := a.turnOf(t, "Pensantus")
		res := a.mustAttack(t, a.ana, e, "Pensantus", fireBolt, "Capitão Goblin", d20(tc.face))
		if got := res.GetRoll().GetOutcome(); got != tc.want {
			t.Errorf("round %d: a spell attack on a natural %d is %v, want %v", i, tc.face, got, tc.want)
		}
	}
}

// The scope of a critical_range entry (applies): spell reaches the spell attack only, weapon (the default) the weapon
// attack only, both the two. Each case is a wizard with the entry, whose spell attack is rolled as an attack, a cleric whose
// spell attack is cast, and a fighter with it, whose weapon attack is rolled; the table's 19 makes the natural 19 critical where the
// scope says so and a plain hit elsewhere. No entry at all is a plain hit on the 19 for the spell.
func TestTableCriticalRangeScopeReachesSpellAttacks(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, applies string
		spell, weapon playv1.AttackOutcome
	}{
		{"spell", "spell", crit, hit},
		{"weapon", "weapon", hit, crit},
		{"default", "", hit, crit},
		{"both", "both", crit, crit},
		{"none", "none", hit, hit},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := newArmedWith(t, func(a *armed) {
				var feats []string
				if tc.applies != "none" {
					res, err := a.master.table.CreateTableEntry(t.Context(), connect.NewRequest(&rulesv1.CreateTableEntryRequest{
						CampaignId: a.campaignID, Body: &rulesv1.CreateTableEntryRequest_TableFeat{TableFeat: &rulesv1.TableFeat{
							NamePt: "Olho Afiado", DescPt: []string{"Um talento da mesa."},
							Effects: []*rulesv1.TableEffect{{Type: "modifier", Target: "critical_range", Mode: "set", Value: "19", Applies: tc.applies}},
						}},
					}))
					if err != nil {
						t.Fatalf("CreateTableEntry() error = %v", err)
					}
					feats = append(feats, res.Msg.GetEntry().GetKey())
				}
				a.pens = a.ana.hero(t, a.campaignID, "Pensantus", "class:wizard", "race:gnome", 1, abilities(10, 14, 12, 16, 8), nil, []string{fireBolt})
				a.toren = a.caio.heroWith(t, a.campaignID, "Toren", classLevel("class:fighter", 1, ""), abilities(16, 13, 14, 10, 8), []string{battleaxe}, nil, nil)
				a.bri = a.bia.caster(t, a.campaignID, "Brisa", "class:cleric", "race:human", 3,
					&rulesv1.AbilityScores{Strength: 10, Dexterity: 16, Constitution: 14, Intelligence: 10, Wisdom: 16, Charisma: 8}, []string{maceKey}, nil, nil,
					[]string{guidingBolt})
				for _, f := range feats {
					a.giveFeat(t, a.bri, f)
					a.giveFeat(t, a.pens, f)
					a.giveFeat(t, a.toren, f)
				}
			})
			a.start(t, plan{
				npcs: []*playv1.Participant{{CharacterId: a.capitao.GetId()}}, npcRolls: []int{3}, reveal: []string{"Capitão Goblin"},
				players: map[string]int32{"Pensantus": 18, "Toren": 10, "Brisa": 5},
				at:      map[string][2]int32{"Brisa": {3, 5}, "Pensantus": {3, 3}, "Capitão Goblin": {9, 3}, "Toren": {10, 3}},
			})
			e := a.turnOf(t, "Pensantus")
			if got := a.mustAttack(t, a.ana, e, "Pensantus", fireBolt, "Capitão Goblin", d20(19)).GetRoll().GetOutcome(); got != tc.spell {
				t.Errorf("spell attack rolled on a 19 is %v, want %v", got, tc.spell)
			}
			e = a.turnOf(t, "Brisa")
			res, err := a.cast(t, a.bia, e, "Brisa", guidingBolt, slotOfLevel(1), a.at(t, "Capitão Goblin"), castD20(19))
			if err != nil {
				t.Fatalf("CastSpell() error = %v", err)
			}
			if got := res.GetCast().GetTargets()[0].GetOutcome(); got != tc.spell {
				t.Errorf("spell attack cast on a 19 is %v, want %v", got, tc.spell)
			}
			e = a.turnOf(t, "Toren")
			if got := a.mustAttack(t, a.caio, e, "Toren", battleaxe, "Capitão Goblin", d20(19)).GetRoll().GetOutcome(); got != tc.weapon {
				t.Errorf("weapon attack on a 19 is %v, want %v", got, tc.weapon)
			}
		})
	}
}
