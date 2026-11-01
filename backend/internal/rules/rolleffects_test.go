package rules

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// The critical_range modifier, and the reroll effect of the table's content. The
// names are invented; the engine is generic.

const (
	rollSubclass = "subclass:mestre-da-sorte" + tableSuffix
	rollFirst    = "feature:sorte-um" + tableSuffix
	rollSecond   = "feature:sorte-dois" + tableSuffix
	rollBgKey    = "background:jogador" + tableSuffix
	rollUses     = "viradas_de_sorte"
)

func rollOverlay(first, second []Effect) Overlay {
	return Overlay{Revision: 1, Subclasses: []TableSubclass{{
		Key: rollSubclass, NamePT: "Mestre da Sorte", Class: "class:fighter",
		Levels: []TableSubclassLevel{{Level: 3, Features: []TableFeature{
			{Key: rollFirst, NamePT: "Sorte Um", DescPT: []string{"Primeiro."}, Effects: first},
			{Key: rollSecond, NamePT: "Sorte Dois", DescPT: []string{"Segundo."}, Effects: second},
		}}},
	}}}
}

func rollBuild(level int) Build {
	b := postureBuild(level)
	b.Classes[0].Subclass = rollSubclass
	return b
}

func criticalRangeOf(t *testing.T, level int, first []Effect) int {
	t.Helper()
	return Derive(rollBuild(level), withOverlay(t, rollOverlay(first, nil))).CriticalRange
}

func TestDeriveCriticalRangeFromATableModifier(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		effect Effect
		level  int
		want   int
	}{
		{"a literal 18", Effect{Type: "modifier", Target: CriticalRangeTarget, Mode: "set", Value: "18"}, 3, 18},
		{"the mode defaults to set", Effect{Type: "modifier", Target: CriticalRangeTarget, Value: "19"}, 3, 19},
		{"a formula follows the level", Effect{Type: "modifier", Target: CriticalRangeTarget, Mode: "set", Value: "level() >= 5 ? 18 : 19"}, 5, 18},
		{"a formula below 15 is clamped", Effect{Type: "modifier", Target: CriticalRangeTarget, Mode: "set", Value: "level() - 10"}, 3, 15},
		{"a formula above 20 is clamped", Effect{Type: "modifier", Target: CriticalRangeTarget, Mode: "set", Value: "level() + 20"}, 3, 20},
		{"a false when keeps 20", Effect{Type: "modifier", Target: CriticalRangeTarget, Mode: "set", Value: "18", When: "level() >= 10"}, 3, 20},
		{"the same effect with its when true lowers it (control)", Effect{Type: "modifier", Target: CriticalRangeTarget, Mode: "set", Value: "18", When: "level() >= 2"}, 3, 18},
		{"a true when applies", Effect{Type: "modifier", Target: CriticalRangeTarget, Mode: "set", Value: "18", When: "level() >= 3"}, 3, 18},
		{"a tagged modifier is a hint only", Effect{Type: "modifier", Target: CriticalRangeTarget, Mode: "set", Value: "18", Tags: []string{"against:undead"}}, 3, 20},
		{"the same modifier untagged lowers it (control)", Effect{Type: "modifier", Target: CriticalRangeTarget, Mode: "set", Value: "18"}, 3, 18},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := criticalRangeOf(t, tc.level, []Effect{tc.effect}); got != tc.want {
				t.Errorf("CriticalRange = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestCriticalRangeTakesTheLowestOfEveryModifierAndTheSRDFeature(t *testing.T) {
	t.Parallel()
	// Two table modifiers: the lowest wins, whatever their order.
	c := withOverlay(t, rollOverlay(
		[]Effect{{Type: "modifier", Target: CriticalRangeTarget, Mode: "set", Value: "19"}},
		[]Effect{{Type: "modifier", Target: CriticalRangeTarget, Mode: "set", Value: "17"}}))
	if got := Derive(rollBuild(3), c).CriticalRange; got != 17 {
		t.Errorf("two modifiers: CriticalRange = %d, want 17", got)
	}
	// A champion has Improved Critical (19) at level 3; a table background gives 18: the lowest wins, and
	// a table 19 does not undo the SRD 18 of Superior Critical at level 15.
	bg := TableBackground{
		Key: rollBgKey, NamePT: "Jogador", Skills: []string{"skill:arcana", "skill:history"},
		Feature: TableFeature{
			Key: "background-feature:olho-de-sorte" + tableSuffix, NamePT: "Olho de sorte", DescPT: []string{"Texto."},
			Effects: []Effect{{Type: "modifier", Target: CriticalRangeTarget, Mode: "set", Value: "18"}},
		},
	}
	c = withOverlay(t, Overlay{Revision: 1, Backgrounds: []TableBackground{bg}})
	champion := func(level int) Build {
		b := postureBuild(level)
		b.Classes[0].Subclass = "subclass:champion"
		b.Background = rollBgKey
		return b
	}
	if got := Derive(champion(3), c).CriticalRange; got != 18 {
		t.Errorf("champion 3 with a table 18: CriticalRange = %d, want 18", got)
	}
	bg.Feature.Effects[0].Value = "19"
	c = withOverlay(t, Overlay{Revision: 1, Backgrounds: []TableBackground{bg}})
	if got := Derive(champion(3), c).CriticalRange; got != 19 {
		t.Errorf("champion 3 with a table 19: CriticalRange = %d, want 19 (Improved Critical)", got)
	}
	if got := Derive(champion(15), c).CriticalRange; got != 18 {
		t.Errorf("champion 15 with a table 19: CriticalRange = %d, want 18 (Superior Critical)", got)
	}
	if got := Derive(postureBuild(3), loadForTest(t)).CriticalRange; got != 20 {
		t.Errorf("no feature: CriticalRange = %d, want 20", got)
	}
}

func TestACriticalRangeModifierDoesNotTouchAnyOtherNumber(t *testing.T) {
	t.Parallel()
	plain := Derive(rollBuild(3), withOverlay(t, rollOverlay(nil, nil)))
	with := Derive(rollBuild(3), withOverlay(t, rollOverlay([]Effect{{Type: "modifier", Target: CriticalRangeTarget, Mode: "set", Value: "18"}}, nil)))
	if with.CriticalRange != 18 || plain.CriticalRange != 20 {
		t.Fatalf("critical ranges = %d with, %d without; want 18 and 20", with.CriticalRange, plain.CriticalRange)
	}
	with.CriticalRange, plain.CriticalRange = 0, 0 // everything else of the sheet is the same
	if !reflect.DeepEqual(with, plain) {
		t.Errorf("the modifier changed more than the critical range:\n with %+v\n plain %+v", with, plain)
	}
}

func luck(extra ...Effect) []Effect {
	return append([]Effect{{Type: "resource", Resource: rollUses, Max: "2", Recharge: RechargeLongRest}}, extra...)
}

func TestDeriveExposesTheRerollsInTheOrderOfTheEffects(t *testing.T) {
	t.Parallel()
	c := withOverlay(t, rollOverlay(
		luck(
			Effect{Type: "reroll", Rolls: []string{RollAppliesAttack, RollAppliesSave}, AtMost: 2, Resource: rollUses, Cost: "2"}),
		[]Effect{
			{Type: "reroll", Rolls: []string{RollDamage}, AtMost: 1, When: "level() >= 3"},
		}))
	for range 3 { // the order is stable across derivations
		d := Derive(rollBuild(3), c)
		wantR := []RerollRule{
			{Applies: []string{RollAppliesAttack, RollAppliesSave}, AtMost: 2, Resource: rollUses, Cost: 2, Source: rollFirst},
			{Applies: []string{RollDamage}, AtMost: 1, Cost: 1, Source: rollSecond},
		}
		if !slices.EqualFunc(d.Rerolls, wantR, func(a, b RerollRule) bool {
			return slices.Equal(a.Applies, b.Applies) && a.AtMost == b.AtMost && a.Resource == b.Resource && a.Cost == b.Cost && a.Source == b.Source
		}) {
			t.Fatalf("rerolls = %+v, want %+v", d.Rerolls, wantR)
		}
	}
	if d := Derive(rollBuild(2), c); len(d.Rerolls) != 0 {
		t.Errorf("before the features: %+v, want none", d.Rerolls)
	}
}

func TestARerollWhoseResourceIsMissingIsDropped(t *testing.T) {
	t.Parallel()
	// The resource appears at level 5 only; the free reroll stays.
	c := withOverlay(t, rollOverlay(
		[]Effect{
			{Type: "resource", Resource: rollUses, Max: "level() >= 5 ? 2 : 0", Recharge: RechargeLongRest},
			{Type: "reroll", Rolls: []string{RollAppliesAttack}, AtMost: 1, Resource: rollUses},
			{Type: "reroll", Rolls: []string{RollAppliesSave}, AtMost: 1, When: "level() >= 3"},
		}, nil))
	d := Derive(rollBuild(3), c)
	if len(d.Rerolls) != 1 || d.Rerolls[0].Applies[0] != RollAppliesSave {
		t.Errorf("level 3 rerolls = %+v, want only the free save reroll", d.Rerolls)
	}
	if d := Derive(rollBuild(5), c); len(d.Rerolls) != 2 {
		t.Errorf("level 5 rerolls = %+v, want both", d.Rerolls)
	}
}

func TestRollEffectRefusalsNameTheirField(t *testing.T) {
	t.Parallel()
	const at = "subclasses[0].levels[0].features[0].effects["
	reroll := func(mut func(e *Effect)) Effect {
		e := Effect{Type: "reroll", Rolls: []string{RollAppliesAttack}, AtMost: 2}
		mut(&e)
		return e
	}
	crit := func(mode, value string) Effect {
		return Effect{Type: "modifier", Target: CriticalRangeTarget, Mode: mode, Value: value}
	}
	cases := []struct {
		name          string
		effect        Effect
		field, reason string
	}{
		{"reroll without rolls", reroll(func(e *Effect) { e.Rolls = nil }), ".rolls", ReasonValue},
		{"reroll with an unknown roll", reroll(func(e *Effect) { e.Rolls = []string{RollAppliesAttack, "initiative"} }), ".rolls", ReasonValue},
		{"reroll with a repeated roll", reroll(func(e *Effect) { e.Rolls = []string{RollAppliesSave, RollAppliesSave} }), ".rolls", ReasonValue},
		{"reroll at_most 0", reroll(func(e *Effect) { e.AtMost = 0 }), ".at_most", ReasonValue},
		{"reroll at_most 21", reroll(func(e *Effect) { e.AtMost = 21 }), ".at_most", ReasonValue},
		{"reroll cost 0", reroll(func(e *Effect) { e.Resource, e.Cost = rollUses, "0" }), ".cost", ReasonValue},
		{"reroll cost without a resource", reroll(func(e *Effect) { e.Cost = "1" }), ".cost", ReasonValue},
		{"reroll cost that does not compile", reroll(func(e *Effect) { e.Resource, e.Cost = rollUses, "prof(" }), ".cost", ReasonFormula},
		{"reroll with an unknown resource", reroll(func(e *Effect) { e.Resource = "fantasma" }), ".resource", ReasonReference},
		{"reroll with a tag", reroll(func(e *Effect) { e.Tags = []string{"against:magic"} }), ".tags", ReasonValue},
		{"reroll with a value", reroll(func(e *Effect) { e.Value = "2" }), ".value", ReasonValue},
		{"reroll with a stray economy", reroll(func(e *Effect) { e.Economy = "reaction" }), ".economy", ReasonValue},
		{"reroll with a stray applies", reroll(func(e *Effect) { e.Applies = "attack" }), ".applies", ReasonValue},
		{"critical_range 14", crit("set", "14"), ".value", ReasonValue},
		{"critical_range 21", crit("set", "21"), ".value", ReasonValue},
		{"critical_range with mode add", crit("add", "-1"), ".mode", ReasonValue},
		{"critical_range with mode max", crit("max", "18"), ".mode", ReasonValue},
		{"critical_range without a value", crit("set", ""), ".value", ReasonValue},
		{"rolls on another type", Effect{Type: "ambush", Applies: AmbushUnactedAdvantage, Rolls: []string{RollAppliesAttack}}, ".rolls", ReasonValue},
		{"at_most on another type", Effect{Type: "ambush", Applies: AmbushUnactedAdvantage, AtMost: 3}, ".at_most", ReasonValue},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			o := rollOverlay(luck(tc.effect), nil)
			o.Strict = []string{rollSubclass}
			_, err := loadForTest(t).With(o)
			var oe *OverlayError
			if !errors.As(err, &oe) || oe.Field != at+"1]"+tc.field || oe.Reason != tc.reason {
				t.Errorf("error = %v, want field %s1]%s reason %s", err, at, tc.field, tc.reason)
			}
		})
	}
}

func TestRollEffectsAreAcceptedWhenRight(t *testing.T) {
	t.Parallel()
	for _, e := range []Effect{
		{Type: "reroll", Rolls: []string{RollAppliesAttack, RollAppliesCheck, RollAppliesSave, RollDamage}, AtMost: 20},
		{Type: "reroll", Rolls: []string{RollDamage}, AtMost: 1},
		{Type: "reroll", Rolls: []string{RollAppliesAttack}, AtMost: 2, Resource: rollUses},
		{Type: "reroll", Rolls: []string{RollAppliesAttack}, AtMost: 2, Resource: rollUses, Cost: "prof()", TextPT: "Role de novo.", When: "level() >= 3"},
		{Type: "modifier", Target: CriticalRangeTarget, Mode: "set", Value: "15"},
		{Type: "modifier", Target: CriticalRangeTarget, Value: "20"},
		{Type: "modifier", Target: CriticalRangeTarget, Mode: "set", Value: "level() >= 5 ? 18 : 19"},
	} {
		o := rollOverlay(luck(e), nil)
		o.Strict = []string{rollSubclass}
		if _, err := loadForTest(t).With(o); err != nil {
			t.Errorf("%+v refused: %v", e, err)
		}
	}
}

func TestRollEffectsAreOnTheMenu(t *testing.T) {
	t.Parallel()
	m := loadForTest(t).EffectMenu()
	for typ, name := range map[string]string{"reroll": "Rolar de novo"} {
		i := slices.IndexFunc(m.Types, func(mt MenuType) bool { return mt.Type == typ })
		if i < 0 || m.Types[i].NamePT != name {
			t.Errorf("menu type %s = %+v, want the name %q", typ, m.Types, name)
		}
	}
	j := slices.IndexFunc(m.Lists, func(l MenuList) bool { return l.Name == ListRollApplies })
	if j < 0 || len(m.Lists[j].Values) != len(rollApplies) {
		t.Errorf("roll_applies list = %+v", m.Lists)
	}
	fields := func(typ string) (names []string) {
		for _, f := range m.Types[slices.IndexFunc(m.Types, func(mt MenuType) bool { return mt.Type == typ })].Fields {
			names = append(names, f.Name)
		}
		return names
	}
	if got, want := fields("reroll"), []string{"rolls", "at_most", "resource", "cost", "when", "text_pt"}; !slices.Equal(got, want) {
		t.Errorf("reroll fields = %v, want %v", got, want)
	}
	for _, mt := range m.Types {
		if mt.Type == "reroll" && !strings.Contains(mt.HintPT, "dado") {
			t.Errorf("%s hint = %q, want a Portuguese explanation", mt.Type, mt.HintPT)
		}
	}
	k := slices.IndexFunc(m.Lists, func(l MenuList) bool { return l.Name == ListModifierTargets })
	if k < 0 || !slices.ContainsFunc(m.Lists[k].Values, func(v MenuValue) bool {
		return v.Key == CriticalRangeTarget && v.NamePT != "" && v.HintPT != "" && slices.Equal(v.OnlyModes, []string{"set"}) && v.Min == 15 && v.Max == 20
	}) {
		t.Errorf("the modifier targets lack %s", CriticalRangeTarget)
	}
}

func TestARerollThatCostsMoreThanTheResourceHasIsDropped(t *testing.T) {
	t.Parallel()
	// The resource has 2 uses at level 3 and 4 from level 5: a cost of 3 can be paid only from level 5.
	c := withOverlay(t, rollOverlay([]Effect{
		{Type: "resource", Resource: rollUses, Max: "level() >= 5 ? 4 : 2", Recharge: RechargeLongRest},
		{Type: "reroll", Rolls: []string{RollAppliesAttack}, AtMost: 1, Resource: rollUses, Cost: "3"},
		{Type: "reroll", Rolls: []string{RollAppliesSave}, AtMost: 1, Resource: rollUses, Cost: "2"},
	}, nil))
	if d := Derive(rollBuild(3), c); len(d.Rerolls) != 1 || d.Rerolls[0].Applies[0] != RollAppliesSave {
		t.Errorf("level 3 rerolls = %+v, want only the one that costs 2", d.Rerolls)
	}
	if d := Derive(rollBuild(5), c); len(d.Rerolls) != 2 {
		t.Errorf("level 5 rerolls = %+v, want both", d.Rerolls)
	}
}

// The engine-load path (the SRD content and stored content) checks the effects with the same rules as the
// table's overlay, without the overlay's field paths.
func TestRollEffectsThroughTheEngineLoad(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)
	bad := map[string]*Effect{
		"reroll without rolls":         {Type: "reroll", AtMost: 2},
		"reroll with an unknown roll":  {Type: "reroll", Rolls: []string{"initiative"}, AtMost: 2},
		"reroll with a repeated roll":  {Type: "reroll", Rolls: []string{RollDamage, RollDamage}, AtMost: 2},
		"reroll at_most 0":             {Type: "reroll", Rolls: []string{RollDamage}},
		"reroll at_most 21":            {Type: "reroll", Rolls: []string{RollDamage}, AtMost: 21},
		"reroll cost 0":                {Type: "reroll", Rolls: []string{RollDamage}, AtMost: 2, Resource: "r", Cost: "0"},
		"reroll cost without resource": {Type: "reroll", Rolls: []string{RollDamage}, AtMost: 2, Cost: "1"},
		"reroll cost that fails":       {Type: "reroll", Rolls: []string{RollDamage}, AtMost: 2, Resource: "r", Cost: "prof("},
		"reroll with a value":          {Type: "reroll", Rolls: []string{RollDamage}, AtMost: 2, Value: "2"},
		"reroll with a tag":            {Type: "reroll", Rolls: []string{RollDamage}, AtMost: 2, Tags: []string{"against:magic"}},
		"rolls on an ambush":           {Type: "ambush", Applies: AmbushUnactedAdvantage, Rolls: []string{RollDamage}},
		"critical_range 14":            {Type: "modifier", Target: CriticalRangeTarget, Mode: "set", Value: "14"},
		"critical_range 21":            {Type: "modifier", Target: CriticalRangeTarget, Mode: "set", Value: "21"},
		"critical_range add":           {Type: "modifier", Target: CriticalRangeTarget, Mode: "add", Value: "-1"},
	}
	for name, e := range bad {
		if err := srd.c.compileEffect("feature:x", e); err == nil {
			t.Errorf("%s: the loader accepted it", name)
		}
	}
	for name, e := range map[string]*Effect{
		"a reroll":             {Type: "reroll", Rolls: []string{RollAppliesAttack, RollDamage}, AtMost: 2},
		"a reroll with a cost": {Type: "reroll", Rolls: []string{RollAppliesSave}, AtMost: 1, Resource: "r", Cost: "2"},
		"a critical range":     {Type: "modifier", Target: CriticalRangeTarget, Mode: "set", Value: "18"},
		"a critical formula":   {Type: "modifier", Target: CriticalRangeTarget, Value: "level() >= 5 ? 18 : 19"},
	} {
		if err := srd.c.compileEffect("feature:x", e); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}
