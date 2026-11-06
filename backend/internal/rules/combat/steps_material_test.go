package combat

import "testing"

// The qualifiers "nonmagical weapons that aren't silvered" and "... aren't adamantine"
// cover a nonmagical attack whose weapon is not of that material, and nothing else.
func TestCoversMaterialQualifiers(t *testing.T) {
	cut := "damage-type:slashing"
	origins := map[string]Origin{
		"plain": {Attack: true}, "magical": {Attack: true, Magical: true},
		"silvered": {Attack: true, Silvered: true}, "adamantine": {Attack: true, Adamantine: true},
		"no attack": {},
	}
	tests := []struct {
		qualifier string
		covers    map[string]bool
	}{
		{QualifierNonmagical, map[string]bool{"plain": true, "silvered": true, "adamantine": true}},
		{QualifierNonmagicalUnlessSilvered, map[string]bool{"plain": true, "adamantine": true}},
		{QualifierNonmagicalUnlessAdamantine, map[string]bool{"plain": true, "silvered": true}},
	}
	for _, tt := range tests {
		for name, o := range origins {
			t.Run(tt.qualifier+"/"+name, func(t *testing.T) {
				m := Modifier{Kind: StepResistance, Source: "monster:x", DamageTypes: []string{cut}, Qualifier: tt.qualifier}
				if got := m.Covers(cut, o); got != tt.covers[name] {
					t.Errorf("Covers = %v, want %v", got, tt.covers[name])
				}
				want := 9
				if tt.covers[name] {
					want = 4
				}
				got, steps := Adjust(9, cut, []Modifier{m}, nil, o)
				if got != want || (len(steps) == 1) != tt.covers[name] {
					t.Errorf("Adjust = %d (%v), want %d", got, steps, want)
				}
				if len(steps) == 1 && steps[0].Qualifier != tt.qualifier {
					t.Errorf("step qualifier = %q", steps[0].Qualifier)
				}
			})
		}
	}
}

// A mixed damage is adjusted type by type: the immunity of a silvered-exempt entry takes
// the slashing to nothing and leaves the fire alone.
func TestAdjustMaterialMixedTypes(t *testing.T) {
	mods := []Modifier{{Kind: StepImmunity, Source: "monster:werewolf-wolf", DamageTypes: []string{"damage-type:slashing"}, Qualifier: QualifierNonmagicalUnlessSilvered}}
	for _, tt := range []struct {
		name        string
		from        Origin
		slash, fire int
	}{
		{"plain", Origin{Attack: true}, 0, 5},
		{"silvered", Origin{Attack: true, Silvered: true}, 7, 5},
	} {
		s, _ := Adjust(7, "damage-type:slashing", mods, nil, tt.from)
		f, _ := Adjust(5, "damage-type:fire", mods, nil, tt.from)
		if s != tt.slash || f != tt.fire {
			t.Errorf("%s: slashing %d fire %d, want %d %d", tt.name, s, f, tt.slash, tt.fire)
		}
	}
}

// AdjustForType leaves the material lists to attacks, like the nonmagical ones.
func TestAdjustForTypeIgnoresMaterialLists(t *testing.T) {
	cut := "damage-type:slashing"
	m := TypeModifiers{ImmuneUnlessSilvered: []string{cut}, ResistantUnlessAdamantine: []string{cut}}
	if got := AdjustForType(10, cut, m); got != 10 {
		t.Errorf("AdjustForType = %d, want 10", got)
	}
}
