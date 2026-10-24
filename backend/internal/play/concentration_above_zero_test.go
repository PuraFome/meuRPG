package play

import (
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// Guard for the concentration at the boundary of 0 hit points: a creature stops concentrating when
// it drops to 0 hit points (SRD 5.1, "Duration"), never while it keeps at least 1. The Goblin has 7
// hit points, Pensantus is the wizard of the casters table.

func aboveZeroSetHP(n int32) func(*playv1.AdjustCombatantHitPointsRequest) {
	return func(r *playv1.AdjustCombatantHitPointsRequest) {
		r.Change = &playv1.AdjustCombatantHitPointsRequest_HitPoints{HitPoints: n}
	}
}

func TestAdjustingAnNPCsHitPointsKeepsTheConcentrationAboveZero(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		edit     func(*playv1.AdjustCombatantHitPointsRequest)
		wantHP   int32
		wantKept bool
	}{
		{"set to a positive number", aboveZeroSetHP(5), 5, true},
		{"set to 1, the boundary", aboveZeroSetHP(1), 1, true},
		{"damage that leaves 1", damageHP(6), 1, true},
		{"damage that leaves some", damageHP(3), 4, true},
		{"heal at full hit points", healHP(4), 7, true},
		{"damage to exactly 0", damageHP(7), 0, false},
		{"set to 0", aboveZeroSetHP(0), 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := newCasters(t)
			e := a.closeFight(t)
			a.concentrate(t, "Goblin", webSpell)

			if _, err := a.adjustHP(t, e, "Goblin", tc.edit); err != nil {
				t.Fatalf("AdjustCombatantHitPoints() error = %v", err)
			}

			goblin := byLabel(t, a.get(t, a.master), "Goblin")
			if got := goblin.GetHitPointsCurrent(); got != tc.wantHP {
				t.Fatalf("the Goblin is at %d hit points, want %d", got, tc.wantHP)
			}
			wantSpell, wantLines := "", 1
			if tc.wantKept {
				wantSpell, wantLines = webSpell, 0
			}
			if got := goblin.GetConcentrationSpell(); got != wantSpell {
				t.Errorf("the Goblin concentrates on %q at %d hit points, want %q", got, tc.wantHP, wantSpell)
			}
			if n := concentrationEndedLines(t, a, webSpell); n != wantLines {
				t.Errorf("the combat log has %d lines saying the concentration on Web ended, want %d", n, wantLines)
			}
		})
	}
}

func TestAdjustingAConcentratingCharacterKeepsTheConcentrationAboveZero(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		hp       int32
		wantKept bool
	}{
		{"a positive number", 10, true},
		{"1, the boundary", 1, true},
		{"0", 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := newCasters(t)
			a.closeFight(t)
			a.concentrate(t, "Pensantus", webSpell)

			a.correct(t, a.pens, hpIs(tc.hp))

			pens := byLabel(t, a.get(t, a.master), "Pensantus")
			wantSpell, wantLines := "", 1
			if tc.wantKept {
				wantSpell, wantLines = webSpell, 0
			}
			if got := pens.GetConcentrationSpell(); got != wantSpell {
				t.Errorf("Pensantus concentrates on %q at %d hit points, want %q", got, tc.hp, wantSpell)
			}
			if n := concentrationEndedLines(t, a, webSpell); n != wantLines {
				t.Errorf("the combat log has %d lines saying the concentration on Web ended, want %d", n, wantLines)
			}
		})
	}
}
