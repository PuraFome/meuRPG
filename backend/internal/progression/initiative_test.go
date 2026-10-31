package progression

import (
	"slices"
	"testing"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/dice"
	"github.com/PuraFome/meuRPG/backend/internal/platform/dice/dicetest"
)

// The app rolls each copy's initiative itself, and a total that equals another's makes them take one joint turn (MR-013).

// npcTotals starts a combat of eight copies of a creature on a table that rolls with roller and returns the copies' totals.
func npcTotals(t *testing.T, roller dice.Roller) []int32 {
	t.Helper()
	tb := newTableRolling(t, campaignsv1.XpMode_XP_MODE_ENEMIES, 4, roller)
	goblin := tb.master.minion(t, tb.campaign, "Goblin", 0)
	var totals []int32
	for _, c := range tb.fight(t, goblin, 8, 0, false).GetCombatants() {
		if c.GetKind() == playv1.CombatantKind_COMBATANT_KIND_NPC {
			totals = append(totals, c.GetInitiative())
		}
	}
	if len(totals) != 8 {
		t.Fatalf("the combat has %d NPCs, want the 8 copies", len(totals))
	}
	return totals
}

// TestEqualRollsTieEightCopiesIntoOneJointTurn proves a tie is real: dice that roll the same face give eight copies one total.
func TestEqualRollsTieEightCopiesIntoOneJointTurn(t *testing.T) {
	t.Parallel()
	totals := npcTotals(t, &dice.Fixed{Faces: slices.Repeat([]int{12}, 8)})
	if got := slices.Compact(slices.Clone(totals)); len(got) != 1 {
		t.Errorf("totals = %v, want all eight equal (one joint turn)", totals)
	}
}

// TestEightCopiesOfACreatureNeverTieOnInitiative proves the harness's default dice keep a test from meeting that tie.
func TestEightCopiesOfACreatureNeverTieOnInitiative(t *testing.T) {
	t.Parallel()
	totals := npcTotals(t, dicetest.New())
	sorted := slices.Sorted(slices.Values(totals))
	if got := slices.Compact(slices.Clone(sorted)); len(got) != 8 {
		t.Errorf("totals = %v: copies with the same total would take one joint turn", totals)
	}
}
