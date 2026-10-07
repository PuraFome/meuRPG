package rules

import (
	"slices"
	"strconv"
	"strings"
	"testing"
)

func menuList(t *testing.T, m EffectMenu, name string) []MenuValue {
	t.Helper()
	for _, l := range m.Lists {
		if l.Name == name {
			return l.Values
		}
	}
	t.Fatalf("the menu has no list %q", name)
	return nil
}

func valueKeys(vs []MenuValue) []string {
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = v.Key
	}
	return out
}

// classWithEffects is a table class that does not cast whose features carry the
// effects, 15 to a feature, so a test can offer the whole menu to With at once.
func classWithEffects(effects []Effect) TableClass {
	tc := classFromDefaults(DefaultTable{Kind: CastingNone, Rows: loadDefaultRows()}, "cardapio")
	for i := range tc.Levels {
		tc.Levels[i].Features = nil
	}
	// A feature offers one choice of options, so each of those has a feature of
	// its own.
	var groups [][]Effect
	var plain []Effect
	for _, e := range effects {
		if e.Type == "choice" && e.Choice == "feature" {
			groups = append(groups, []Effect{e})
			continue
		}
		plain = append(plain, e)
	}
	for len(plain) > 0 {
		n := min(15, len(plain))
		groups, plain = append(groups, plain[:n]), plain[n:]
	}
	for n, g := range groups {
		row := &tc.Levels[n%MaxLevel]
		row.Features = append(row.Features, TableFeature{
			Key: "feature:cardapio-" + strconv.Itoa(n) + tableSuffix, NamePT: "Cardápio " + strconv.Itoa(n), Effects: g,
		})
	}
	return tc
}

// loadDefaultRows are 20 plain rows (no casting).
func loadDefaultRows() []DefaultRow {
	rows := make([]DefaultRow, MaxLevel)
	for i := range rows {
		rows[i] = DefaultRow{Level: i + 1, ProfBonus: 2 + i/4}
	}
	return rows
}

// TestEffectMenuIsWhatTheValidatorAccepts: the menu's types are the table's
// (overlayEffectTypes), their fields are the ones the validator reads, the
// required ones are refused when missing, and every value of every closed list is
// accepted by With, so the editor offers nothing the server will refuse.
func TestEffectMenuIsWhatTheValidatorAccepts(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)
	m := srd.EffectMenu()

	var types []string
	for _, mt := range m.Types {
		types = append(types, mt.Type)
		if mt.NamePT == "" || mt.HintPT == "" {
			t.Errorf("type %s has no Portuguese name or hint", mt.Type)
		}
		var own []string
		for _, f := range mt.Fields {
			if f.Name != "when" && f.Name != "tags" && f.Name != "text_pt" {
				own = append(own, f.Name)
			}
			if f.List != "" {
				menuList(t, m, f.List) // the list exists
			}
		}
		want := slices.Clone(ownFields[mt.Type])
		slices.Sort(want)
		slices.Sort(own)
		if !slices.Equal(own, want) {
			t.Errorf("type %s: menu fields %v, validator reads %v", mt.Type, own, want)
		}
	}
	if !slices.Equal(types, overlayEffectTypes) {
		t.Errorf("menu types = %v, the table's menu is %v", types, overlayEffectTypes)
	}
	for _, l := range m.Lists {
		if len(l.Values) == 0 {
			t.Errorf("list %s is empty", l.Name)
		}
		seen := map[string]bool{}
		for _, v := range l.Values {
			if v.Key == "" || v.NamePT == "" {
				t.Errorf("list %s has a value without a key or a Portuguese name: %+v", l.Name, v)
			}
			if seen[v.Key] {
				t.Errorf("list %s repeats %q", l.Name, v.Key)
			}
			seen[v.Key] = true
		}
	}

	// The closed sets are the engine's.
	for name, want := range map[string][]string{
		ListModifierModes: modifierModes, ListProficiencyLevels: proficiencyLevels, ListRollModes: rollModes,
		ListSenses: senses, ListRecharges: recharges, ListEconomies: economies, ListChoiceKinds: overlayChoiceKinds,
	} {
		if got := valueKeys(menuList(t, m, name)); !slices.Equal(got, want) {
			t.Errorf("list %s = %v, want %v", name, got, want)
		}
	}

	// Every value of every list is accepted, in the field it goes in.
	var effects []Effect
	for _, v := range menuList(t, m, ListModifierTargets) {
		effects = append(effects, Effect{Type: "modifier", Target: v.Key, Mode: "add", Value: "1"})
	}
	for _, v := range menuList(t, m, ListModifierModes) {
		effects = append(effects, Effect{Type: "modifier", Target: "ac", Mode: v.Key, Value: "1"})
	}
	for _, v := range menuList(t, m, ListProficiencies) {
		effects = append(effects, Effect{Type: "proficiency", Proficiency: v.Key})
	}
	for _, v := range menuList(t, m, ListProficiencyLevels) {
		effects = append(effects, Effect{Type: "proficiency", Proficiency: "skill:arcana", Level: v.Key})
	}
	for _, v := range menuList(t, m, ListRollModes) {
		effects = append(effects, Effect{Type: "roll_mode", Roll: v.Key, Targets: []string{"attack"}})
	}
	for _, v := range menuList(t, m, ListRollTargets) {
		effects = append(effects, Effect{Type: "roll_mode", Roll: "advantage", Targets: []string{v.Key}})
	}
	for _, v := range menuList(t, m, ListSenses) {
		effects = append(effects, Effect{Type: "sense", Sense: v.Key, RangeFt: 30})
	}
	for i, v := range menuList(t, m, ListRecharges) {
		effects = append(effects, Effect{Type: "resource", Resource: "uso_" + strconv.Itoa(i), Max: "1", Recharge: v.Key})
	}
	for _, v := range menuList(t, m, ListEconomies) {
		effects = append(effects, Effect{Type: "grant_action", Economy: v.Key})
	}
	for _, v := range menuList(t, m, ListSkills) {
		effects = append(effects, Effect{Type: "choice", Choice: "skill", Count: 1, From: []string{v.Key}})
	}
	for _, v := range menuList(t, m, ListLanguages) {
		effects = append(effects, Effect{Type: "choice", Choice: "language", Count: 1, From: []string{v.Key}})
	}
	for _, v := range menuList(t, m, ListTools) {
		effects = append(effects, Effect{Type: "choice", Choice: "tool", Count: 1, From: []string{v.Key}})
	}
	for _, kind := range menuList(t, m, ListChoiceKinds) {
		effects = append(effects, Effect{Type: "choice", Choice: kind.Key, Count: 1, From: nil})
		if kind.Key == "feature" {
			effects = effects[:len(effects)-1] // a feature choice needs its options (below)
		}
	}
	for n := m.ExtraAttackMin; n <= m.ExtraAttackMax; n++ {
		effects = append(effects, Effect{Type: "extra_attack", Count: n})
	}
	// The option sets: each option of each set, offered by a feature choice.
	if len(m.OptionSets) == 0 {
		t.Fatal("the menu has no option set (fighting styles...)")
	}
	for _, set := range m.OptionSets {
		if set.NamePT == "" || set.Choose < 0 || len(set.Options) == 0 {
			t.Errorf("option set %+v is incomplete", set)
		}
		for _, o := range set.Options {
			if o.NamePT == "" {
				t.Errorf("option %s of %s has no name", o.Key, set.Key)
			}
		}
		for keys := valueKeys(set.Options); len(keys) > 0; {
			n := min(len(keys), maxEffectList)
			effects = append(effects, Effect{Type: "choice", Choice: "feature", Count: 1, From: keys[:n]})
			keys = keys[n:]
		}
	}
	// The tags: a prefix and something after it.
	for _, p := range menuList(t, m, ListTagPrefixes) {
		effects = append(effects, Effect{Type: "roll_mode", Roll: "advantage", Targets: []string{"attack"}, Tags: []string{p.Key + "teste"}})
	}
	if len(effects) < 100 {
		t.Fatalf("only %d effects built from the menu", len(effects))
	}
	o := Overlay{Revision: 1}
	// The same effect is only worth offering once; the budget counts the distinct
	// formulas, which here are "1".
	o.Classes = append(o.Classes, classWithEffects(effects))
	if _, err := srd.With(o); err != nil {
		t.Fatalf("the menu offers what With refuses: %v", err)
	}
}

// TestEffectMenuRequiredFieldsAreRequired: each field the menu marks required is
// refused when empty, and the effect is accepted with it.
func TestEffectMenuRequiredFieldsAreRequired(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)
	m := srd.EffectMenu()
	valid := map[string]Effect{
		"modifier":     {Type: "modifier", Target: "ac", Mode: "add", Value: "1"},
		"proficiency":  {Type: "proficiency", Proficiency: "skill:arcana"},
		"resource":     {Type: "resource", Resource: "surto", Max: "1", Recharge: "long_rest"},
		"sense":        {Type: "sense", Sense: "darkvision", RangeFt: 30},
		"roll_mode":    {Type: "roll_mode", Roll: "advantage", Targets: []string{"attack"}},
		"grant_action": {Type: "grant_action", Economy: "action"},
		"extra_attack": {Type: "extra_attack", Count: 2},
		"choice":       {Type: "choice", Choice: "skill", Count: 1},
		"note":         {Type: "note"},
	}
	clearField := func(e *Effect, field string) {
		switch field {
		case "target":
			e.Target = ""
		case "mode":
			e.Mode = ""
		case "value":
			e.Value = ""
		case "proficiency":
			e.Proficiency = ""
		case "resource":
			e.Resource = ""
		case "max":
			e.Max = ""
		case "recharge":
			e.Recharge = ""
		case "sense":
			e.Sense = ""
		case "range_ft":
			e.RangeFt = 0
		case "roll":
			e.Roll = ""
		case "targets":
			e.Targets = nil
		case "economy":
			e.Economy = ""
		case "count":
			e.Count = 0
		case "choice":
			e.Choice = ""
		default:
			t.Fatalf("no way to clear %q", field)
		}
	}
	with := func(e Effect) error {
		_, err := srd.With(Overlay{Revision: 1, Classes: []TableClass{classWithEffects([]Effect{e})}})
		return err
	}
	for _, mt := range m.Types {
		e := valid[mt.Type]
		if err := with(e); err != nil {
			t.Fatalf("the valid %s effect is refused: %v", mt.Type, err)
		}
		for _, f := range mt.Fields {
			if !f.Required {
				continue
			}
			broken := e
			clearField(&broken, f.Name)
			if with(broken) == nil {
				t.Errorf("a %s effect without %s is accepted, the menu says it is required", mt.Type, f.Name)
			}
		}
	}
	// Bounds the menu states.
	for _, mt := range m.Types {
		for _, f := range mt.Fields {
			if f.Name == "count" && mt.Type == "extra_attack" && (f.Min != m.ExtraAttackMin || f.Max != m.ExtraAttackMax) {
				t.Errorf("extra_attack count bounds = %d..%d, menu says %d..%d", f.Min, f.Max, m.ExtraAttackMin, m.ExtraAttackMax)
			}
		}
	}
	for _, n := range []int{m.ExtraAttackMin - 1, m.ExtraAttackMax + 1} {
		if with(Effect{Type: "extra_attack", Count: n}) == nil {
			t.Errorf("extra_attack with %d attacks is accepted", n)
		}
	}
}

// TestEffectMenuFormulaHelpers: each helper the menu lists compiles where it is
// meant to, with the table's class indexes; one the menu does not list does not.
func TestEffectMenuFormulaHelpers(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)
	o := genOverlay(t, srd)
	c := withOverlayOn(t, srd, o)
	m := c.EffectMenu()

	samples := map[string]string{
		"level()": "level()", `classLevel("<classe>")`: `classLevel("wizard")`, `mod("<habilidade>")`: `mod("int")`,
		`score("<habilidade>")`: `score("dex")`, "prof()": "prof()", "floor(x)": `floor(level() / 2)`, "ceil(x)": `ceil(level() / 3)`,
		"min(a, b)": "min(level(), 5)", "max(a, b)": "max(level(), 1)",
	}
	conditions := map[string]string{"armor()": `armor() == "none"`, "shield()": "shield()"}
	for _, h := range m.Helpers {
		if h.HintPT == "" {
			t.Errorf("helper %s has no hint", h.Call)
		}
		var e Effect
		switch {
		case samples[h.Call] != "":
			e = Effect{Type: "modifier", Target: "ac", Mode: "add", Value: samples[h.Call]}
		case conditions[h.Call] != "":
			e = Effect{Type: "modifier", Target: "ac", Mode: "add", Value: "1", When: conditions[h.Call]}
		default:
			t.Errorf("no sample for helper %s", h.Call)
			continue
		}
		if _, err := srd.With(Overlay{Revision: 1, Classes: []TableClass{classWithEffects([]Effect{e})}}); err != nil {
			t.Errorf("helper %s is refused: %v", h.Call, err)
		}
	}
	if _, err := srd.With(Overlay{Revision: 1, Classes: []TableClass{classWithEffects([]Effect{{Type: "modifier", Target: "ac", Mode: "add", Value: "random()"}})}}); err == nil {
		t.Error("a helper the menu does not list is accepted")
	}

	// classLevel takes the indexes the menu lists, the table's included.
	for _, idx := range []string{"wizard", "gen-full-prepared@mesa", "fighter"} {
		if !slices.Contains(m.ClassIndexes, idx) {
			t.Errorf("class index %q is not in the menu: %v", idx, m.ClassIndexes)
		}
	}
	if slices.Contains(srd.EffectMenu().ClassIndexes, "gen-full-prepared@mesa") {
		t.Error("the SRD's menu lists a table class")
	}
	if m.MaxFeaturesPerClass != MaxTableFeatures || m.MaxEffectsPerFeature != MaxFeatureEffects {
		t.Errorf("limits = %d, %d", m.MaxFeaturesPerClass, m.MaxEffectsPerFeature)
	}
}

// TestEffectMenuNamesArePortuguese: every value of every closed list and every
// option set has a name the content translated, never the English fallback.
func TestEffectMenuNamesArePortuguese(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)
	m := srd.EffectMenu()
	missing := map[string]bool{}
	check := func(list, key string) {
		// The menu's own words (modes, senses, tag prefixes...) are not content keys.
		if strings.ContainsAny(key, ".*") || !strings.Contains(key, ":") || strings.HasSuffix(key, ":") {
			return
		}
		_, own := srd.c.namesPT[key]
		viaRef := false
		if p := srd.c.proficiencies[key]; p != nil && len(p.Refs) == 1 {
			_, viaRef = srd.c.namesPT[p.Refs[0]]
		}
		if !own && !viaRef {
			missing[list+" "+key] = true
		}
	}
	for _, l := range m.Lists {
		for _, v := range l.Values {
			check(l.Name, v.Key)
		}
	}
	setNames := map[string]string{}
	for _, set := range m.OptionSets {
		if other, dup := setNames[set.NamePT]; dup {
			t.Errorf("option sets %s and %s are both called %q", other, set.Key, set.NamePT)
		}
		setNames[set.NamePT] = set.Key
		check("option set", set.Key)
		for _, o := range set.Options {
			check("option of "+set.Key, o.Key)
		}
	}
	if len(missing) > 0 {
		t.Errorf("%d menu values with no Portuguese name: %v", len(missing), sortedKeys(missing))
	}
}
