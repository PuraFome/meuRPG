package rules

import "testing"

// shieldFighter is a fighter in chain mail with a shield and the weapons listed.
func shieldFighter(weapons ...string) Build {
	b := standard("class:fighter", 1)
	b.Armor, b.Shield = "equipment:chain-mail", true
	b.Weapons = weapons
	return b
}

// TestAShieldAddsNothingBesideATwoHandedWeapon: the shield's bonus holds only while a
// hand is free for it. The sheet lists the weapons carried, so a two-handed weapon on
// it takes the shield's bonus away, and the player gets it back by taking the weapon
// off the sheet. A versatile weapon is one-handed here: the sheet does not say how
// it is held.
func TestAShieldAddsNothingBesideATwoHandedWeapon(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	tests := []struct {
		name    string
		weapons []string
		wantAC  int
	}{
		{"no weapon", nil, 18},
		{"a one-handed weapon", []string{"equipment:longsword"}, 18},
		{"a versatile weapon", []string{"equipment:quarterstaff"}, 18},
		{"a two-handed weapon", []string{"equipment:greatsword"}, 16},
		{"a one-handed and a two-handed weapon", []string{"equipment:longsword", "equipment:greatsword"}, 16},
		{"a two-handed ranged weapon", []string{"equipment:longbow"}, 16},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := Derive(shieldFighter(tt.weapons...), c).ArmorClass; got != tt.wantAC {
				t.Errorf("AC with %v = %d, want %d", tt.weapons, got, tt.wantAC)
			}
		})
	}
}
