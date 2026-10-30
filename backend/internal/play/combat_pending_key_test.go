package play

import (
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// A stat block action's key names the creature's type: a player reads the pending damage of an NPC's
// action without it, the master with it (RN-20).
func TestPendingDamageHidesACreatureActionKey(t *testing.T) {
	t.Parallel()
	npc := "npc-1"
	cs := []playdb.Combatant{{ID: npc, Kind: kindNPC}, {ID: "pc-1", Kind: kindPlayer}}
	p := playdb.PendingDamage{AttackerID: &npc, TargetID: "pc-1", AttackKey: "monster:ghoul#claws"}
	var s *Service
	for _, tt := range []struct {
		name string
		v    combatViewer
		want string
	}{
		{"the master", combatViewer{master: true}, "monster:ghoul#claws"},
		{"a player", combatViewer{}, ""},
	} {
		if got := s.pendingView(t.Context(), "camp", p, cs, tt.v).GetAttackKey(); got != tt.want {
			t.Errorf("%s reads %q, want %q", tt.name, got, tt.want)
		}
	}
}
