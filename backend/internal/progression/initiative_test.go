package progression

import (
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// TestEightCopiesOfACreatureNeverTieOnInitiative: the app rolls each copy's
// initiative itself, and a total that equals another's makes them one joint
// turn (MR-013). A test that starts a combat must never meet that tie it did
// not ask for, so the harness hands the service dice that give eight copies
// of a creature eight different totals.
func TestEightCopiesOfACreatureNeverTieOnInitiative(t *testing.T) {
	t.Parallel()
	tb := newTable(t, enemies, 4)
	goblin := tb.master.minion(t, tb.campaign, "Goblin", 0)
	enc := tb.fight(t, goblin, 8, 0, false)
	seen := map[int32]string{}
	copies := 0
	for _, c := range enc.GetCombatants() {
		if c.GetKind() != playv1.CombatantKind_COMBATANT_KIND_NPC {
			continue
		}
		copies++
		if other, tied := seen[c.GetInitiative()]; tied {
			t.Errorf("%s and %s both have initiative %d: they would take one joint turn", other, c.GetLabel(), c.GetInitiative())
		}
		seen[c.GetInitiative()] = c.GetLabel()
	}
	if copies != 8 {
		t.Fatalf("the combat has %d NPCs, want the 8 copies", copies)
	}
}
