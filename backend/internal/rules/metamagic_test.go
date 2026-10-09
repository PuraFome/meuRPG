package rules

import (
	"errors"
	"slices"
	"testing"
)

func metamagicByKey(opts []MetamagicOption, key string) MetamagicOption {
	for _, o := range opts {
		if o.Key == key {
			return o
		}
	}
	return MetamagicOption{}
}

func TestMetamagicCostsAreTheSRDs(t *testing.T) {
	t.Parallel()
	flat := map[string]int{
		MetamagicCareful: 1, MetamagicDistant: 1, MetamagicEmpowered: 1, MetamagicExtended: 1,
		MetamagicHeightened: 3, MetamagicQuickened: 2, MetamagicSubtle: 1,
	}
	for key, want := range flat {
		for _, level := range []int{0, 1, 5, 9} {
			if got := MetamagicCost(key, level); got != want {
				t.Errorf("%s on a level %d spell costs %d, want %d", key, level, got, want)
			}
		}
	}
	// Twinned Spell costs the spell's level, 1 for a cantrip.
	for level, want := range map[int]int{0: 1, 1: 1, 3: 3, 9: 9} {
		if got := MetamagicCost(MetamagicTwinned, level); got != want {
			t.Errorf("Twinned Spell on a level %d spell costs %d, want %d", level, got, want)
		}
	}
	if MetamagicCost("feature:nothing", 3) != 0 {
		t.Error("an unknown option costs nothing")
	}
}

func TestMetamagicKnownListsOnlyTheOptionsTheSheetHas(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	b := standard("class:sorcerer", 5)
	b.FeatureChoices = []string{MetamagicTwinned, MetamagicCareful}
	d := Derive(b, c)
	// Twinned Spell is listed by Metamagic at levels 3, 10 and 17: the sheet takes it at level 3.
	for _, is := range d.Issues {
		if is.Code == IssueUnknownKey {
			t.Errorf("a level 5 sorcerer cannot take the options: %s", is.Message)
		}
	}
	got := MetamagicKnown(d.Features)
	if !slices.Equal(got, []string{MetamagicCareful, MetamagicTwinned}) {
		t.Errorf("known = %v, want Careful and Twinned", got)
	}
	if len(MetamagicKnown(Derive(standard("class:wizard", 5), c).Features)) != 0 {
		t.Error("a wizard knows no Metamagic")
	}
}

func TestMetamagicEligibilityOfRealSpells(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	all := []string{MetamagicCareful, MetamagicDistant, MetamagicEmpowered, MetamagicExtended, MetamagicHeightened, MetamagicQuickened, MetamagicSubtle, MetamagicTwinned}
	type want map[string]bool
	cases := []struct {
		spell string
		slot  int
		want  want
	}{
		// Ray of frost: one target, a spell attack, 60 ft, an instant, 1 action: the SRD's own example for Twinned.
		{"spell:ray-of-frost", 0, want{MetamagicCareful: false, MetamagicDistant: true, MetamagicEmpowered: true, MetamagicExtended: false, MetamagicHeightened: false, MetamagicQuickened: true, MetamagicSubtle: true, MetamagicTwinned: true}},
		// Fireball: an area with a saving throw.
		{"spell:fireball", 3, want{MetamagicCareful: true, MetamagicDistant: true, MetamagicEmpowered: true, MetamagicExtended: false, MetamagicHeightened: true, MetamagicQuickened: true, MetamagicSubtle: true, MetamagicTwinned: false}},
		// Magic missile and scorching ray are the SRD's examples of spells Twinned cannot take.
		{"spell:magic-missile", 1, want{MetamagicTwinned: false}},
		{"spell:scorching-ray", 2, want{MetamagicTwinned: false}},
		// Shield has a range of self.
		{"spell:shield", 1, want{MetamagicDistant: false, MetamagicTwinned: false, MetamagicQuickened: false}},
		// Cure wounds is touch: Distant makes it 30 ft, and Twinned takes the level.
		{"spell:cure-wounds", 1, want{MetamagicDistant: true, MetamagicTwinned: true, MetamagicEmpowered: false}},
		// Mage armor lasts 8 hours.
		{"spell:mage-armor", 1, want{MetamagicExtended: true, MetamagicTwinned: true}},
		// A spell with a cast time of 1 minute is not Quickened.
		{"spell:find-familiar", 1, want{MetamagicQuickened: false}},
	}
	for _, tc := range cases {
		opts := c.MetamagicOptions(all, tc.spell, tc.slot)
		if len(opts) != len(all) {
			t.Fatalf("%s: %d options, want %d", tc.spell, len(opts), len(all))
		}
		for key, allowed := range tc.want {
			o := metamagicByKey(opts, key)
			if o.Allowed != allowed {
				t.Errorf("%s with %s: allowed = %v (%q), want %v", tc.spell, key, o.Allowed, o.ReasonPT, allowed)
			}
			if !o.Allowed && o.ReasonPT == "" {
				t.Errorf("%s with %s: refused without a reason", tc.spell, key)
			}
			if o.Allowed && o.ReasonPT != "" {
				t.Errorf("%s with %s: allowed with a reason %q", tc.spell, key, o.ReasonPT)
			}
		}
	}
}

func TestMetamagicOptionsCarryCostNameAndSummary(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	opts := c.MetamagicOptions([]string{MetamagicTwinned, MetamagicCareful}, "spell:fireball", 3)
	if len(opts) != 2 || opts[0].Key != MetamagicCareful || opts[1].Key != MetamagicTwinned {
		t.Fatalf("options = %+v", opts)
	}
	if opts[0].NamePT != "Magia Cuidadosa" || opts[0].Cost != 1 || opts[0].SummaryPT == "" {
		t.Errorf("careful = %+v", opts[0])
	}
	if opts[1].NamePT != "Magia Duplicada" || opts[1].Cost != 3 {
		t.Errorf("twinned on a fireball costs the spell's level: %+v", opts[1])
	}
	if opts[1].Allowed || opts[1].ReasonPT != "Bola de Fogo atinge uma área: não tem “um só alvo”." {
		t.Errorf("twinned refusal = %q", opts[1].ReasonPT)
	}
	if got := c.MetamagicOptions([]string{MetamagicTwinned}, "spell:nothing", 1); got != nil {
		t.Errorf("an unknown spell has no options: %v", got)
	}
}

func TestACantripTwinnedCostsOnePoint(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	o := metamagicByKey(c.MetamagicOptions([]string{MetamagicTwinned}, "spell:ray-of-frost", 0), MetamagicTwinned)
	if !o.Allowed || o.Cost != 1 {
		t.Errorf("twinned ray of frost = %+v, want allowed for 1 point", o)
	}
}

func TestTwinnedStopsAtTheSlotWhereTheSpellTakesMoreTargets(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	// Hold person takes one more creature for each slot level above the 2nd: at the
	// 2nd level it is a single-target spell, above it is not ("incapable of targeting
	// more than one creature at the spell's current level").
	d, ok := c.SpellDetails("spell:hold-person")
	if !ok {
		t.Skip("no hold person")
	}
	at2 := metamagicByKey(c.MetamagicOptions([]string{MetamagicTwinned}, "spell:hold-person", 2), MetamagicTwinned)
	at3 := metamagicByKey(c.MetamagicOptions([]string{MetamagicTwinned}, "spell:hold-person", 3), MetamagicTwinned)
	if d.Target.PerSlotLevel > 0 && (!at2.Allowed || at3.Allowed) {
		t.Errorf("hold person: at the 2nd level %v, at the 3rd %v", at2.Allowed, at3.Allowed)
	}
}

func TestCheckMetamagicChoice(t *testing.T) {
	t.Parallel()
	known := []string{MetamagicCareful, MetamagicEmpowered, MetamagicTwinned}
	for name, tc := range map[string]struct {
		chosen []string
		ok     bool
	}{
		"none":                     {nil, true},
		"one":                      {[]string{MetamagicTwinned}, true},
		"empowered with another":   {[]string{MetamagicEmpowered, MetamagicTwinned}, true},
		"two others":               {[]string{MetamagicCareful, MetamagicTwinned}, false},
		"three":                    {[]string{MetamagicCareful, MetamagicEmpowered, MetamagicTwinned}, false},
		"the same twice":           {[]string{MetamagicTwinned, MetamagicTwinned}, false},
		"an option not known":      {[]string{MetamagicSubtle}, false},
		"empowered and an unknown": {[]string{MetamagicEmpowered, MetamagicSubtle}, false},
	} {
		err := CheckMetamagicChoice(tc.chosen, known)
		if (err == nil) != tc.ok || (err != nil && !errors.Is(err, ErrMetamagic)) {
			t.Errorf("%s: err = %v, want ok=%v", name, err, tc.ok)
		}
	}
}

func TestMetamagicNumbersFollowTheCharismaModifier(t *testing.T) {
	t.Parallel()
	for mod, want := range map[int]int{-1: 1, 0: 1, 1: 1, 3: 3, 5: 5} {
		if CarefulCreatures(mod) != want || EmpoweredDice(mod) != want {
			t.Errorf("Charisma %+d: careful %d, empowered %d, want %d", mod, CarefulCreatures(mod), EmpoweredDice(mod), want)
		}
	}
	if DistantRangeFt(RangeRanged, 60) != 120 || DistantRangeFt(RangeTouch, 0) != 30 {
		t.Error("Distant doubles a range and makes touch 30 ft")
	}
}
