package play

import (
	"strings"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// A druid in a beast form keeps the power attacks of the druid's features that the beast's attack line fits. The wolf's Bite is a
// natural attack line of the stat block: a weapon line, melee, with no weapon properties, and one the beast is always
// proficient with (rules.monsterAttack). So a trade that asks only for a melee weapon is listed on the Bite and accepted,
// a trade that also asks for proficiency fits it too, and a trade that names a weapon property (heavy) never does.

const (
	beastMeleeName   = "Golpe Livre"
	beastTrainedName = "Golpe Treinado"
	beastHeavyName   = "Golpe Pesado"
)

// newBeastPowerTable is a table with a Druid 5 of a subclass that has the three trades; heavyKey is the key of the heavy one.
func newBeastPowerTable(t *testing.T) (a *armed, heavyKey string) {
	t.Helper()
	trade := func(name string, e *rulesv1.TableEffect) *rulesv1.TableFeature {
		e.Type, e.AttackPenalty, e.DamageBonus, e.Weapon = "power_attack", 2, 4, "melee"
		return &rulesv1.TableFeature{NamePt: name, DescPt: []string{"Troca precisão por dano."}, Effects: []*rulesv1.TableEffect{e}}
	}
	a = newArmedWith(t, func(a *armed) {
		res, err := a.master.table.CreateTableEntry(t.Context(), connect.NewRequest(&rulesv1.CreateTableEntryRequest{
			CampaignId: a.campaignID, Body: &rulesv1.CreateTableEntryRequest_TableSubclass{TableSubclass: &rulesv1.TableSubclass{
				NamePt: "Círculo da Presa", ClassKey: "class:druid", DescPt: []string{"Golpes de fera."},
				Levels: []*rulesv1.TableSubclassLevel{{Level: 2, Features: []*rulesv1.TableFeature{
					trade(beastMeleeName, &rulesv1.TableEffect{}),
					trade(beastTrainedName, &rulesv1.TableEffect{OnlyProficient: true}),
					trade(beastHeavyName, &rulesv1.TableEffect{WeaponProperties: []string{"weapon-property:heavy"}}),
				}}},
			}},
		}))
		if err != nil {
			t.Fatalf("CreateTableEntry() error = %v", err)
		}
		features := res.Msg.GetEntry().GetTableSubclass().GetLevels()[0].GetFeatures()
		heavyKey = features[2].GetKey()
		a.bri = a.bia.heroWith(t, a.campaignID, "Sálvia", classLevel("class:druid", 5, res.Msg.GetEntry().GetKey()),
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 12, Constitution: 14, Intelligence: 10, Wisdom: 16, Charisma: 8}, nil, nil, nil)
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 2,
			&rulesv1.AbilityScores{Strength: 16, Dexterity: 13, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}, []string{battleaxe}, nil)
		a.pens = a.ana.hero(t, a.campaignID, "Pensantus", "class:wizard", "race:gnome", 1,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 14, Constitution: 12, Intelligence: 16, Wisdom: 10, Charisma: 8}, nil, []string{fireBolt})
	})
	return a, heavyKey
}

func TestABeastFormKeepsThePowerAttacksItsBiteFits(t *testing.T) {
	t.Parallel()
	a, heavyKey := newBeastPowerTable(t)
	a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: a.goblin.GetId()}}, npcRolls: []int{1},
		players: map[string]int32{"Sálvia": 20, "Toren": 10, "Pensantus": 5},
		reveal:  []string{"Goblin"},
		at:      map[string][2]int32{"Sálvia": {6, 5}, "Goblin": {7, 5}, "Toren": {2, 2}, "Pensantus": {12, 2}},
	})
	a.mustAssume(t, a.bia, a.bri, wolfKey)
	e := a.turnOf(t, "Sálvia")

	bite := attackOption(a.mustOptions(t, a.bia, e, "Sálvia"), wolfBite)
	if bite == nil {
		t.Fatal("the wolf's Bite is not among the attack options")
	}
	keys := map[string]string{}
	for _, p := range bite.GetPowerAttacks() {
		keys[p.GetName()] = p.GetKey()
	}
	if keys[beastMeleeName] == "" || keys[beastTrainedName] == "" || keys[beastHeavyName] != "" || len(keys) != 2 {
		t.Fatalf("trades on the Bite = %v, want %q and %q (the Bite is a proficient melee line) and not %q (it has no heavy property)",
			keys, beastMeleeName, beastTrainedName, beastHeavyName)
	}

	// The listed trades are accepted, with the penalty off the roll.
	swing := func(key string) (*playv1.RollAttackResponse, error) {
		return a.attack(t, a.bia, e, "Sálvia", wolfBite, "Goblin", func(r *playv1.RollAttackRequest) {
			d20(15)(r)
			r.PowerAttackKey = key
		})
	}
	plain, err := swing("")
	if err != nil {
		t.Fatalf("RollAttack(Bite) error = %v", err)
	}
	a.undoLast(t, e)
	for _, name := range []string{beastMeleeName, beastTrainedName} {
		res, err := swing(keys[name])
		if err != nil {
			t.Fatalf("RollAttack(Bite, %s) error = %v", name, err)
		}
		if got, want := res.GetRoll().GetD20().GetModifier(), plain.GetRoll().GetD20().GetModifier()-2; got != want {
			t.Errorf("modifier of the Bite with %s = %d, want %d (the Bite's less the penalty of 2)", name, got, want)
		}
		a.undoLast(t, e)
	}

	// The heavy trade is not listed, and naming its key anyway is refused for the property.
	_, err = swing(heavyKey)
	if connect.CodeOf(err) != connect.CodeInvalidArgument || !strings.Contains(err.Error(), "propriedade") {
		t.Errorf("RollAttack(Bite, heavy trade) error = %v, want invalid_argument for the missing property", err)
	}
}
