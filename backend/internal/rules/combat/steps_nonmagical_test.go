package combat

import (
	"slices"
	"testing"
)

// The resistance and immunity "from nonmagical attacks" of the SRD's stat blocks:
// they cover an attack that is not magical, and nothing else.
func TestAdjustNonmagicalQualifier(t *testing.T) {
	cut := "damage-type:slashing"
	fire := "damage-type:fire"
	nonmagicalRes := Modifier{Kind: StepResistance, Source: "monster:ghost", DamageTypes: []string{cut}, Qualifier: QualifierNonmagical}
	nonmagicalImm := Modifier{Kind: StepImmunity, Source: "monster:golem", DamageTypes: []string{cut}, Qualifier: QualifierNonmagical}
	mundane := Origin{Attack: true}
	magic := Origin{Attack: true, Magical: true}
	tests := []struct {
		name   string
		amount int
		typ    string
		mods   []Modifier
		ignore []string
		origin Origin
		want   int
		kinds  []string
	}{
		{"a nonmagical attack is halved", 9, cut, []Modifier{nonmagicalRes}, nil, mundane, 4, []string{StepResistance}},
		{"a magical attack is not", 9, cut, []Modifier{nonmagicalRes}, nil, magic, 9, nil},
		{"damage that is no attack is not", 9, cut, []Modifier{nonmagicalRes}, nil, Origin{}, 9, nil},
		{"another type is not", 9, fire, []Modifier{nonmagicalRes}, nil, mundane, 9, nil},
		{"immunity takes a nonmagical attack to nothing", 9, cut, []Modifier{nonmagicalImm}, nil, mundane, 0, []string{StepImmunity}},
		{"immunity lets a magical attack through", 9, cut, []Modifier{nonmagicalImm}, nil, magic, 9, nil},
		{"immunity to a non-attack damage does not apply", 9, cut, []Modifier{nonmagicalImm}, nil, Origin{}, 9, nil},
		{"two nonmagical resistances count once", 9, cut, []Modifier{
			nonmagicalRes, {Kind: StepResistance, Source: "item:ring", DamageTypes: []string{cut}, Qualifier: QualifierNonmagical},
		}, nil, mundane, 4, []string{StepResistance}},
		{"a plain resistance and a nonmagical one count once", 9, cut, []Modifier{
			nonmagicalRes, {Kind: StepResistance, Source: "feature:rage", DamageTypes: []string{cut}},
		}, nil, mundane, 4, []string{StepResistance}},
		{"a plain resistance still halves a magical attack", 9, cut, []Modifier{
			nonmagicalRes, {Kind: StepResistance, Source: "feature:rage", DamageTypes: []string{cut}},
		}, nil, magic, 4, []string{StepResistance}},
		{"a nonmagical immunity beats a resistance on a nonmagical attack", 9, cut, []Modifier{
			nonmagicalImm, {Kind: StepResistance, Source: "feature:rage", DamageTypes: []string{cut}},
		}, nil, mundane, 0, []string{StepImmunity}},
		{"the master leaves the source out", 9, cut, []Modifier{nonmagicalRes}, []string{"monster:ghost"}, mundane, 9, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, steps := Adjust(tt.amount, tt.typ, tt.mods, tt.ignore, tt.origin)
			if got != tt.want {
				t.Fatalf("Adjust = %d, want %d (steps %v)", got, tt.want, steps)
			}
			var kinds []string
			for _, s := range steps {
				kinds = append(kinds, s.Kind)
			}
			if !slices.Equal(kinds, tt.kinds) {
				t.Fatalf("steps = %v, want %v", kinds, tt.kinds)
			}
		})
	}
}

// The step of a nonmagical resistance says so.
func TestAdjustNonmagicalStepCarriesTheQualifier(t *testing.T) {
	cut := "damage-type:slashing"
	_, steps := Adjust(9, cut, []Modifier{
		{Kind: StepResistance, Source: "monster:ghost", DamageTypes: []string{cut}, Qualifier: QualifierNonmagical},
	}, nil, Origin{Attack: true})
	if len(steps) != 1 || steps[0].Qualifier != QualifierNonmagical || steps[0].Before != 9 || steps[0].After != 4 {
		t.Fatalf("steps = %+v", steps)
	}
	_, plain := Adjust(9, cut, []Modifier{{Kind: StepResistance, Source: "feature:rage", DamageTypes: []string{cut}}}, nil, Origin{Attack: true})
	if len(plain) != 1 || plain[0].Qualifier != "" {
		t.Fatalf("a plain resistance has a qualifier: %+v", plain)
	}
}

// AdjustForType is for the damage that is no attack (a trap, a zone, a spell's own
// fire): the clause "from nonmagical attacks" does not reach it.
func TestAdjustForTypeLeavesTheNonmagicalClauseToAttacks(t *testing.T) {
	cut := "damage-type:slashing"
	m := TypeModifiers{ResistantNonmagical: []string{cut}, ImmuneNonmagical: []string{cut}}
	if got := AdjustForType(10, cut, m); got != 10 {
		t.Errorf("AdjustForType = %d, want 10", got)
	}
}
