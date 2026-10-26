package play

import (
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// onTurnOf passes the turns, as the master, until the combatant is on turn (setup, not play).
func onTurnOf(label string) func(t *testing.T, c *cave) {
	return func(t *testing.T, c *cave) {
		t.Helper()
		c.passTo(t, c.get(t, c.master), label)
	}
}

// The master dragging an NPC on its own turn walks the way a player's move does: the cost of the line, the
// rubble, the walls and the creatures in the way, the NPC's movement spent, and a drag past what is left
// refused (the master decides to force it). Off its turn, forced, and for a player's character the master's
// move stays free (TestMoveCombatantFrozen_TheMaster). The cave: Goblin 1 has 30 ft (300 dft).
func TestMoveCombatantFrozen_TheMasterDraggingAnNPCOnItsTurn(t *testing.T) {
	t.Parallel()
	farEast := standAt(map[string][2]int32{"Goblin 1": {22, 7}})
	runCaveMoves(t, []caveMove{
		{
			name: "two squares spend two squares", setup: onTurnOf("Goblin 1"), as: "master", label: "Goblin 1", col: 16, row: 5,
			want: moveWant{col: 16, row: 5, usedDft: 100, lastDft: 100, event: `{"col": 16, "combatant_id": "<Goblin 1>", "cost_dft": 100, "cost_ft": 10, "distance_dft": 100, "distance_ft": 10, "from": {"col": 18, "cover": "none", "placed": true, "row": 5}, "on_turn": true, "round": 1, "row": 5, "spent_dft": 100}`},
		},
		{
			name: "a diagonal costs the exact line, as a player's does", setup: onTurnOf("Goblin 1"), as: "master", label: "Goblin 1", col: 16, row: 3,
			want: moveWant{col: 16, row: 3, usedDft: 141, lastDft: 141, event: `{"col": 16, "combatant_id": "<Goblin 1>", "cost_dft": 141, "cost_ft": 14, "distance_dft": 141, "distance_ft": 14, "from": {"col": 18, "cover": "none", "placed": true, "row": 5}, "on_turn": true, "round": 1, "row": 3, "spent_dft": 141}`},
		},
		{
			name: "through rubble the path costs more than its length", setup: then(standAt(map[string][2]int32{"Goblin 1": {3, 9}}), onTurnOf("Goblin 1")), as: "master", label: "Goblin 1", col: 6, row: 9,
			want: moveWant{col: 6, row: 9, usedDft: 250, lastDft: 150, event: `{"col": 6, "combatant_id": "<Goblin 1>", "cost_dft": 250, "cost_ft": 25, "distance_dft": 150, "distance_ft": 15, "from": {"col": 3, "cover": "none", "placed": true, "row": 9}, "on_turn": true, "round": 1, "row": 9, "spent_dft": 250}`},
		},
		{
			name: "past the movement left is refused, so the master decides", setup: then(farEast, onTurnOf("Goblin 1")), as: "master", label: "Goblin 1", col: 8, row: 7,
			refused: new(blockedBy("TOO_FAR", "that is beyond your movement", `"missingFt":40`, `"missingDft":400`)),
		},
		{
			name: "past the movement left, forced, is free", setup: then(farEast, onTurnOf("Goblin 1")), as: "master", label: "Goblin 1", col: 8, row: 7,
			edit: []func(*playv1.MoveCombatantRequest){asForced},
			want: moveWant{col: 8, row: 7, usedDft: 0, lastDft: 0, event: `{"col": 8, "combatant_id": "<Goblin 1>", "distance_dft": 700, "distance_ft": 70, "from": {"col": 22, "cover": "none", "placed": true, "row": 7}, "on_turn": true, "round": 1, "row": 7}`},
		},
		{
			name: "onto a creature", setup: then(standAt(map[string][2]int32{"Goblin 2": {19, 5}}), onTurnOf("Goblin 1")), as: "master", label: "Goblin 1", col: 19, row: 5,
			refused: new(blockedBy("SQUARE_OCCUPIED", "another combatant is on that square")),
		},
	})
}

// paysItsWay is the whole rule of which of the master's drags walk like a player's.
func TestPaysItsWay(t *testing.T) {
	t.Parallel()
	one, two := int32(1), int32(2)
	user := "u1"
	npc := playdb.Combatant{GridCol: &one, GridRow: &two}
	player := playdb.Combatant{GridCol: &one, GridRow: &two, UserID: &user}
	offMap := playdb.Combatant{}
	cases := []struct {
		name   string
		target playdb.Combatant
		master bool
		onTurn bool
		req    moveRequest
		want   bool
	}{
		{name: "an NPC on its turn", target: npc, master: true, onTurn: true, want: true},
		{name: "an NPC off its turn", target: npc, master: true, onTurn: false, want: false},
		{name: "a player's character on its turn", target: player, master: true, onTurn: true, want: false},
		{name: "forced", target: npc, master: true, onTurn: true, req: moveRequest{forced: true}, want: false},
		{name: "a long jump", target: npc, master: true, onTurn: true, req: moveRequest{jump: playv1.JumpKind_JUMP_KIND_LONG}, want: false},
		{name: "a placement", target: npc, master: true, onTurn: true, req: moveRequest{place: true}, want: false},
		{name: "an NPC off the map", target: offMap, master: true, onTurn: true, want: false},
		{name: "not the master", target: npc, master: false, onTurn: true, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := moveAttempt{req: tc.req, target: tc.target, onTurn: tc.onTurn, v: combatViewer{master: tc.master}}
			if got := a.paysItsWay(); got != tc.want {
				t.Errorf("paysItsWay() = %v, want %v", got, tc.want)
			}
		})
	}
}
