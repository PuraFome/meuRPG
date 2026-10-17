package play

import (
	"context"
	"sync"
	"testing"

	maplink "github.com/PuraFome/meuRPG/backend/internal/maps/link"
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

// dragFogRecorder is a fog source that records the moves the combat tells it.
type dragFogRecorder struct {
	mu    sync.Mutex
	moved [][2]grid.Square
}

func (*dragFogRecorder) CombatSight(context.Context, string, string) (maplink.CombatSight, error) {
	return nil, nil
}

func (r *dragFogRecorder) CombatMoved(_ context.Context, _, _ string, _ bool, from, to grid.Square) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.moved = append(r.moved, [2]grid.Square{from, to})
}

func (r *dragFogRecorder) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.moved = nil
}

func (r *dragFogRecorder) told(from, to grid.Square) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, m := range r.moved {
		if m[0] == from && m[1] == to {
			return true
		}
	}
	return false
}

// TestUndoingADragPutsTheDraggedCreatureBackWithItsCoverMarkAndTheFogFollows: the master's cover mark on
// the dragged creature clears with the drag and comes back with the undo, and the fog is told the creature
// went back to the square it left.
func TestUndoingADragPutsTheDraggedCreatureBackWithItsCoverMarkAndTheFogFollows(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	c := dragFight(t, a, arenaPlan{})
	fog := &dragFogRecorder{}
	a.h.svc.SetFog(fog)
	a.execSQL(t, `UPDATE combatants SET cover_mark = 'half' WHERE id = $1`, c.id(t, c.hob))

	if _, err := c.moveTo(t, a.caio, "Toren", 3, 0); err != nil {
		t.Fatalf("a three-square drag: %v", err)
	}
	if got := a.moveRowOf(t, c.hob); got.CoverMark != "none" || (got.Col != 3 || got.Row != 1) {
		t.Fatalf("the dragged creature after the drag = %+v, want (3,1) with the mark cleared", got)
	}
	fog.reset()

	a.undoLast(t, c.refresh(t))
	got := a.moveRowOf(t, c.hob)
	if got.Col != 4 || got.Row != 3 || got.CoverMark != "half" {
		t.Errorf("the dragged creature after the undo = %+v, want (4,3) with the mark half", got)
	}
	if !fog.told(grid.Square{Col: 3, Row: 1}, grid.Square{Col: 4, Row: 3}) {
		t.Errorf("the fog was told %v, want the dragged creature back from (3,1) to (4,3)", fog.moved)
	}
}
