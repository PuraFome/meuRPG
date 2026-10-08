package rules

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// tableFeats is a table with the feats the tests below use: one for each kind of
// prerequisite, a half feat (+1 to one of two abilities) and a feat with no
// prerequisite and no ability increase.
func tableFeats(t *testing.T) *Content {
	t.Helper()
	feat := func(slug, name string, p FeatPrerequisite, effects ...Effect) TableFeat {
		return TableFeat{
			TableEntry: TableEntry{Key: "feat:" + slug + tableSuffix, NamePT: name}, DescPT: []string{"Texto de " + name + "."},
			Prerequisite: p, Effects: effects,
		}
	}
	half := Effect{Type: "ability_increase", Count: 1, From: []string{"str", "dex"}, Value: "1"}
	c, err := loadForTest(t).With(Overlay{Revision: 1, Feats: []TableFeat{
		feat("livre", "Livre", FeatPrerequisite{}),
		feat("forte", "Forte", FeatPrerequisite{Minimums: map[Ability]int{STR: 13, CON: 13}}),
		feat("agil-ou-esperto", "Ágil ou esperto", FeatPrerequisite{AnyOf: map[Ability]int{DEX: 15, INT: 15}}),
		feat("armadura-media", "Armadura média", FeatPrerequisite{Proficiency: "proficiency:medium-armor"}),
		feat("conjurador", "Conjurador", FeatPrerequisite{Spellcasting: true}),
		feat("humano", "Humano", FeatPrerequisite{Race: "race:human"}),
		feat("veterano", "Veterano", FeatPrerequisite{Level: 5}),
		feat("meio-talento", "Meio talento", FeatPrerequisite{}, half),
	}})
	if err != nil {
		t.Fatalf("With() error = %v", err)
	}
	return c
}

func unmetKinds(un []FeatUnmet) []string {
	var out []string
	for _, u := range un {
		out = append(out, u.Kind)
	}
	return out
}

func featOption(t *testing.T, opts []FeatOption, key string) FeatOption {
	t.Helper()
	for _, o := range opts {
		if o.Key == key {
			return o
		}
	}
	t.Fatalf("no feat option %s in %d options", key, len(opts))
	return FeatOption{}
}

// TestGrapplerIsTheSRDFeat: the SRD's one feat comes with the SRD's prerequisite
// (Strength 13), its Portuguese name, and its effects reach the sheet once taken.
func TestGrapplerIsTheSRDFeat(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	e, ok := c.Feat("feat:grappler")
	if !ok {
		t.Fatal("the SRD content has no Grappler")
	}
	if e.NamePT != "Agarrador" || e.Name != "Grappler" || e.Prerequisite.Minimums[STR] != 13 || e.Table {
		t.Errorf("Grappler = %+v", e)
	}
	b := torenLevelUp()
	b.Feats = []string{"feat:grappler"}
	d := Derive(b, c)
	var feature *Feature
	for i := range d.Features {
		if d.Features[i].Key == "feat:grappler" {
			feature = &d.Features[i]
		}
	}
	if feature == nil || feature.SourcePT != "Talento" {
		t.Fatalf("the sheet does not list the feat as a feature: %+v", feature)
	}
	hinted := 0
	for _, h := range d.Hints {
		if h.Source == "feat:grappler" {
			hinted++
		}
	}
	if hinted != 2 {
		t.Errorf("Grappler gives %d hints, want 2 (advantage against a grappled creature, and the pin)", hinted)
	}
	if issues := Validate(b, c); issues != nil {
		t.Errorf("Validate() = %v", issues)
	}
	b.Feats = []string{"feat:nope"}
	if err := Validate(b, c); err == nil {
		t.Error("an unknown feat key is accepted")
	}
}

// TestFeatOptionsCheckEveryKindOfPrerequisite: each prerequisite the table can ask
// is judged on the sheet, and an option says what the character lacks.
func TestFeatOptionsCheckEveryKindOfPrerequisite(t *testing.T) {
	t.Parallel()
	c := tableFeats(t)
	toren := torenLevelUp() // human Champion 3, STR 16, DEX 13, CON 14, INT 8; medium armor proficiency

	opts := FeatOptions(toren, c)
	want := map[string][]string{
		"feat:livre":           nil,
		"feat:forte":           nil,
		"feat:agil-ou-esperto": {FeatUnmetAbilityAnyOf},
		"feat:armadura-media":  nil,
		"feat:conjurador":      {FeatUnmetSpellcasting},
		"feat:humano":          nil,
		"feat:veterano":        {FeatUnmetLevel},
		"feat:meio-talento":    nil,
		"feat:grappler":        nil,
	}
	for key, kinds := range want {
		o := featOption(t, opts, key+tableSuffixOf(key))
		if got := unmetKinds(o.Unmet); !slices.Equal(got, kinds) || o.Qualifies != (len(kinds) == 0) {
			t.Errorf("%s: unmet %v qualifies %v, want unmet %v", key, got, o.Qualifies, kinds)
		}
	}

	if either := featOption(t, opts, "feat:agil-ou-esperto"+tableSuffix); len(either.Unmet) != 1 || either.Unmet[0].Kind != FeatUnmetAbilityAnyOf || len(either.Unmet[0].Abilities) != 2 {
		t.Errorf("an any-of prerequisite lists both abilities: %+v", either.Unmet)
	}

	// A weaker, non-human, unarmored caster of level 5 flips every answer.
	mage := sorcererBuild(5)
	mage.Race = "race:elf"
	mage.Subrace = "subrace:high-elf"
	opts = FeatOptions(mage, c)
	flipped := map[string][]string{
		"feat:forte":           {FeatUnmetAbilityMinimum}, // STR 8 is short; CON 14 is enough
		"feat:agil-ou-esperto": nil,                       // DEX 16 as a high elf
		"feat:armadura-media":  {FeatUnmetProficiency},
		"feat:conjurador":      nil,
		"feat:humano":          {FeatUnmetRace},
		"feat:veterano":        nil,
	}
	for key, kinds := range flipped {
		o := featOption(t, opts, key+tableSuffixOf(key))
		if got := unmetKinds(o.Unmet); !slices.Equal(got, kinds) {
			t.Errorf("%s for the mage: unmet %v, want %v", key, got, kinds)
		}
	}
	// Taken feats are not offered again.
	toren.Feats = []string{"feat:grappler"}
	for _, o := range FeatOptions(toren, c) {
		if o.Key == "feat:grappler" {
			t.Error("a feat the character has is offered again")
		}
	}
}

func tableSuffixOf(key string) string {
	if key == "feat:grappler" {
		return ""
	}
	return tableSuffix
}

// TestTableFeatIsRefusedWhenItsShapeIsWrong: the prerequisite and the effects of a
// table feat are checked, each problem at its own field.
func TestTableFeatIsRefusedWhenItsShapeIsWrong(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)
	good := func() TableFeat {
		return TableFeat{TableEntry: TableEntry{Key: "feat:teste" + tableSuffix, NamePT: "Teste"}}
	}
	cases := []struct {
		name        string
		mutate      func(*TableFeat)
		field, why  string
		wantNoError bool
	}{
		{name: "a valid feat", mutate: func(*TableFeat) {}, wantNoError: true},
		{"an unknown ability", func(f *TableFeat) { f.Prerequisite.Minimums = map[Ability]int{"luck": 13} }, "feats[0].prerequisite.minimums", ReasonValue, false},
		{"a minimum over 30", func(f *TableFeat) { f.Prerequisite.AnyOf = map[Ability]int{STR: 31} }, "feats[0].prerequisite.any_of.strength", ReasonValue, false},
		{"an unknown proficiency", func(f *TableFeat) { f.Prerequisite.Proficiency = "proficiency:nada" }, "feats[0].prerequisite.proficiency_key", ReasonReference, false},
		{"an unknown race", func(f *TableFeat) { f.Prerequisite.Race = "race:nada" }, "feats[0].prerequisite.race_key", ReasonReference, false},
		{"a level over 20", func(f *TableFeat) { f.Prerequisite.Level = 21 }, "feats[0].prerequisite.level", ReasonValue, false},
		{"a key that is not a feat's", func(f *TableFeat) { f.Key = "class:teste" + tableSuffix }, "feats[0].key", ReasonKey, false},
		{"a text that is too long", func(f *TableFeat) { f.DescPT = []string{strings.Repeat("a", maxTextRunes+1)} }, "feats[0].desc_pt[0]", ReasonText, false},
		{"an effect that is not on the menu", func(f *TableFeat) { f.Effects = []Effect{{Type: "handler", Handler: "monk.martial_arts"}} }, "feats[0].effects[0].type", ReasonEffect, false},
		{"two ability increases", func(f *TableFeat) {
			e := Effect{Type: "ability_increase", Count: 1, From: []string{"str"}, Value: "1"}
			f.Effects = []Effect{e, e}
		}, "feats[0].effects[1]", ReasonLimit, false},
		{"an increase with no abilities", func(f *TableFeat) { f.Effects = []Effect{{Type: "ability_increase", Count: 1, Value: "1"}} }, "feats[0].effects[0].from", ReasonValue, false},
		{"an increase that picks more than it lists", func(f *TableFeat) {
			f.Effects = []Effect{{Type: "ability_increase", Count: 3, From: []string{"str", "dex"}, Value: "1"}}
		}, "feats[0].effects[0].count", ReasonValue, false},
		{"an increase of 3", func(f *TableFeat) {
			f.Effects = []Effect{{Type: "ability_increase", Count: 1, From: []string{"str"}, Value: "3"}}
		}, "feats[0].effects[0].value", ReasonValue, false},
		{"an increase of a formula", func(f *TableFeat) {
			f.Effects = []Effect{{Type: "ability_increase", Count: 1, From: []string{"str"}, Value: "level()"}}
		}, "feats[0].effects[0].value", ReasonValue, false},
		{"an ability listed twice", func(f *TableFeat) {
			f.Effects = []Effect{{Type: "ability_increase", Count: 1, From: []string{"str", "str"}, Value: "1"}}
		}, "feats[0].effects[0].from", ReasonValue, false},
		{"an unknown ability to raise", func(f *TableFeat) {
			f.Effects = []Effect{{Type: "ability_increase", Count: 1, From: []string{"luck"}, Value: "1"}}
		}, "feats[0].effects[0].from", ReasonValue, false},
		{"a stray field on an increase", func(f *TableFeat) {
			f.Effects = []Effect{{Type: "ability_increase", Count: 1, From: []string{"str"}, Value: "1", Target: "ac"}}
		}, "feats[0].effects[0].target", ReasonValue, false},
		{"a choice of feats the SRD does not have", func(f *TableFeat) {
			f.Effects = []Effect{{Type: "choice", Choice: "feat", Count: 1, From: []string{"feat:nada"}}}
		}, "feats[0].effects[0].from", ReasonReference, false},
		{"a choice of feats that lists a class", func(f *TableFeat) {
			f.Effects = []Effect{{Type: "choice", Choice: "feat", Count: 1, From: []string{"class:fighter"}}}
		}, "feats[0].effects[0].from", ReasonReference, false},
		{"a choice of more feats than it lists", func(f *TableFeat) {
			f.Effects = []Effect{{Type: "choice", Choice: "feat", Count: 2, From: []string{"feat:grappler"}}}
		}, "feats[0].effects[0].count", ReasonValue, false},
		{"a choice of any feat", func(f *TableFeat) { f.Effects = []Effect{{Type: "choice", Choice: "feat", Count: 1}} }, "", "", true},
		{"a choice of the SRD's feat", func(f *TableFeat) {
			f.Effects = []Effect{{Type: "choice", Choice: "feat", Count: 1, From: []string{"feat:grappler"}}}
		}, "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := good()
			tc.mutate(&f)
			_, err := srd.With(Overlay{Revision: 1, Strict: []string{f.Key}, Feats: []TableFeat{f}})
			if tc.wantNoError {
				if err != nil {
					t.Fatalf("With() error = %v", err)
				}
				return
			}
			oe, ok := errors.AsType[*OverlayError](err)
			if !ok {
				t.Fatalf("With() error = %v, want an OverlayError at %s", err, tc.field)
			}
			found := false
			for _, v := range oe.Violations() {
				found = found || (v.Field == tc.field && v.Reason == tc.why)
			}
			if !found {
				t.Errorf("violations = %+v, want one at %s (%s)", oe.Violations(), tc.field, tc.why)
			}
		})
	}
}

// TestAbilityIncreaseBelongsToFeats: a feature (a class, race or background's)
// cannot have an ability_increase: nothing would apply it.
func TestAbilityIncreaseBelongsToFeats(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)
	tc := classWithEffects([]Effect{{Type: "ability_increase", Count: 1, From: []string{"str"}, Value: "1"}})
	_, err := srd.With(Overlay{Revision: 1, Classes: []TableClass{tc}})
	oe, ok := errors.AsType[*OverlayError](err)
	if !ok || oe.Reason != ReasonEffect {
		t.Fatalf("With() error = %v, want a forbidden_effect", err)
	}
}

// TestTableFeatsAreLiveContent: a table feat is in the content with its
// prerequisite and the ability increase it gives, can be switched off, retired,
// and a sheet that has it lists it.
func TestTableFeatsAreLiveContent(t *testing.T) {
	t.Parallel()
	c := tableFeats(t)
	e, ok := c.Feat("feat:meio-talento" + tableSuffix)
	if !ok || !e.Table || e.Increase == nil || e.Increase.Count != 1 || e.Increase.Value != 1 || !slices.Equal(e.Increase.From, []Ability{STR, DEX}) {
		t.Fatalf("the half feat = %+v", e)
	}
	if !c.Switchable("feat:meio-talento"+tableSuffix) || !c.Switchable("feat:grappler") || c.Switchable("feat:nada") {
		t.Error("feats are options the master can switch")
	}
	b := torenLevelUp()
	b.Feats = []string{"feat:meio-talento" + tableSuffix}
	if keys := TableKeys(b); !slices.Equal(keys, []string{"feat:meio-talento" + tableSuffix}) {
		t.Errorf("TableKeys = %v: a sheet with a table feat cannot move to another campaign", keys)
	}
	d := Derive(b, c)
	if !slices.ContainsFunc(d.Features, func(f Feature) bool { return f.Key == "feat:meio-talento"+tableSuffix && f.NamePT == "Meio talento" }) {
		t.Errorf("the sheet does not list the table feat: %+v", d.Features)
	}

	off, err := loadForTest(t).With(Overlay{Revision: 2, Off: []string{"feat:grappler", "feat:livre" + tableSuffix}, Feats: []TableFeat{{
		TableEntry: TableEntry{Key: "feat:livre" + tableSuffix, NamePT: "Livre", Archived: false}, DescPT: []string{"x"},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if !off.Off("feat:grappler") || !off.Hidden("feat:grappler") || !off.Hidden("feat:livre"+tableSuffix) {
		t.Error("a switched off feat is hidden from the players")
	}
}

// TestLevelUpTakesAFeatInPlaceOfTheIncrease: at an Ability Score Improvement
// level a feat can be taken instead of the increase, when the character meets its
// prerequisite and the feat's own ability increase is applied exactly.
func TestLevelUpTakesAFeatInPlaceOfTheIncrease(t *testing.T) {
	t.Parallel()
	c := tableFeats(t)
	before := torenLevelUp() // Champion 3: level 4 has an Ability Score Improvement
	avg := LevelUpHitPoints{Average: true}

	offer, err := LevelUpOptions(before, "class:fighter", c)
	if err != nil {
		t.Fatal(err)
	}
	if !offer.AbilityScoreImprovement || len(offer.Feats) != 9 {
		t.Fatalf("offer: asi=%v feats=%d, want an ASI level with the 9 feats of the content", offer.AbilityScoreImprovement, len(offer.Feats))
	}
	if !featOption(t, offer.Feats, "feat:forte"+tableSuffix).Qualifies {
		t.Error("a Strength and Constitution 13 feat is refused to STR 16, CON 14")
	}
	if featOption(t, offer.Feats, "feat:veterano"+tableSuffix).Qualifies {
		t.Error("a level 5 prerequisite is met by a level 4 character")
	}

	// A feat with a met prerequisite and no increase: no ability change.
	after := mustApply(t, c, before, LevelUpChoices{Class: "class:fighter", Feat: "feat:forte" + tableSuffix, HitPoints: avg})
	if err := CheckLevelUp(before, after, c); err != nil {
		t.Fatalf("CheckLevelUp() error = %v", err)
	}
	if !slices.Equal(after.Feats, []string{"feat:forte" + tableSuffix}) || len(after.ExtraAbilityBonuses) != 0 {
		t.Errorf("after: feats %v bonuses %v", after.Feats, after.ExtraAbilityBonuses)
	}

	// The half feat raises one of its two abilities by 1.
	half := "feat:meio-talento" + tableSuffix
	after = mustApply(t, c, before, LevelUpChoices{Class: "class:fighter", Feat: half, AbilityIncrease: map[Ability]int{DEX: 1}, HitPoints: avg})
	if err := CheckLevelUp(before, after, c); err != nil {
		t.Fatalf("the half feat with +1 DEX: %v", err)
	}
	if got := Derive(after, c).Abilities[1].Score; got != 14 {
		t.Errorf("DEX after the half feat = %d, want 14", got)
	}

	refused := []struct {
		name   string
		ch     LevelUpChoices
		reason string
	}{
		{"an unmet prerequisite", LevelUpChoices{Feat: "feat:agil-ou-esperto" + tableSuffix}, LevelUpReasonFeatPrerequisite},
		{"a level the character has not reached", LevelUpChoices{Feat: "feat:veterano" + tableSuffix}, LevelUpReasonFeatPrerequisite},
		{"a feat that does not exist", LevelUpChoices{Feat: "feat:nada"}, LevelUpReasonFeat},
		{"the half feat without its increase", LevelUpChoices{Feat: half}, LevelUpReasonAbilityShape},
		{"the half feat raising an ability it does not list", LevelUpChoices{Feat: half, AbilityIncrease: map[Ability]int{CON: 1}}, LevelUpReasonAbilityShape},
		{"the half feat raising two abilities", LevelUpChoices{Feat: half, AbilityIncrease: map[Ability]int{STR: 1, DEX: 1}}, LevelUpReasonAbilityShape},
		{"the half feat raising by 2", LevelUpChoices{Feat: half, AbilityIncrease: map[Ability]int{STR: 2}}, LevelUpReasonAbilityShape},
		{"a feat without an increase raising an ability", LevelUpChoices{Feat: "feat:forte" + tableSuffix, AbilityIncrease: map[Ability]int{STR: 1}}, LevelUpReasonAbilityShape},
	}
	for _, tc := range refused {
		tc.ch.Class, tc.ch.HitPoints = "class:fighter", avg
		after := mustApply(t, c, before, tc.ch)
		wantRefusal(t, CheckLevelUp(before, after, c), tc.reason, "")
	}

	// A feat needs an Ability Score Improvement level, and only one is taken.
	level4 := mustApply(t, c, before, LevelUpChoices{Class: "class:fighter", HitPoints: avg}) // level 4, no ASI taken
	for _, ch := range []LevelUpChoices{{Feat: "feat:livre" + tableSuffix}} {
		ch.Class, ch.HitPoints = "class:fighter", avg
		after := mustApply(t, c, level4, ch) // level 5 has no ASI
		wantRefusal(t, CheckLevelUp(level4, after, c), LevelUpReasonFeat, "full.feat_keys")
	}
	two := mustApply(t, c, before, LevelUpChoices{Class: "class:fighter", Feat: "feat:livre" + tableSuffix, HitPoints: avg})
	two.Feats = append(two.Feats, "feat:forte"+tableSuffix)
	wantRefusal(t, CheckLevelUp(before, two, c), LevelUpReasonFeat, "full.feat_keys")
	gone := before.clone()
	gone.Feats = []string{"feat:livre" + tableSuffix}
	kept := mustApply(t, c, gone, LevelUpChoices{Class: "class:fighter", HitPoints: avg})
	kept.Feats = nil
	wantRefusal(t, CheckLevelUp(gone, kept, c), LevelUpReasonFeat, "full.feat_keys")
	dup := mustApply(t, c, gone, LevelUpChoices{Class: "class:fighter", Feat: "feat:livre" + tableSuffix, HitPoints: avg})
	wantRefusal(t, CheckLevelUp(gone, dup, c), LevelUpReasonFeat, "full.feat_keys[1]")

	// Not an ASI level: the offer has no feat.
	offer, err = LevelUpOptions(level4, "class:fighter", c)
	if err != nil || offer.AbilityScoreImprovement || len(offer.Feats) != 0 {
		t.Errorf("level 5 offer: %+v, %v", offer.Feats, err)
	}
}

// TestFeatAbilityIncreaseNeverPassesTwenty: an ability a feat raises stops at 20
// (SRD 5.1, Ability Score Improvement): the option says so when the feat cannot be
// taken, and the level-up refuses an increase above 20.
func TestFeatAbilityIncreaseNeverPassesTwenty(t *testing.T) {
	t.Parallel()
	c := tableFeats(t)
	half := "feat:meio-talento" + tableSuffix
	avg := LevelUpHitPoints{Average: true}

	before := torenLevelUp()
	before.BaseScores[STR] = 19 // human +1: 20
	offer, err := LevelUpOptions(before, "class:fighter", c)
	if err != nil {
		t.Fatal(err)
	}
	if o := featOption(t, offer.Feats, half); !o.Qualifies {
		t.Fatalf("with DEX below 20 the half feat is still possible: %+v", o.Unmet)
	}
	after := mustApply(t, c, before, LevelUpChoices{Class: "class:fighter", Feat: half, AbilityIncrease: map[Ability]int{STR: 1}, HitPoints: avg})
	wantRefusal(t, CheckLevelUp(before, after, c), LevelUpReasonAbilityAbove20, "full.extra_ability_bonuses.strength")
	after = mustApply(t, c, before, LevelUpChoices{Class: "class:fighter", Feat: half, AbilityIncrease: map[Ability]int{DEX: 1}, HitPoints: avg})
	if err := CheckLevelUp(before, after, c); err != nil {
		t.Errorf("raising the ability that is below 20: %v", err)
	}

	before.BaseScores[DEX] = 19 // human +1: 20 too
	offer, _ = LevelUpOptions(before, "class:fighter", c)
	if o := featOption(t, offer.Feats, half); o.Qualifies || !slices.Contains(unmetKinds(o.Unmet), FeatUnmetAbilityCap) {
		t.Errorf("both abilities at 20: the half feat qualifies=%v unmet=%v", o.Qualifies, unmetKinds(o.Unmet))
	}
	// The plain Ability Score Improvement keeps its own cap.
	after = mustApply(t, c, before, LevelUpChoices{Class: "class:fighter", AbilityIncrease: map[Ability]int{STR: 2}, HitPoints: avg})
	wantRefusal(t, CheckLevelUp(before, after, c), LevelUpReasonAbilityAbove20, "")
}
