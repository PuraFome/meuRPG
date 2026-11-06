package play

import (
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// tableResistantKey is the key the entry below gets.
const tableResistantKey = "creature:lobo-espectral@mesa"

// tableResistantHP is the hit points of the table creature below.
const tableResistantHP = 40

// tableResistantSetup is the setup of a fight against a table creature built on the SRD wolf (it resists
// nothing) with a resistance to piercing from nonmagical weapons that aren't silvered, and a Toren whose
// rapier has the given mark.
func tableResistantSetup(t *testing.T, mark *charactersv1.WeaponMark) func(a *armed) {
	return func(a *armed) {
		res, err := a.master.table.CreateTableEntry(t.Context(), connect.NewRequest(&rulesv1.CreateTableEntryRequest{
			CampaignId: a.campaignID, Body: &rulesv1.CreateTableEntryRequest_TableCreature{TableCreature: &rulesv1.TableCreature{
				NamePt: "Lobo Espectral", BaseKey: "monster:wolf", HitPoints: tableResistantHP,
				DamageMods: []*rulesv1.TableCreatureDamageMod{{
					Kind: "resist", DamageTypes: []string{"damage-type:piercing"}, Qualifier: "nonmagical_unless_silvered",
				}},
			}},
		}))
		if err != nil {
			t.Fatalf("CreateTableEntry(creature) error = %v", err)
		}
		if got := res.Msg.GetEntry().GetKey(); got != tableResistantKey {
			t.Fatalf("the creature's key = %q, want %q", got, tableResistantKey)
		}
		markedToren(t, mark)(a)
	}
}

// A table creature's resistance with the silvered qualifier reaches combat as an SRD stat block's does: a plain
// rapier loses half (9 becomes 4), a silvered one all 9, and a magical one all 9.
func TestTableCreatureSilveredResistanceHalvesAPlainBlowOnly(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		mark *charactersv1.WeaponMark
		want int32
	}{
		{"plain", nil, tableResistantHP - 4},
		{"silvered", &charactersv1.WeaponMark{WeaponKey: rapier, Silvered: true}, tableResistantHP - 9},
		{"adamantine only", &charactersv1.WeaponMark{WeaponKey: rapier, Adamantine: true}, tableResistantHP - 4},
		{"magical", &charactersv1.WeaponMark{WeaponKey: rapier, Magical: true}, tableResistantHP - 9},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := hitFor(t, tableResistantSetup(t, tt.mark), tableResistantKey); got != tt.want {
				t.Errorf("hit points after the blow = %d, want %d", got, tt.want)
			}
		})
	}
}
