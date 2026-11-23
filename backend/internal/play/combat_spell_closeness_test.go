package play

import (
	"fmt"
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

// How close the targets of an SRD spell must stand, checked on bare combatants: no database, and distances the
// test map is too small for. The spells are read from the SRD content.

// srdSpellOf is the spell as the target check reads it, from the SRD content.
func srdSpellOf(t *testing.T, key string) link.Spell {
	t.Helper()
	c, err := rules.LoadSRD()
	if err != nil {
		t.Fatal(err)
	}
	d, ok := c.SpellDetails(key)
	if !ok {
		t.Fatalf("%s is not an SRD spell", key)
	}
	tg := d.Target
	return link.Spell{
		Key: key, Level: d.Spell.Level, RangeKind: d.Range.Kind, RangeFt: d.Range.DistanceFt, AttackType: d.AttackType,
		TargetCount: tg.MaxTargets(d.Spell.Level, d.Spell.Level), TargetPerLevel: tg.PerSlotLevel,
		WithinFt: tg.WithinFt, WithinFirst: tg.WithinOf == rules.WithinFirst, RangeExtraFt: tg.RangeExtraFt, ReachFt: tg.ReachFt,
	}
}

// openRow is a map of one row, 40 squares long, with no wall.
var openRow = grid.Terrain{Grid: grid.Grid{Columns: 40, Rows: 1}}

// onRow is a combatant standing at a column of the first row; the caster stands at column 0.
func onRow(id string, col int32) playdb.Combatant {
	row := int32(0)
	return playdb.Combatant{ID: id, GridCol: &col, GridRow: &row}
}

// TestSpellClosenessOfLegalCasts: a cast the SRD allows is never refused, however it stands against the spell's range.
func TestSpellClosenessOfLegalCasts(t *testing.T) {
	t.Parallel()
	caster := onRow("caster", 0)
	for _, tc := range []struct {
		name    string
		spell   string
		slot    int
		targets []int32 // columns, in the order the caster names them
		missing int32   // 0: the cast goes through; else the feet the refusal says are missing
	}{
		{"Chain Lightning: first at 150 ft, second at 160 ft", "spell:chain-lightning", 6, []int32{30, 32}, 0},
		{"Chain Lightning: the third 30 ft from the first and 60 ft from the second", "spell:chain-lightning", 6, []int32{20, 14, 26}, 0},
		{"Chain Lightning: a second target 35 ft from the first", "spell:chain-lightning", 6, []int32{20, 27}, 5},
		{"Chain Lightning: the first beyond 150 ft", "spell:chain-lightning", 6, []int32{31}, 5},
		{"Mass Cure Wounds: a creature at 90 ft", "spell:mass-cure-wounds", 5, []int32{18}, 0},
		{"Mass Cure Wounds: a creature at 95 ft", "spell:mass-cure-wounds", 5, []int32{19}, 5},
		{"Mass Cure Wounds: two creatures 60 ft apart", "spell:mass-cure-wounds", 5, []int32{6, 18}, 0},
		{"Mass Cure Wounds: two creatures 65 ft apart", "spell:mass-cure-wounds", 5, []int32{5, 18}, 5},
		{"Mass Cure Wounds: the farthest pair is not the first's", "spell:mass-cure-wounds", 5, []int32{9, 3, 18}, 15},
		{"Acid Splash: two creatures 5 ft apart in range", "spell:acid-splash", 0, []int32{11, 12}, 0},
		{"Acid Splash: two creatures 10 ft apart", "spell:acid-splash", 0, []int32{10, 12}, 5},
		{"Acid Splash: the second beyond the range", "spell:acid-splash", 0, []int32{12, 13}, 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			targets := make([]playdb.Combatant, len(tc.targets))
			for i, col := range tc.targets {
				targets[i] = onRow(fmt.Sprintf("t%d", i), col)
			}
			err := (&Service{}).checkTargets(combatViewer{}, openRow, nil, srdSpellOf(t, tc.spell), caster, targets, nil, 0, tc.slot, false)
			if tc.missing == 0 {
				if err != nil {
					t.Fatalf("a legal cast was refused: %v", err)
				}
				return
			}
			b := wantEncounterBlocked(t, err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_TARGET_OUT_OF_REACH)
			if b.GetMissingFt() != tc.missing {
				t.Errorf("missing %d ft, want %d", b.GetMissingFt(), tc.missing)
			}
		})
	}
}

// TestSpellClosenessIsNotCheckedForTheMasterOrTheatre: the master and a combat without a map are never held to it.
func TestSpellClosenessIsNotCheckedForTheMasterOrTheatre(t *testing.T) {
	t.Parallel()
	targets := []playdb.Combatant{onRow("a", 3), onRow("b", 19)}
	sp := srdSpellOf(t, "spell:mass-cure-wounds")
	if err := (&Service{}).checkTargets(combatViewer{master: true}, openRow, nil, sp, onRow("caster", 0), targets, nil, 0, 5, false); err != nil {
		t.Errorf("the master was held to it: %v", err)
	}
	if err := (&Service{}).checkTargets(combatViewer{}, openRow, nil, sp, onRow("caster", 0), targets, nil, 0, 5, true); err != nil {
		t.Errorf("a combat without a map was held to it: %v", err)
	}
}
