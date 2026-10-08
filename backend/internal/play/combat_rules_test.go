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
		cb("low", new(int32(9)), 0, 1),
		cb("tieB", new(int32(12)), 0, 2),
		cb("tieA", new(int32(12)), 0, 3),
		cb("highBonus", new(int32(12)), 3, 4),
		cb("top", new(int32(19)), 1, 5),
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
	cs := []playdb.Combatant{cb("a", new(int32(3)), 0, 0), cb("b", new(int32(2)), 0, 1), cb("c", new(int32(1)), 0, 2)}
	cs[1].Defeated = true
	next := func(current, skip string) (string, bool, bool) {
		ids, newRound, ok := nextTurnGroup(cs, current, skip)
		if len(ids) != 1 && ok {
			t.Fatalf("nextTurnGroup(%q, %q) = %v, want a group of one", current, skip, ids)
		}
		if !ok {
			return "", newRound, ok
		}
		return ids[0], newRound, ok
	}

	if next, newRound, ok := next("a", ""); next != "c" || newRound || !ok {
		t.Errorf("after a = %q, newRound %v, ok %v; want c, false, true (b is defeated)", next, newRound, ok)
	}
	if next, newRound, ok := next("c", ""); next != "a" || !newRound || !ok {
		t.Errorf("after c = %q, newRound %v, ok %v; want a, true, true", next, newRound, ok)
	}
	// The one that leaves does not count; the turn goes on in the same round.
	if next, newRound, _ := next("a", "a"); next != "c" || newRound {
		t.Errorf("removing a: next %q, newRound %v; want c, false", next, newRound)
	}
	// Alone: its own turn again, in a new round.
	cs[2].Defeated = true
	if next, newRound, ok := next("a", ""); next != "a" || !newRound || !ok {
		t.Errorf("alone = %q, newRound %v, ok %v; want a, true, true", next, newRound, ok)
	}
	cs[0].Defeated = true
	if _, _, ok := next("a", ""); ok {
		t.Error("nextTurnGroup() with everybody defeated: ok = true")
	}
	// A current that is not in the list (it was removed): from the first one.
	cs[0].Defeated, cs[1].Defeated, cs[2].Defeated = false, false, false
	if next, newRound, _ := next("gone", ""); next != "a" || newRound {
		t.Errorf("after a missing current = %q, newRound %v; want a, false", next, newRound)
	}
}

// TestMR013_GroupsAreTheRunsWithTheSameTotal: adjacent combatants with the
// same total are one group, whatever their bonus or kind; one alone on its
// total is a group of one; the next turn goes to the next group, and its
// defeated members do not take it.
func TestMR013_GroupsAreTheRunsWithTheSameTotal(t *testing.T) {
	t.Parallel()
	cs := []playdb.Combatant{
		cb("brisa", new(int32(19)), 3, 0), cb("toren", new(int32(19)), 1, 1), cb("capitao", new(int32(16)), 0, 2),
		cb("g1", new(int32(12)), 0, 3), cb("g2", new(int32(12)), 0, 4), cb("g3", new(int32(9)), 0, 5),
	}
	var shape []int
	for _, g := range groupRuns(cs) {
		shape = append(shape, len(g))
	}
	if !slices.Equal(shape, []int{2, 1, 2, 1}) {
		t.Fatalf("groups = %v, want 2 1 2 1", shape)
	}
	next, newRound, ok := nextTurnGroup(cs, "toren", "")
	if !ok || newRound || !slices.Equal(next, []string{"capitao"}) {
		t.Errorf("after toren = %v, %v, %v; want capitao in the same round", next, newRound, ok)
	}
	cs[3].Defeated = true
	next, _, _ = nextTurnGroup(cs, "capitao", "")
	if !slices.Equal(next, []string{"g2"}) {
		t.Errorf("after capitao = %v, want only g2 (g1 is defeated)", next)
	}
	next, newRound, _ = nextTurnGroup(cs, "g3", "")
	if !slices.Equal(next, []string{"brisa", "toren"}) || !newRound {
		t.Errorf("after g3 = %v, %v; want brisa and toren in a new round", next, newRound)
	}
	// Without an initiative, a combatant is a group of its own.
	cs[1].Initiative, cs[0].Initiative = nil, nil
	if n := len(groupRuns(cs[:2])); n != 2 {
		t.Errorf("two combatants without an initiative make %d groups, want 2", n)
	}
}

func TestRN21_MovementLeftIsKeptInTenthsOfAFoot(t *testing.T) {
	t.Parallel()
	c := playdb.Combatant{SpeedFt: 25}
	if got := movementLeftDFt(c); got != 250 {
		t.Errorf("movementLeftDFt() at the start = %d, want 250", got)
	}
	// A diagonal step is 7,1 ft: three of them leave 25 - 21,3 = 3,7 ft.
	c.MovementUsedDft = 3 * 71
	if got, ft := movementLeftDFt(c), movementLeftFt(c); got != 37 || ft != 3 {
		t.Errorf("after three diagonal steps = %d dft, %d ft; want 37 dft and 3 ft (rounded down)", got, ft)
	}
	c.Dashed = true
	if got := movementLeftDFt(c); got != 287 {
		t.Errorf("movementLeftDFt() after the Dash = %d, want 287", got)
	}
	c.Dashed, c.MovementUsedDft = false, 1000
	if got := movementLeftDFt(c); got != 0 {
		t.Errorf("movementLeftDFt() overspent = %d, want 0", got)
	}
	// A creature with a fly speed moves with the better of its speeds.
	c = playdb.Combatant{SpeedFt: 10, SpeedFlyFt: 50}
	if got := speedDFt(c); got != 500 {
		t.Errorf("speedDFt() of a flier = %d, want 500", got)
	}
}

// TestRN21_AFlierIsWhoMovesOnItsFlySpeed: a creature that walks faster than it
// flies moves by walking, so difficult terrain costs it.
func TestRN21_AFlierIsWhoMovesOnItsFlySpeed(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		walk, fly int32
		want      bool
	}{
		{"flies faster than it walks", 10, 50, true},
		{"flies as fast as it walks", 30, 30, true},
		{"walks faster than it flies", 60, 10, false},
		{"cannot fly", 30, 0, false},
	} {
		if got := moverOf(playdb.Combatant{SpeedFt: tc.walk, SpeedFlyFt: tc.fly}).Flier; got != tc.want {
			t.Errorf("%s: moverOf(walk %d, fly %d).Flier = %v, want %v", tc.name, tc.walk, tc.fly, got, tc.want)
		}
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
	// A name that only begins with the other's ("Goblin Boss") is not a copy of it.
	if got := copyLabels("Goblin", 1, map[string]bool{"Goblin Boss": true}); !slices.Equal(got, []string{"Goblin"}) {
		t.Errorf("one Goblin beside a Goblin Boss = %v, want [Goblin]", got)
	}
	if got := copyLabels("Goblin", 1, map[string]bool{"Goblin 1": true, "Goblin 2": true}); !slices.Equal(got, []string{"Goblin 3"}) {
		t.Errorf("one Goblin beside numbered ones = %v, want [Goblin 3]", got)
	}
	long := "Um nome de monstro absurdamente comprido que passa do limite"
	for _, l := range copyLabels(long, 10, map[string]bool{}) {
		if n := len([]rune(l)); n > maxLabelLength {
			t.Errorf("label %q has %d characters, want at most %d", l, n, maxLabelLength)
		}
	}
}
