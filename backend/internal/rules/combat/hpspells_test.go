package combat

import (
	"slices"
	"testing"
)

func TestResolvePool(t *testing.T) {
	t.Parallel()
	type want struct {
		index    int
		affected bool
		reason   string
		left     int
	}
	tests := []struct {
		name      string
		total     int
		creatures []HPCreature
		want      []want
	}{
		{
			// E8-03: 5d8 = 15; Goblin 1 (7) sleeps, 8 are left; the Capitão (27) does not fit.
			name: "Sono in the canonical fight", total: 15,
			creatures: []HPCreature{{HP: 27}, {HP: 7}},
			want:      []want{{1, true, PoolAffected, 8}, {0, false, PoolTooHigh, 8}},
		},
		{
			name: "ascending order, each one subtracts", total: 20,
			creatures: []HPCreature{{HP: 9}, {HP: 4}, {HP: 7}},
			want:      []want{{1, true, "", 16}, {2, true, "", 9}, {0, true, "", 0}},
		},
		{
			name: "exactly what is left still fits", total: 7,
			creatures: []HPCreature{{HP: 7}},
			want:      []want{{0, true, "", 0}},
		},
		{
			name: "one more than what is left does not", total: 6,
			creatures: []HPCreature{{HP: 7}},
			want:      []want{{0, false, PoolTooHigh, 6}},
		},
		{
			name: "ties keep the target order", total: 8,
			creatures: []HPCreature{{HP: 5}, {HP: 5}, {HP: 5}},
			want:      []want{{0, true, "", 3}, {1, false, PoolTooHigh, 3}, {2, false, PoolTooHigh, 3}},
		},
		{
			name: "the unconscious and the ones at 0 are skipped and spend nothing", total: 10,
			creatures: []HPCreature{{HP: 3, Unconscious: true}, {HP: 0}, {HP: 6}},
			want:      []want{{1, false, PoolSkipped, 10}, {0, false, PoolSkipped, 10}, {2, true, "", 4}},
		},
		{
			name: "nobody in the area", total: 15,
			creatures: nil, want: []want{},
		},
		{
			name: "a pool of 0 affects nobody", total: 0,
			creatures: []HPCreature{{HP: 1}},
			want:      []want{{0, false, PoolTooHigh, 0}},
		},
		{
			// Upcast: Sono at the 3rd circle rolls 9d8; the same arithmetic.
			name: "a bigger pool reaches more", total: 40,
			creatures: []HPCreature{{HP: 27}, {HP: 7}, {HP: 7}},
			want:      []want{{1, true, "", 33}, {2, true, "", 26}, {0, false, PoolTooHigh, 26}},
		},
	}
	for _, tt := range tests {
		got := ResolvePool(tt.total, tt.creatures)
		if len(got) != len(tt.want) {
			t.Errorf("%s: %d steps, want %d", tt.name, len(got), len(tt.want))
			continue
		}
		for i, w := range tt.want {
			g := got[i]
			if g.Index != w.index || g.Affected != w.affected || g.Reason != w.reason || g.Left != w.left || g.HP != tt.creatures[w.index].HP {
				t.Errorf("%s: step %d = %+v, want %+v", tt.name, i, g, w)
			}
		}
	}
}

func TestResolveThreshold(t *testing.T) {
	t.Parallel()
	tests := []struct {
		threshold, hp int
		want          bool
	}{
		{100, 27, true},
		{100, 100, true}, // exactly 100 is still affected (Matar)
		{100, 101, false},
		{150, 150, true}, // Atordoar
		{150, 151, false},
		{100, 0, true},
	}
	for _, tt := range tests {
		if got := ResolveThreshold(tt.threshold, tt.hp); got != tt.want {
			t.Errorf("ResolveThreshold(%d, %d) = %v, want %v", tt.threshold, tt.hp, got, tt.want)
		}
	}
}

func TestResolveZeroHP(t *testing.T) {
	t.Parallel()
	if !ResolveZeroHP(0) || ResolveZeroHP(1) || ResolveZeroHP(27) {
		t.Error("Poupar os Moribundos works only at exactly 0 hit points")
	}
}

func TestResolveFlatHeal(t *testing.T) {
	t.Parallel()
	ends := []string{"condition:blinded", "condition:deafened"}
	got := ResolveFlatHeal(70, 12, 40, []string{"condition:blinded", "condition:poisoned", "condition:deafened"}, ends)
	if got.HP != 40 || got.Healed != 28 || !slices.Equal(got.Conditions, []string{"condition:poisoned"}) || !slices.Equal(got.Ended, ends) {
		t.Errorf("heal capped at the maximum = %+v", got)
	}
	got = ResolveFlatHeal(70, 0, 200, nil, ends)
	if got.HP != 70 || got.Healed != 70 || got.Conditions == nil || len(got.Conditions) != 0 || len(got.Ended) != 0 {
		t.Errorf("heal from 0 with no conditions = %+v", got)
	}
}
