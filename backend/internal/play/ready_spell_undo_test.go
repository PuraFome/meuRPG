package play

import (
	"slices"
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The undo of the release of a held spell (the CastSpell of its readier's open window) gives back everything the release did
// and nothing else: the area it laid, its concentration as the held spell had it, the reaction, the row waiting with its window
// open and its wait clean, and the slots as they were (the release spent none, and the Ready's stay spent).
func TestTheUndoOfAReleasedHeldSpellHoldsItAgain(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name    string
		spell   string
		slot    int32
		cave    func(t *testing.T) *cave
		fight   func(c *cave, t *testing.T) *playv1.Encounter
		release func(c *cave, t *testing.T)
		laid    func(c *cave, t *testing.T) int // what the release laid on the map: zones, or creatures asleep
	}{
		{"Web", webKey, 2, zoneCave, (*cave).zoneFight,
			func(c *cave, t *testing.T) { c.mustCastArea(t, c.ana, "Pensantus", webKey, nil, at(9, 7), nil) },
			func(c *cave, t *testing.T) int { return len(zonesOf(c.get(t, c.master), webKey)) }},
		{"Wall of Fire", wallOfFireKey, 4, wallCave, (*cave).zoneFight,
			func(c *cave, t *testing.T) {
				c.h.roller.queue(5, 5, 5, 5, 5, 5)
				c.mustCastArea(t, c.ana, "Pensantus", wallOfFireKey, nil, at(7, 8), nil, northWall(playv1.WallSide_WALL_SIDE_LEFT))
			},
			func(c *cave, t *testing.T) int { return len(zonesOf(c.get(t, c.master), wallOfFireKey)) }},
		{"Sleep", sleepSpell, 1, newAreaCave, (*cave).areaFight,
			func(c *cave, t *testing.T) {
				c.mustCastArea(t, c.ana, "Pensantus", sleepSpell, nil, at(20, 7), nil, poolSum(20))
			},
			func(c *cave, t *testing.T) int {
				n := 0
				for _, cb := range c.get(t, c.master).GetCombatants() {
					if slices.Contains(cb.GetConditions(), "condition:unconscious") {
						n++
					}
				}
				return n
			}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := tt.cave(t)
			e := tt.fight(c, t)
			if tt.name != "Sleep" {
				e = c.passTo(t, e, "Pensantus")
			}
			c.mustReady(t, c.ana, c.readySpell(t, e, "Pensantus", tt.spell, tt.slot, ""))
			e = c.mustEndTurn(t, c.master, e)
			w := c.mustFire(t, e, "Pensantus")
			if err := c.markRolling(t, c.ana, e, w.GetId()); err != nil { // the wait is on when the release comes
				t.Fatalf("the mark: %v", err)
			}
			held := byLabel(t, c.get(t, c.master), "Pensantus").GetConcentrationSpell()
			levels := []int32{1, 2, 3, 4, 5}
			slots, laid := c.slotsOf(t, "Pensantus", levels...), tt.laid(c, t)
			tt.release(c, t)
			if c.readiedStatus(t, "Pensantus") != readiedFired {
				t.Fatalf("the release left the row %q, want fired", c.readiedStatus(t, "Pensantus"))
			}
			if got := tt.laid(c, t); got <= laid { // the check can fail: the release did lay something
				t.Fatalf("the release laid nothing (%d, was %d)", got, laid)
			}
			c.mustUndo(t, e)
			pens := byLabel(t, c.get(t, c.master), "Pensantus")
			win := c.windowOf(t, c.master, kindReadied)
			if got := tt.laid(c, t); got != laid {
				t.Errorf("after the undo %d of what the release laid is left, want %d", got, laid)
			}
			if pens.GetConcentrationSpell() != held || pens.GetReactionUsed() || c.readiedStatus(t, "Pensantus") != readiedWaiting || !slices.Equal(c.slotsOf(t, "Pensantus", levels...), slots) {
				t.Errorf("after the undo: concentration %q (want %q), reaction used %v, row %q, slots %v (want %v); want the held spell as it was",
					pens.GetConcentrationSpell(), held, pens.GetReactionUsed(), c.readiedStatus(t, "Pensantus"), c.slotsOf(t, "Pensantus", levels...), slots)
			}
			if win == nil || win.GetReactorId() != pens.GetId() || win.GetRolling() || win.GetWaitBeganAt() != nil || win.GetWaitedMs() != 0 {
				t.Errorf("after the undo the READIED window reads %v; want it open with a clean wait", win)
			}
		})
	}
}
