package combat

import (
	"slices"
	"testing"
)

func TestAdjustAppliesResistanceThenVulnerabilityRoundingDown(t *testing.T) {
	fire := "damage-type:fire"
	cut := "damage-type:slashing"
	tests := []struct {
		name   string
		amount int
		typ    string
		mods   []Modifier
		ignore []string
		want   int
		kinds  []string
	}{
		{"no modifier", 28, fire, nil, nil, 28, nil},
		{"tiefling takes half of fire", 28, fire, []Modifier{{StepResistance, "race:tiefling", []string{fire}}}, nil, 14, []string{StepResistance}},
		{"an odd number rounds down", 7, cut, []Modifier{{StepResistance, "feature:rage", []string{cut}}}, nil, 3, []string{StepResistance}},
		{"a one is halved to nothing", 1, cut, []Modifier{{StepResistance, "feature:rage", []string{cut}}}, nil, 0, []string{StepResistance}},
		{"two resistances to one type count once", 10, cut, []Modifier{
			{StepResistance, "feature:rage", []string{cut}}, {StepResistance, "item:ring", []string{cut}},
		}, nil, 5, []string{StepResistance}},
		{"the same source twice counts once", 10, cut, []Modifier{
			{StepResistance, "feature:rage", []string{cut}}, {StepResistance, "feature:rage", []string{cut}},
		}, nil, 5, []string{StepResistance}},
		{"vulnerability doubles", 7, "damage-type:bludgeoning", []Modifier{{StepVulnerability, "monster:skeleton", []string{"damage-type:bludgeoning"}}}, nil, 14, []string{StepVulnerability}},
		{"two vulnerabilities count once", 7, "damage-type:bludgeoning", []Modifier{
			{StepVulnerability, "a", []string{"damage-type:bludgeoning"}}, {StepVulnerability, "b", []string{"damage-type:bludgeoning"}},
		}, nil, 14, []string{StepVulnerability}},
		{"resistance first, then vulnerability", 7, fire, []Modifier{
			{StepVulnerability, "v", []string{fire}}, {StepResistance, "r", []string{fire}},
		}, nil, 6, []string{StepResistance, StepVulnerability}},
		{"immunity takes everything", 30, "damage-type:poison", []Modifier{
			{StepImmunity, "monster:skeleton", []string{"damage-type:poison"}}, {StepResistance, "x", []string{"damage-type:poison"}},
		}, nil, 0, []string{StepImmunity}},
		{"another type is untouched", 12, "damage-type:cold", []Modifier{{StepResistance, "race:tiefling", []string{fire}}}, nil, 12, nil},
		{"the master ignores the source", 28, fire, []Modifier{{StepResistance, "race:tiefling", []string{fire}}}, []string{"race:tiefling"}, 28, nil},
		{"ignoring one of two sources leaves the other", 10, cut, []Modifier{
			{StepResistance, "feature:rage", []string{cut}}, {StepResistance, "item:ring", []string{cut}},
		}, []string{"feature:rage"}, 5, []string{StepResistance}},
		{"no type is never adjusted", 9, "", []Modifier{{StepResistance, "x", []string{fire}}}, nil, 9, nil},
		{"zero stays zero", 0, fire, []Modifier{{StepVulnerability, "x", []string{fire}}}, nil, 0, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, steps := Adjust(tt.amount, tt.typ, tt.mods, tt.ignore)
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
			if len(steps) > 0 && steps[len(steps)-1].After != got {
				t.Fatalf("the last step must end at the landed damage: %v vs %d", steps, got)
			}
		})
	}
}

func TestAdjustNamesEverySourceOfAStep(t *testing.T) {
	cut := "damage-type:slashing"
	_, steps := Adjust(10, cut, []Modifier{
		{StepResistance, "feature:rage", []string{cut}}, {StepResistance, "item:ring", []string{cut}},
	}, nil)
	if len(steps) != 1 || !slices.Equal(steps[0].Sources, []string{"feature:rage", "item:ring"}) || steps[0].Before != 10 || steps[0].After != 5 {
		t.Fatalf("steps = %+v", steps)
	}
}
