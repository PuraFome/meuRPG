package play

import (
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// TestShapeOfSpellReadsTheReachOfASelfSpell: Produce Flame's range is Self and its hurl reaches 30 ft, which is the
// range its attack roll is read at; an ordinary ranged spell keeps its range.
func TestShapeOfSpellReadsTheReachOfASelfSpell(t *testing.T) {
	t.Parallel()
	if got := shapeOfSpell(link.Spell{RangeKind: rules.RangeSelf, ReachFt: 30, AttackType: "ranged"}); got.RangeFt != 30 || got.Melee {
		t.Errorf("a Self spell with a 30 ft reach = %+v, want ranged at 30 ft", got)
	}
	if got := shapeOfSpell(link.Spell{RangeKind: rules.RangeRanged, RangeFt: 120, AttackType: "ranged"}); got.RangeFt != 120 {
		t.Errorf("a 120 ft spell = %+v, want 120 ft", got)
	}
}
