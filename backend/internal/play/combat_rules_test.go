package play

import (
	"slices"
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// The combat's pure rules (combat_rules.go): no database.

func cb(id string, initiative *int32, bonus, order int32) playdb.Combatant {
	return playdb.Combatant{ID: id, Initiative: initiative, InitiativeBonus: bonus, OrderIndex: order}
}

func i32p(n int32) *int32 { return &n }

func ids(cs []playdb.Combatant) []string {
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, c.ID)
	}
	return out
}

func TestRN19_OrderByInitiativeThenBonusThenTheMastersPlaces(t *testing.T) {
	t.Parallel()
	cs := []playdb.Combatant{
		cb("none", nil, 0, 0),
		cb("low", i32p(9), 0, 1),
		cb("tieB", i32p(12), 0, 2),
		cb("tieA", i32p(12), 0, 3),
		cb("highBonus", i32p(12), 3, 4),
		cb("top", i32p(19), 1, 5),
	}
	got := ids(orderCombatants(cs))
	want := []string{"top", "highBonus", "tieB", "tieA", "low", "none"}
	if !slices.Equal(got, want) {
		t.Fatalf("orderCombatants() = %v, want %v", got, want)
	}

	// The master puts tieA before tieB; they keep the places they hold.
	ordered := orderCombatants(cs)
	swapped := withOrder(ordered, []string{"tieA", "tieB"})
	if got := ids(swapped); !slices.Equal(got, []string{"top", "highBonus", "tieA", "tieB", "low", "none"}) {
		t.Fatalf("withOrder() = %v", got)
	}
	// Renumbered as saveOrder does, the new order is stable under a re-sort.
	for i := range swapped {
		swapped[i].OrderIndex = int32(i)
	}
	if got := ids(orderCombatants(swapped)); !slices.Equal(got, ids(swapped)) {
		t.Errorf("re-sorting the decided order changed it: %v", got)
	}

	ties := unresolvedTies(cs)
	if !ties["tieA"] || !ties["tieB"] || ties["highBonus"] || ties["top"] {
		t.Errorf("unresolvedTies() = %v, want only tieA and tieB", ties)
	}
	for i := range swapped {
		swapped[i].TieOrdered = true
	}
	if ties := unresolvedTies(swapped); len(ties) != 0 {
		t.Errorf("unresolvedTies() after ordering = %v, want none", ties)
	}
}

func TestMR013_NextTurnSkipsTheDefeatedAndCountsRounds(t *testing.T) {
	t.Parallel()
	cs := []playdb.Combatant{cb("a", i32p(3), 0, 0), cb("b", i32p(2), 0, 1), cb("c", i32p(1), 0, 2)}
	cs[1].Defeated = true

	if next, newRound, ok := nextTurn(cs, "a", ""); next != "c" || newRound || !ok {
		t.Errorf("after a = %q, newRound %v, ok %v; want c, false, true (b is defeated)", next, newRound, ok)
	}
	if next, newRound, ok := nextTurn(cs, "c", ""); next != "a" || !newRound || !ok {
		t.Errorf("after c = %q, newRound %v, ok %v; want a, true, true", next, newRound, ok)
	}
	// The one that leaves does not count; the turn goes on in the same round.
	if next, newRound, _ := nextTurn(cs, "a", "a"); next != "c" || newRound {
		t.Errorf("removing a: next %q, newRound %v; want c, false", next, newRound)
	}
	// Alone: its own turn again, in a new round.
	cs[2].Defeated = true
	if next, newRound, ok := nextTurn(cs, "a", ""); next != "a" || !newRound || !ok {
		t.Errorf("alone = %q, newRound %v, ok %v; want a, true, true", next, newRound, ok)
	}
	cs[0].Defeated = true
	if _, _, ok := nextTurn(cs, "a", ""); ok {
		t.Error("nextTurn() with everybody defeated: ok = true")
	}
	// A current that is not in the list (it was removed): from the first one.
	cs[0].Defeated, cs[1].Defeated, cs[2].Defeated = false, false, false
	if next, newRound, _ := nextTurn(cs, "gone", ""); next != "a" || newRound {
		t.Errorf("after a missing current = %q, newRound %v; want a, false", next, newRound)
	}
}

func TestRN21_MovementLeftAndCostPerSquare(t *testing.T) {
	t.Parallel()
	col, row := int32(2), int32(2)
	c := playdb.Combatant{SpeedFt: 25, GridCol: &col, GridRow: &row}
	if got := moveCostFt(c, 5, 2); got != 15 {
		t.Errorf("moveCostFt(3 squares across) = %d, want 15", got)
	}
	if got := moveCostFt(c, 5, 5); got != 15 { // diagonals cost one square each
		t.Errorf("moveCostFt(3 squares diagonally) = %d, want 15", got)
	}
	c.MovementUsedFt = 15
	if got := movementLeftFt(c); got != 10 {
		t.Errorf("movementLeftFt() = %d, want 10", got)
	}
	c.Dashed = true
	if got := movementLeftFt(c); got != 35 {
		t.Errorf("movementLeftFt() after the Dash = %d, want 35", got)
	}
	c.MovementUsedFt = 100
	if got := movementLeftFt(c); got != 0 {
		t.Errorf("movementLeftFt() overspent = %d, want 0", got)
	}
}

func TestRN20_StateWords(t *testing.T) {
	t.Parallel()
	npc := func(hp, hpMax int32, defeated bool) playdb.Combatant {
		return playdb.Combatant{Kind: kindNPC, HpCurrent: &hp, HpMax: &hpMax, Defeated: defeated}
	}
	for _, tt := range []struct {
		name string
		c    playdb.Combatant
		want playv1.CombatantState
	}{
		{"full", npc(7, 7, false), playv1.CombatantState_COMBATANT_STATE_UNHURT},
		{"hurt", npc(4, 7, false), playv1.CombatantState_COMBATANT_STATE_HURT},
		{"half", npc(5, 10, false), playv1.CombatantState_COMBATANT_STATE_BADLY_HURT},
		{"one hit point", npc(1, 27, false), playv1.CombatantState_COMBATANT_STATE_BADLY_HURT},
		{"zero", npc(0, 7, false), playv1.CombatantState_COMBATANT_STATE_DEFEATED},
		{"defeated flag", npc(3, 7, true), playv1.CombatantState_COMBATANT_STATE_DEFEATED},
		{"a player's character has none", playdb.Combatant{Kind: kindPlayer}, playv1.CombatantState_COMBATANT_STATE_UNSPECIFIED},
	} {
		if got := stateOf(tt.c); got != tt.want {
			t.Errorf("%s: stateOf() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestMR013_GridSquares(t *testing.T) {
	t.Parallel()
	g := link.Grid{Columns: 20, Rows: 10}
	for _, tt := range []struct{ x, y, col, row int32 }{
		{0, 0, 0, 0}, {499, 999, 0, 0}, {500, 1000, 1, 1}, {5000, 5000, 10, 5}, {10000, 10000, 19, 9}, {9999, 5000, 19, 5},
	} {
		if col, row := squareOf(g, tt.x, tt.y); col != tt.col || row != tt.row {
			t.Errorf("squareOf(%d, %d) = %d, %d; want %d, %d", tt.x, tt.y, col, row, tt.col, tt.row)
		}
	}
	// The middle of a square falls back in the same square.
	for col := range int32(20) {
		for row := range int32(10) {
			x, y := centerOf(g, col, row)
			if c, r := squareOf(g, x, y); c != col || r != row {
				t.Fatalf("centerOf(%d, %d) = %d, %d, which is square %d, %d", col, row, x, y, c, r)
			}
		}
	}
}

func TestRN19_CopyLabels(t *testing.T) {
	t.Parallel()
	taken := map[string]bool{}
	if got := copyLabels("Goblin", 1, taken); !slices.Equal(got, []string{"Goblin"}) {
		t.Errorf("one copy = %v, want [Goblin]", got)
	}
	if got := copyLabels("Goblin", 2, taken); !slices.Equal(got, []string{"Goblin 2", "Goblin 3"}) {
		t.Errorf("reinforcements = %v, want [Goblin 2 Goblin 3]", got)
	}
	if got := copyLabels("Orc", 3, map[string]bool{}); !slices.Equal(got, []string{"Orc 1", "Orc 2", "Orc 3"}) {
		t.Errorf("three copies = %v", got)
	}
	long := "Um nome de monstro absurdamente comprido que passa do limite"
	for _, l := range copyLabels(long, 10, map[string]bool{}) {
		if n := len([]rune(l)); n > maxLabelLength {
			t.Errorf("label %q has %d characters, want at most %d", l, n, maxLabelLength)
		}
	}
}
