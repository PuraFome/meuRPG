package rules

import "testing"

// Ki-Empowered Strikes (monk 6) makes the unarmed strike, and only it, count as magical;
// a monk below 6, a monk weapon and anyone else's unarmed strike are not.
func TestKiEmpoweredStrikesMakeTheUnarmedStrikeMagical(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	multi := monkBuild(6, 9, 18)
	multi.Classes = append(multi.Classes, ClassLevel{Class: "class:fighter", Level: 3})
	for _, tc := range []struct {
		name  string
		build Build
		want  bool
	}{
		{"monk 5", monkBuild(5, 9, 18), false},
		{"monk 6", monkBuild(6, 9, 18), true},
		{"monk 6 and fighter 3", multi, true},
		{"fighter 6", standard("class:fighter", 6), false},
	} {
		d := Derive(tc.build, c)
		unarmed, ok := attackOf(d, UnarmedStrikeKey)
		if !ok {
			t.Fatalf("%s: no unarmed strike", tc.name)
		}
		if unarmed.Magical != tc.want {
			t.Errorf("%s: unarmed strike Magical = %v, want %v", tc.name, unarmed.Magical, tc.want)
		}
		for _, w := range []string{"equipment:dagger", "equipment:quarterstaff"} {
			if a, ok := attackOf(d, w); ok && a.Magical {
				t.Errorf("%s: %s is magical, want a nonmagical weapon", tc.name, w)
			}
		}
	}
}
