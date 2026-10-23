package combat

import "testing"

// A save that halves the damage on a success: Evasion (SRD 5.1, Monk and Rogue) turns a
// Dexterity save into no damage on a success and half on a failure, unless the creature
// is incapacitated; any other ability, or a creature without Evasion, keeps the plain rule.
func TestHalfSaveShare(t *testing.T) {
	t.Parallel()
	evasive := Creature{Evasion: true}
	with := func(condition string) Creature {
		return Creature{Evasion: true, Conditions: []string{condition}}
	}
	for _, tc := range []struct {
		name    string
		c       Creature
		ability string
		saved   bool
		want    SaveShare
	}{
		{"no Evasion, dex, success", Creature{}, "dex", true, ShareHalf},
		{"no Evasion, dex, failure", Creature{}, "dex", false, ShareFull},
		{"Evasion, dex, success", evasive, "dex", true, ShareNone},
		{"Evasion, dex, failure", evasive, "dex", false, ShareHalf},
		{"Evasion, con, success", evasive, "con", true, ShareHalf},
		{"Evasion, con, failure", evasive, "con", false, ShareFull},
		{"Evasion, wis, failure", evasive, "wis", false, ShareFull},
		{"Evasion, incapacitated, success", with("condition:incapacitated"), "dex", true, ShareHalf},
		{"Evasion, incapacitated, failure", with("condition:incapacitated"), "dex", false, ShareFull},
		{"Evasion, stunned, failure", with("condition:stunned"), "dex", false, ShareFull},
		{"Evasion, unconscious, failure", with("condition:unconscious"), "dex", false, ShareFull},
		{"Evasion, prone, success", with("condition:prone"), "dex", true, ShareNone},
	} {
		if got := HalfSaveShare(tc.c, tc.ability, tc.saved); got != tc.want {
			t.Errorf("%s: HalfSaveShare = %v, want %v", tc.name, got, tc.want)
		}
	}
}
