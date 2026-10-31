package characters

import (
	"testing"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// A stat block's Magic Resistance reaches the traits of a creature and of an NPC made from it.
func TestMagicResistanceReachesTheTraits(t *testing.T) {
	t.Parallel()
	srd, err := rules.LoadSRD()
	if err != nil {
		t.Fatalf("LoadSRD: %v", err)
	}
	for key, want := range map[string]bool{"monster:imp": true, "monster:goblin": false} {
		creature, ok := creatureDerived(srd, key, "full")
		if !ok {
			t.Fatalf("creatureDerived(%s) not found", key)
		}
		npc := basicDerived(srd, &charactersv1.BasicSheet{MonsterKey: key})
		if got := traitsOf(srd, creature).MagicResistance; got != want {
			t.Errorf("%s creature: MagicResistance = %v, want %v", key, got, want)
		}
		if got := traitsOf(srd, npc).MagicResistance; got != want {
			t.Errorf("%s NPC: MagicResistance = %v, want %v", key, got, want)
		}
	}
}
