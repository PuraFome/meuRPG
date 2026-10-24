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
		{"tiefling takes half of fire", 28, fire, []Modifier{{Kind: StepResistance, Source: "race:tiefling", DamageTypes: []string{fire}}}, nil, 14, []string{StepResistance}},
		{"an odd number rounds down", 7, cut, []Modifier{{Kind: StepResistance, Source: "feature:rage", DamageTypes: []string{cut}}}, nil, 3, []string{StepResistance}},
		{"a one is halved to nothing", 1, cut, []Modifier{{Kind: StepResistance, Source: "feature:rage", DamageTypes: []string{cut}}}, nil, 0, []string{StepResistance}},
		{"two resistances to one type count once", 10, cut, []Modifier{
			{Kind: StepResistance, Source: "feature:rage", DamageTypes: []string{cut}}, {Kind: StepResistance, Source: "item:ring", DamageTypes: []string{cut}},
		}, nil, 5, []string{StepResistance}},
		{"an all-but-one resistance and the rage's overlap count once", 10, cut, []Modifier{
			{Kind: StepResistance, Source: "feature:rage", DamageTypes: []string{cut, "damage-type:piercing", "damage-type:bludgeoning"}},
			{Kind: StepResistance, Source: "feature:fixture-path", DamageTypes: []string{cut, "damage-type:piercing", fire}},
		}, nil, 5, []string{StepResistance}},
		{"the same source twice counts once", 10, cut, []Modifier{
			{Kind: StepResistance, Source: "feature:rage", DamageTypes: []string{cut}}, {Kind: StepResistance, Source: "feature:rage", DamageTypes: []string{cut}},
		}, nil, 5, []string{StepResistance}},
		{"vulnerability doubles", 7, "damage-type:bludgeoning", []Modifier{{Kind: StepVulnerability, Source: "monster:skeleton", DamageTypes: []string{"damage-type:bludgeoning"}}}, nil, 14, []string{StepVulnerability}},
		{"two vulnerabilities count once", 7, "damage-type:bludgeoning", []Modifier{
			{Kind: StepVulnerability, Source: "a", DamageTypes: []string{"damage-type:bludgeoning"}}, {Kind: StepVulnerability, Source: "b", DamageTypes: []string{"damage-type:bludgeoning"}},
		}, nil, 14, []string{StepVulnerability}},
		{"resistance first, then vulnerability", 7, fire, []Modifier{
			{Kind: StepVulnerability, Source: "v", DamageTypes: []string{fire}}, {Kind: StepResistance, Source: "r", DamageTypes: []string{fire}},
		}, nil, 6, []string{StepResistance, StepVulnerability}},
		{"immunity takes everything", 30, "damage-type:poison", []Modifier{
			{Kind: StepImmunity, Source: "monster:skeleton", DamageTypes: []string{"damage-type:poison"}}, {Kind: StepResistance, Source: "x", DamageTypes: []string{"damage-type:poison"}},
		}, nil, 0, []string{StepImmunity}},
		{"another type is untouched", 12, "damage-type:cold", []Modifier{{Kind: StepResistance, Source: "race:tiefling", DamageTypes: []string{fire}}}, nil, 12, nil},
		{"the master ignores the source", 28, fire, []Modifier{{Kind: StepResistance, Source: "race:tiefling", DamageTypes: []string{fire}}}, []string{"race:tiefling"}, 28, nil},
		{"ignoring one of two sources leaves the other", 10, cut, []Modifier{
			{Kind: StepResistance, Source: "feature:rage", DamageTypes: []string{cut}}, {Kind: StepResistance, Source: "item:ring", DamageTypes: []string{cut}},
		}, []string{"feature:rage"}, 5, []string{StepResistance}},
		{"no type is never adjusted", 9, "", []Modifier{{Kind: StepResistance, Source: "x", DamageTypes: []string{fire}}}, nil, 9, nil},
		{"zero stays zero", 0, fire, []Modifier{{Kind: StepVulnerability, Source: "x", DamageTypes: []string{fire}}}, nil, 0, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, steps := Adjust(tt.amount, tt.typ, tt.mods, tt.ignore, Origin{})
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
		{Kind: StepResistance, Source: "feature:rage", DamageTypes: []string{cut}}, {Kind: StepResistance, Source: "item:ring", DamageTypes: []string{cut}},
	}, nil, Origin{})
	if len(steps) != 1 || !slices.Equal(steps[0].Sources, []string{"feature:rage", "item:ring"}) || steps[0].Before != 10 || steps[0].After != 5 {
		t.Fatalf("steps = %+v", steps)
	}
}
