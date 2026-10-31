package progression

import (
	"slices"
	"testing"

	"connectrpc.com/connect"

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

// scriptD20 makes the next d20s the app rolls come out as faces, on a table with the default dice.
func (tb *table) scriptD20(faces ...int) {
	tb.h.t.Helper()
	roller, ok := tb.h.roller.(*dicetest.Roller)
	if !ok {
		tb.h.t.Fatalf("the table rolls with %T, which cannot be scripted", tb.h.roller)
	}
	roller.ScriptFor(tb.h.t, 20, faces...)
}

// requireOwnTotals fails when two combatants of the combat, one of them among ids, have the same initiative total.
// The rolls are deterministic, so a tie here is a tie on every run: script the faces with ScriptFor (scriptD20).
func (tb *table) requireOwnTotals(t *testing.T, encounterID string, ids ...string) {
	t.Helper()
	res, err := tb.master.combat.GetEncounter(t.Context(), connect.NewRequest(&playv1.GetEncounterRequest{CampaignId: tb.campaign}))
	if err != nil {
		t.Fatalf("GetEncounter() error = %v", err)
	}
	if res.Msg.GetEncounter().GetId() != encounterID {
		t.Fatalf("the open combat is %s, want %s", res.Msg.GetEncounter().GetId(), encounterID)
	}
	for _, own := range res.Msg.GetEncounter().GetCombatants() {
		if !slices.Contains(ids, own.GetId()) || own.Initiative == nil {
			continue
		}
		for _, c := range res.Msg.GetEncounter().GetCombatants() {
			if c.GetId() != own.GetId() && c.Initiative != nil && c.GetInitiative() == own.GetInitiative() {
				t.Fatalf("%s and %s tie on initiative %d and would take one joint turn: script the faces with ScriptFor (scriptD20)", own.GetLabel(), c.GetLabel(), own.GetInitiative())
			}
		}
	}
}
