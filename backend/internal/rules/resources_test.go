package rules

import (
	"errors"
	"testing"
)

func TestLayOnHandsAmountIsOneUpToThePool(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		amount, pool int
		ok           bool
	}{{1, 25, true}, {25, 25, true}, {17, 17, true}, {0, 25, false}, {-3, 25, false}, {26, 25, false}, {1, 0, false}} {
		err := LayOnHandsAmount(tc.amount, tc.pool)
		if (err == nil) != tc.ok || (err != nil && !errors.Is(err, ErrLayOnHands)) {
			t.Errorf("%d of %d: err = %v, want ok=%v", tc.amount, tc.pool, err, tc.ok)
		}
	}
	if LayOnHandsCureCost != 5 {
		t.Errorf("a cure costs %d, the SRD says 5", LayOnHandsCureCost)
	}
}

func TestLayOnHandsHasNoEffectOnUndeadAndConstructs(t *testing.T) {
	t.Parallel()
	for typ, want := range map[string]bool{"undead": true, "construct": true, "humanoid": false, "fiend": false, "beast": false, "": false} {
		if got := LayOnHandsNoEffect(typ); got != want {
			t.Errorf("type %q: no effect = %v, want %v", typ, got, want)
		}
	}
}

func TestThePoolIsFiveHitPointsForEachPaladinLevel(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	for _, level := range []int{1, 5, 11, 20} {
		if got := resourceMax(Derive(standard("class:paladin", level), c), LayOnHandsKey); got != 5*level {
			t.Errorf("paladin %d: pool %d, want %d", level, got, 5*level)
		}
	}
}

func TestSlotCreationCostsAreTheSRDTable(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	want := map[int]int{1: 2, 2: 3, 3: 5, 4: 6, 5: 7}
	got := c.SlotCreationCosts()
	if len(got) != len(want) {
		t.Fatalf("costs = %v, want %v", got, want)
	}
	for level, cost := range want {
		if got[level] != cost {
			t.Errorf("a slot of level %d costs %d, want %d", level, got[level], cost)
		}
	}
	for _, level := range []int{0, 6, 9} {
		if _, ok := c.SlotCreationCost(level); ok {
			t.Errorf("a slot of level %d can be created: the SRD stops at the 5th", level)
		}
	}
}

func TestCreateSlotCheck(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	if cost, err := c.CreateSlotCheck(2, 5); err != nil || cost != 3 {
		t.Errorf("a 2nd-level slot with 5 points: %d, %v", cost, err)
	}
	if cost, err := c.CreateSlotCheck(2, 3); err != nil || cost != 3 {
		t.Errorf("exactly the cost: %d, %v", cost, err)
	}
	for name, tc := range map[string]struct{ level, points int }{
		"not enough points": {4, 5},
		"above the 5th":     {6, 20},
		"level 0":           {0, 20},
	} {
		if _, err := c.CreateSlotCheck(tc.level, tc.points); !errors.Is(err, ErrFlexibleCasting) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestConvertSlotGivesItsLevelInPointsUpToTheMaximum(t *testing.T) {
	t.Parallel()
	if gain, err := ConvertSlotCheck(2, 2, 5); err != nil || gain != 2 {
		t.Errorf("2nd-level slot at 2 of 5: %d, %v", gain, err)
	}
	if gain, err := ConvertSlotCheck(1, 4, 5); err != nil || gain != 1 {
		t.Errorf("reaching the maximum exactly: %d, %v", gain, err)
	}
	if _, err := ConvertSlotCheck(2, 4, 5); !errors.Is(err, ErrSorceryPointsOver) {
		t.Errorf("passing the maximum: err = %v, want ErrSorceryPointsOver", err)
	}
	if _, err := ConvertSlotCheck(1, 5, 5); !errors.Is(err, ErrSorceryPointsFull) {
		t.Errorf("at the maximum: err = %v, want ErrSorceryPointsFull", err)
	}
	if _, err := ConvertSlotCheck(0, 0, 5); !errors.Is(err, ErrFlexibleCasting) {
		t.Errorf("slot level 0: err = %v", err)
	}
	if !errors.Is(ErrSorceryPointsFull, ErrFlexibleCasting) || !errors.Is(ErrSorceryPointsOver, ErrFlexibleCasting) {
		t.Error("the maximum refusals are Flexible Casting refusals")
	}
}

func TestTheMaximumSorceryPointsAreTheSorcererLevel(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	for _, level := range []int{2, 5, 10, 20} {
		if got := resourceMax(Derive(standard("class:sorcerer", level), c), SorceryPointsKey); got != level {
			t.Errorf("sorcerer %d: %d points, want %d", level, got, level)
		}
	}
}

func TestBardicInspirationDieFollowsTheBardLevel(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	for level := 1; level <= 20; level++ {
		want := 6
		switch {
		case level >= 15:
			want = 12
		case level >= 10:
			want = 10
		case level >= 5:
			want = 8
		}
		if got := c.BardicInspirationDie(level); got != want {
			t.Errorf("bard %d: d%d, want d%d", level, got, want)
		}
	}
	if c.BardicInspirationDie(0) != 0 || c.BardicInspirationDie(21) != 0 {
		t.Error("a level outside 1 to 20 has no die")
	}
}

func TestBardicInspirationLastsTenMinutes(t *testing.T) {
	t.Parallel()
	if BardicInspirationRounds*6 != 10*60 { //nolint:mnd // 6 seconds a round, 10 minutes of 60 seconds
		t.Errorf("%d rounds are not 10 minutes", BardicInspirationRounds)
	}
	if BardicInspirationExpiry(3) != 103 {
		t.Errorf("a die given in round 3 expires in %d", BardicInspirationExpiry(3))
	}
}

func TestBardicInspirationRefusals(t *testing.T) {
	t.Parallel()
	ok := BardicInspirationTarget{OnMap: true, Distance: 45, CanHear: true}
	if r := BardicInspirationRefusal(ok); r != "" {
		t.Errorf("a creature at 45 ft that hears: %q", r)
	}
	edge := ok
	edge.Distance = BardicInspirationRangeFt
	if r := BardicInspirationRefusal(edge); r != "" {
		t.Errorf("60 ft is within range: %q", r)
	}
	for name, mod := range map[string]func(*BardicInspirationTarget){
		"the bard itself":   func(t *BardicInspirationTarget) { t.IsBard = true },
		"beyond 60 ft":      func(t *BardicInspirationTarget) { t.Distance = 65 },
		"cannot hear":       func(t *BardicInspirationTarget) { t.CanHear = false },
		"already has a die": func(t *BardicInspirationTarget) { t.HasDie = true },
	} {
		target := ok
		mod(&target)
		if BardicInspirationRefusal(target) == "" {
			t.Errorf("%s: not refused", name)
		}
	}
	// In the theatre of the mind there is no distance: the master judges it.
	theatre := BardicInspirationTarget{Distance: 500, CanHear: true}
	if r := BardicInspirationRefusal(theatre); r != "" {
		t.Errorf("theatre of the mind: %q", r)
	}
}
