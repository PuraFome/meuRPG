package rules

import (
	"errors"
	"testing"
)

func TestRechargesOnRest(t *testing.T) {
	t.Parallel()
	cases := []struct {
		recharge string
		short    bool
		long     bool
	}{
		{RechargeShortRest, true, true},
		{RechargeLongRest, false, true},
		{RechargeDawn, false, false},
		{RechargeNone, false, false},
		{"", false, false},
	}
	for _, tc := range cases {
		if got := RechargesOnRest(tc.recharge, RestShort); got != tc.short {
			t.Errorf("%q on a short rest = %v, want %v", tc.recharge, got, tc.short)
		}
		if got := RechargesOnRest(tc.recharge, RestLong); got != tc.long {
			t.Errorf("%q on a long rest = %v, want %v", tc.recharge, got, tc.long)
		}
	}
}

// TestEveryClassResourceComesBackWhenTheSRDSays reads the recharge the sheet derives
// for each resource of the classes the task names, so a change in the effects
// that moves a resource to another rest fails here.
func TestEveryClassResourceComesBackWhenTheSRDSays(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	cases := []struct {
		class    string
		level    int
		resource string
		short    bool // back after a short rest (a long rest always restores these too)
	}{
		{"class:barbarian", 3, "rage", false},          // Barbarian 1: a long rest
		{"class:monk", 5, "ki", true},                  // Monk 2: a short or long rest
		{"class:sorcerer", 5, "sorcery_points", false}, // Sorcerer 2: a long rest
		{"class:cleric", 6, "channel_divinity", true},  // Cleric 2: a short or long rest
		{"class:paladin", 5, "lay_on_hands", false},    // Paladin 1: a long rest
		{"class:paladin", 5, "channel_divinity", true}, // Paladin 3, Sacred Oath
		{"class:bard", 4, "bardic_inspiration", false}, // Bard 1: a long rest
		{"class:bard", 5, "bardic_inspiration", true},  // Bard 5, Font of Inspiration: short or long
		{"class:druid", 4, "wild_shape", true},         // Druid 2: a short or long rest
		{"class:fighter", 3, "second_wind", true},      // Fighter 1
		{"class:fighter", 3, "action_surge", true},     // Fighter 2
		{"class:fighter", 9, "indomitable", false},     // Fighter 9: a long rest
		{"class:wizard", 3, "arcane_recovery", false},  // Wizard 1: once per day
	}
	for _, tc := range cases {
		d := Derive(standard(tc.class, tc.level), c)
		found := false
		for _, r := range d.Resources {
			if r.Key != tc.resource {
				continue
			}
			found = true
			if got := RechargesOnRest(r.Recharge, RestShort); got != tc.short {
				t.Errorf("%s %d: %s back after a short rest = %v, want %v (recharge %q)", tc.class, tc.level, tc.resource, got, tc.short, r.Recharge)
			}
			if !RechargesOnRest(r.Recharge, RestLong) {
				t.Errorf("%s %d: %s does not come back after a long rest (recharge %q)", tc.class, tc.level, tc.resource, r.Recharge)
			}
		}
		if !found {
			t.Errorf("%s %d has no %s: %+v", tc.class, tc.level, tc.resource, d.Resources)
		}
	}
}

func TestSlotsAndTemporaryHitPointsAndTheLongRestCondition(t *testing.T) {
	t.Parallel()
	if SpellSlotsReturnOn(RestShort) || !SpellSlotsReturnOn(RestLong) {
		t.Error("spell slots come back after a long rest only")
	}
	if !PactSlotsReturnOn(RestShort) || !PactSlotsReturnOn(RestLong) {
		t.Error("pact slots come back after a short or long rest")
	}
	if TemporaryHitPointsEndOn(RestShort) || !TemporaryHitPointsEndOn(RestLong) {
		t.Error("temporary hit points end with a long rest only")
	}
	if LongRestNeedsHitPoints(0) || !LongRestNeedsHitPoints(1) {
		t.Error("a long rest needs 1 hit point")
	}
}

var fighter5Wizard1 = []HitDice{{Die: 10, Count: 5}, {Die: 6, Count: 1}}

func TestHitDiceSpentAreKeptBySize(t *testing.T) {
	t.Parallel()
	used := HitDiceUsed{10: 2, 6: 1}
	if used.Total() != 3 || HitDiceTotal(fighter5Wizard1) != 6 {
		t.Fatalf("total used %d, total %d", used.Total(), HitDiceTotal(fighter5Wizard1))
	}
	left := HitDiceLeft(fighter5Wizard1, used)
	if left[10] != 3 || left[6] != 0 || len(left) != 1 {
		t.Errorf("left = %v, want 3d10 only", left)
	}
	// A size the character lost, and a count above its dice, are cut.
	dirty := HitDiceUsed{10: 9, 8: 2, 6: -1, 12: 0}
	clean := dirty.Clean(fighter5Wizard1)
	if len(clean) != 1 || clean[10] != 5 {
		t.Errorf("clean = %v, want 5d10", clean)
	}
}

func TestSplitHitDiceUsedFromTheOldCount(t *testing.T) {
	t.Parallel()
	cases := []struct {
		have  []HitDice
		total int
		want  HitDiceUsed
	}{
		{[]HitDice{{Die: 8, Count: 4}}, 3, HitDiceUsed{8: 3}},
		{[]HitDice{{Die: 8, Count: 4}}, 9, HitDiceUsed{8: 4}},
		{fighter5Wizard1, 0, HitDiceUsed{}},
		{fighter5Wizard1, 4, HitDiceUsed{10: 4}},
		{fighter5Wizard1, 6, HitDiceUsed{10: 5, 6: 1}},
		{[]HitDice{{Die: 6, Count: 1}, {Die: 10, Count: 5}}, 6, HitDiceUsed{10: 5, 6: 1}},
	}
	for _, tc := range cases {
		got := SplitHitDiceUsed(tc.have, tc.total)
		if len(got) != len(tc.want) {
			t.Errorf("%v used %d = %v, want %v", tc.have, tc.total, got, tc.want)
			continue
		}
		for die, n := range tc.want {
			if got[die] != n {
				t.Errorf("%v used %d = %v, want %v", tc.have, tc.total, got, tc.want)
			}
		}
	}
}

func TestSpendHitDiceByType(t *testing.T) {
	t.Parallel()
	used := HitDiceUsed{10: 4}
	after, err := SpendHitDice(fighter5Wizard1, used, HitDiceUsed{10: 1, 6: 1})
	if err != nil || after[10] != 5 || after[6] != 1 {
		t.Fatalf("spend = %v, %v", after, err)
	}
	if used[10] != 4 || used[6] != 0 {
		t.Errorf("the spent dice passed in were changed: %v", used)
	}
	for name, spend := range map[string]HitDiceUsed{
		"a size the character has not": {8: 1},
		"more than are left":           {10: 2},
		"zero dice":                    {10: 0},
		"a negative count":             {6: -1},
	} {
		if _, err := SpendHitDice(fighter5Wizard1, used, spend); !errors.Is(err, ErrHitDice) {
			t.Errorf("%s: err = %v, want ErrHitDice", name, err)
		}
	}
}

func TestHitDieHealAddsTheConstitutionModifierAndNeverTakesHitPoints(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ face, con, want int }{{6, 2, 8}, {1, 0, 1}, {1, -2, 0}, {10, -1, 9}} {
		if got := HitDieHeal(tc.face, tc.con); got != tc.want {
			t.Errorf("d%d roll + %d = %d, want %d", tc.face, tc.con, got, tc.want)
		}
	}
}

func TestLongRestGivesBackHalfOfAllTheDiceAtLeastOne(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ total, want int }{{1, 1}, {2, 1}, {3, 1}, {4, 2}, {8, 4}, {20, 10}} {
		if got := LongRestHitDiceLimit([]HitDice{{Die: 8, Count: tc.total}}); got != tc.want {
			t.Errorf("%d dice: limit %d, want %d", tc.total, got, tc.want)
		}
	}
}

func TestHitDiceReturnedByLongRest(t *testing.T) {
	t.Parallel()
	// 6 dice: half is 3. The default gives the largest back first.
	used := HitDiceUsed{10: 4, 6: 1}
	back, after, err := HitDiceReturned(fighter5Wizard1, used, nil)
	if err != nil || back[10] != 3 || len(back) != 1 || after[10] != 1 || after[6] != 1 {
		t.Fatalf("default: back %v, after %v, err %v", back, after, err)
	}
	// The player chooses the types.
	back, after, err = HitDiceReturned(fighter5Wizard1, used, HitDiceUsed{10: 2, 6: 1})
	if err != nil || back.Total() != 3 || after[10] != 2 || len(after) != 1 {
		t.Fatalf("choice: back %v, after %v, err %v", back, after, err)
	}
	// Fewer than the limit is allowed.
	if _, after, err = HitDiceReturned(fighter5Wizard1, used, HitDiceUsed{6: 1}); err != nil || after[10] != 4 || after[6] != 0 {
		t.Errorf("fewer: after %v, err %v", after, err)
	}
	// Nothing spent: nothing comes back.
	if back, _, err = HitDiceReturned(fighter5Wizard1, HitDiceUsed{}, nil); err != nil || back.Total() != 0 {
		t.Errorf("nothing spent: back %v, err %v", back, err)
	}
	// One die, whatever the level: at least one.
	one := []HitDice{{Die: 8, Count: 1}}
	if back, _, err = HitDiceReturned(one, HitDiceUsed{8: 1}, nil); err != nil || back[8] != 1 {
		t.Errorf("a level 1 character gets its die back: %v, %v", back, err)
	}
	for name, choice := range map[string]HitDiceUsed{
		"more than the limit":  {10: 3, 6: 1},
		"more than are spent":  {6: 2},
		"a size not owned":     {8: 1},
		"a zero count":         {10: 0},
		"above spent of a die": {10: 5},
	} {
		if _, _, err := HitDiceReturned(fighter5Wizard1, used, choice); !errors.Is(err, ErrHitDice) {
			t.Errorf("%s: err = %v, want ErrHitDice", name, err)
		}
	}
}
