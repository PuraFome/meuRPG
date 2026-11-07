package combat

import (
	"slices"
	"testing"
)

// The flat damage reduction of a weapon attack (#572): taken off before resistance, never below 0,
// and only for the weapon attacks that hit. The sources are invented.
func TestAdjustWeaponDamageReduction(t *testing.T) {
	t.Parallel()
	cut := "damage-type:slashing"
	fire := "damage-type:fire"
	plain := Modifier{Kind: StepReduction, Source: "feature:thick-hide", Amount: 3}
	mundaneOnly := Modifier{Kind: StepReduction, Source: "feature:soft-skin", Amount: 4, Qualifier: QualifierNonmagical}
	resist := Modifier{Kind: StepResistance, Source: "feature:cold-blood", DamageTypes: []string{cut}}
	immune := Modifier{Kind: StepImmunity, Source: "feature:stone-flesh", DamageTypes: []string{cut}}
	mundaneWeapon := Origin{Attack: true, Weapon: true}
	magicWeapon := Origin{Attack: true, Weapon: true, Magical: true}
	tests := []struct {
		name    string
		amount  int
		typ     string
		mods    []Modifier
		ignore  []string
		origin  Origin
		want    int
		sources []string // the sources of the reduction step, nil for no step
	}{
		{"a weapon attack loses the amount", 10, cut, []Modifier{plain}, nil, mundaneWeapon, 7, []string{"feature:thick-hide"}},
		{"it covers every damage type", 10, fire, []Modifier{plain}, nil, mundaneWeapon, 7, []string{"feature:thick-hide"}},
		{"a magic weapon is reduced by a plain one", 10, cut, []Modifier{plain}, nil, magicWeapon, 7, []string{"feature:thick-hide"}},
		{"the nonmagical one reduces a nonmagical weapon", 10, cut, []Modifier{mundaneOnly}, nil, mundaneWeapon, 6, []string{"feature:soft-skin"}},
		{"the nonmagical one leaves a magic weapon alone", 10, cut, []Modifier{mundaneOnly}, nil, magicWeapon, 10, nil},
		{"it never goes below 0", 2, cut, []Modifier{plain}, nil, mundaneWeapon, 0, []string{"feature:thick-hide"}},
		{"a spell attack is not reduced", 10, fire, []Modifier{plain}, nil, Origin{Attack: true, Magical: true}, 10, nil},
		{"damage that is no attack is not reduced", 10, cut, []Modifier{plain}, nil, Origin{}, 10, nil},
		{"the master leaves the source out", 10, cut, []Modifier{plain}, []string{"feature:thick-hide"}, mundaneWeapon, 10, nil},
		{"two sources add and both are named", 10, cut, []Modifier{plain, mundaneOnly}, nil, mundaneWeapon, 3, []string{"feature:thick-hide", "feature:soft-skin"}},
		{"one source left out leaves the other", 10, cut, []Modifier{plain, mundaneOnly}, []string{"feature:soft-skin"}, mundaneWeapon, 7, []string{"feature:thick-hide"}},
		{"the same source twice adds both amounts", 10, cut, []Modifier{plain, {Kind: StepReduction, Source: "feature:thick-hide", Amount: 2, Qualifier: QualifierNonmagical}}, nil, mundaneWeapon, 5, []string{"feature:thick-hide"}},
		{"resistance comes after it: 10 - 3 = 7, halved to 3", 10, cut, []Modifier{plain, resist}, nil, mundaneWeapon, 3, []string{"feature:thick-hide"}},
		{"without the reduction the same damage is halved to 5", 10, cut, []Modifier{resist}, nil, mundaneWeapon, 5, nil},
		{"immunity still takes it to 0", 10, cut, []Modifier{plain, immune}, nil, mundaneWeapon, 0, []string{"feature:thick-hide"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, steps := Adjust(tt.amount, tt.typ, tt.mods, tt.ignore, tt.origin)
			if got != tt.want {
				t.Fatalf("Adjust = %d, want %d (steps %v)", got, tt.want, steps)
			}
			var sources []string
			for _, s := range steps {
				if s.Kind != StepReduction {
					continue
				}
				if s.DamageType != tt.typ || s.Before != tt.amount {
					t.Errorf("step = %+v, want type %s before %d", s, tt.typ, tt.amount)
				}
				sources = s.Sources
				if s.After != max(tt.amount-reductionOf(tt.mods, tt.ignore, tt.origin), 0) {
					t.Errorf("step After = %d", s.After)
				}
			}
			if !slices.Equal(sources, tt.sources) {
				t.Errorf("reduction sources = %v, want %v", sources, tt.sources)
			}
		})
	}
}

// reductionOf is the amount the test expects the covering reductions to take, written apart from Adjust.
func reductionOf(mods []Modifier, ignored []string, from Origin) int {
	total := 0
	for _, m := range mods {
		if m.Kind == StepReduction && from.Weapon && !slices.Contains(ignored, m.Source) && (m.Qualifier == "" || !from.Magical) {
			total += m.Amount
		}
	}
	return total
}
