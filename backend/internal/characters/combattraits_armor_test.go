package characters

import (
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// Only a player's character can be spoiled by armor it is not proficient with: an NPC, a creature
// and a druid in a beast form are never.
func TestArmorTrainingOfIsForAPlayersCharacterOnly(t *testing.T) {
	t.Parallel()
	d := rules.Derived{ArmorUntrained: rules.ArmorUntrained{Armor: true, Shield: true}}
	wolf, none := "monster:wolf", ""
	for _, tt := range []struct {
		name  string
		kind  string
		beast *string
		want  bool
	}{
		{"a player", kindPlayer, nil, true},
		{"a player, no beast named", kindPlayer, &none, true},
		{"a player in a beast form", kindPlayer, &wolf, false},
		{"an enemy", "enemy", nil, false},
		{"a boss", "boss", nil, false},
		{"a story NPC", "story", nil, false},
	} {
		if got := armorTrainingOf(d, tt.kind, tt.beast).Any(); got != tt.want {
			t.Errorf("%s: untrained = %v, want %v", tt.name, got, tt.want)
		}
	}
}
