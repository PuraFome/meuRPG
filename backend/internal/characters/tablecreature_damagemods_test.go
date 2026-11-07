package characters

import (
	"testing"

	"google.golang.org/protobuf/encoding/protojson"

	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// The damage entries of a table creature survive the protojson a stored entry and a pack are written in,
// and reach the engine's creature with their kind, types and qualifier.
func TestATableCreatureKeepsItsDamageModsThroughProtojson(t *testing.T) {
	t.Parallel()
	in := &rulesv1.TableCreature{NamePt: "Lobo", BaseKey: "monster:wolf", DamageMods: []*rulesv1.TableCreatureDamageMod{
		{Kind: "resist", DamageTypes: []string{"damage-type:piercing", "damage-type:slashing"}, Qualifier: "nonmagical_unless_silvered"},
		{Kind: "vulnerable", DamageTypes: []string{"damage-type:fire"}},
	}}
	raw, err := protojson.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	out := &rulesv1.TableCreature{}
	if err := protojson.Unmarshal(raw, out); err != nil {
		t.Fatal(err)
	}
	got := tableCreatureOf(rules.TableEntry{Key: "creature:lobo@mesa"}, out).DamageMods
	if len(got) != 2 || got[0].Kind != "resist" || got[0].Qualifier != "nonmagical_unless_silvered" || len(got[0].DamageTypes) != 2 || got[1].Kind != "vulnerable" || got[1].Qualifier != "" {
		t.Errorf("DamageMods = %+v", got)
	}
	// An entry stored before the field has none.
	old := &rulesv1.TableCreature{}
	if err := protojson.Unmarshal([]byte(`{"namePt":"Lobo","baseKey":"monster:wolf"}`), old); err != nil || len(tableCreatureOf(rules.TableEntry{}, old).DamageMods) != 0 {
		t.Errorf("an old entry: %v", err)
	}
}
