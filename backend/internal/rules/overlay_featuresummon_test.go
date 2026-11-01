package rules

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// The summon of a table feature. The names are invented; the creatures are a table creature on the SRD's wolf and the
// SRD's giant spider.

const (
	packFeature  = "feature:matilha-do-guardiao" + tableSuffix
	packResource = "chamados"
)

// summonFeatureClass is a table class with no casting whose level 1 feature brings the pack leader or a wolf, spending
// a use of the resource "chamados" (2 uses, back on a long rest).
func summonFeatureClass() TableClass {
	tc := classFromDefaults(DefaultTable{Kind: CastingNone, Rows: loadDefaultRows()}, "guardiao")
	for i := range tc.Levels {
		tc.Levels[i].Features = nil
	}
	tc.Levels[0].Features = []TableFeature{{
		Key: packFeature, NamePT: "Matilha do Guardião", DescPT: []string{"O guardião chama a matilha."},
		Effects: []Effect{{Type: "resource", Resource: packResource, Max: "2", Recharge: RechargeLongRest}},
		Summon: &TableSummon{
			CreatureKeys: []string{packLeader, wolfKey}, Count: "1 + floor(prof() / 2) + slot()", MaxLive: 6,
			TurnMode: SummonTurnOwnerRollGroup, Replaces: SummonReplacesSame, ResourceKey: packResource,
			Concentration: true, Duration: TableDuration{Kind: DurationTimed, Amount: 1, Unit: DurationHour},
		},
	}}
	return tc
}

func featureSummonOverlay(edit func(*TableFeature)) Overlay {
	tc := summonFeatureClass()
	if edit != nil {
		edit(&tc.Levels[0].Features[0])
	}
	return Overlay{Revision: 1, Creatures: []TableCreature{packLeaderCreature()}, Classes: []TableClass{tc}}
}

func TestATableFeatureSummonsThroughTheSpellChecks(t *testing.T) {
	t.Parallel()
	c := withOverlayOn(t, loadForTest(t), featureSummonOverlay(nil))
	spell := FeatureSummonSpellKey(packFeature)
	if !slices.Contains(c.SummonSpells(), spell) {
		t.Fatalf("SummonSpells() = %v, want the feature's spell among them", c.SummonSpells())
	}
	rule, ok := c.SummonRule(spell)
	if !ok || rule.Feature != packFeature || rule.Resource != packResource || rule.MaxLive != 6 || !rule.ReplacesSame || rule.TurnMode != SummonTurnOwnerRollGroup {
		t.Errorf("SummonRule = %+v, %v; want the feature, its resource and its rule", rule, ok)
	}
	// slot() is 0 in the count of a feature: 1 + floor(2 / 2) + 0 at level 1, the proficiency bonus +2.
	b := Build{
		BaseScores: standard("class:x", 1).BaseScores, Race: "race:human", Background: "background:acolyte", Languages: testLanguages,
		Classes: []ClassLevel{{Class: "class:guardiao" + tableSuffix, Level: 1}},
	}
	got, err := c.SummonOptions(spell, featureSummonLevel, b)
	if err != nil || len(got.Options) != 1 || got.Options[0].Count != 2 || !got.Concentration || len(got.Options[0].Forms) != 2 {
		t.Fatalf("SummonOptions = %+v, %v; want one option of 2 creatures, concentration, two forms", got, err)
	}
	if _, err := c.CheckSummon(spell, featureSummonLevel, b, SummonPick{Creatures: []string{packLeader, wolfKey}}); err != nil {
		t.Errorf("CheckSummon = %v, want the pick accepted", err)
	}
	if _, err := c.CheckSummon(spell, featureSummonLevel, b, SummonPick{Creatures: []string{packLeader}}); !errors.Is(err, ErrSummonCount) {
		t.Errorf("CheckSummon(one of two) = %v, want ErrSummonCount", err)
	}
	d := Derive(b, c)
	if !slices.ContainsFunc(d.GrantedSpells, func(g GrantedSpell) bool {
		return g.Spell.Key == spell && g.Source == packFeature && g.Resource == packResource && !g.AtWill && !g.AlsoWithSlot && g.FeatureSummon
	}) {
		t.Errorf("GrantedSpells = %+v, want the feature's spell, spending the resource", d.GrantedSpells)
	}
	if i := slices.IndexFunc(d.Resources, func(r Resource) bool { return r.Key == packResource }); i < 0 || d.Resources[i].Max != 2 || d.Resources[i].Recharge != RechargeLongRest {
		t.Errorf("Resources = %+v, want 2 uses back on a long rest", d.Resources)
	}
}

func TestTheSpellOfAFeatureSummonIsHidden(t *testing.T) {
	t.Parallel()
	c := withOverlayOn(t, loadForTest(t), featureSummonOverlay(nil))
	spell := FeatureSummonSpellKey(packFeature)
	if slices.ContainsFunc(c.ListSpells(SpellFilter{}), func(e SpellEntry) bool { return e.Key == spell }) {
		t.Error("ListSpells lists the feature's hidden spell")
	}
	if slices.ContainsFunc(c.Catalog().Spells, func(e SpellEntry) bool { return e.Key == spell }) {
		t.Error("the catalog lists the feature's hidden spell")
	}
	// No entry can name it: it is not a table spell.
	named := featureSummonOverlay(func(f *TableFeature) {
		f.Effects = append(f.Effects, Effect{Type: "granted_spell", Spell: spell, Uses: packResource, Ability: "int"})
	})
	if _, err := loadForTest(t).With(named); err == nil {
		t.Error("an effect granting the hidden spell by its key was accepted")
	}
}

func TestATableFeatureSummonIsRefusedAtTheFieldThatIsWrong(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		edit  func(*TableFeature)
		field string
		more  string
	}{
		{"no resource", func(f *TableFeature) { f.Summon.ResourceKey = "" }, ".summon.resource_key", ReasonValue},
		{"a resource nobody defines", func(f *TableFeature) { f.Summon.ResourceKey = "pontos" }, ".summon.resource_key", ReasonReference},
		{"a cost of 2", func(f *TableFeature) { f.Summon.ResourceCost = 2 }, ".summon.resource_cost", ReasonValue},
		{"creatures for each circle", func(f *TableFeature) { f.Summon.CountPerSlotLevel = 1 }, ".summon.count_per_slot_level", ReasonValue},
		{"no count", func(f *TableFeature) { f.Summon.Count = " " }, ".summon.count", ReasonValue},
		{"a count that does not compile", func(f *TableFeature) { f.Summon.Count = "1 +" }, ".summon.count", ReasonFormula},
		{"acting after the owner", func(f *TableFeature) { f.Summon.TurnMode = SummonTurnAfterOwner }, ".summon.turn_mode", ReasonValue},
		{"a mount", func(f *TableFeature) { f.Summon.Role = SummonRoleMount }, ".summon.role", ReasonValue},
		{"no creature", func(f *TableFeature) { f.Summon.CreatureKeys = nil }, ".summon.creature_keys", ReasonLimit},
		{"nine creatures", func(f *TableFeature) {
			f.Summon.CreatureKeys = []string{"monster:wolf", "monster:giant-spider", "monster:bat", "monster:cat", "monster:crab", "monster:frog", "monster:hawk", "monster:lizard", "monster:rat"}
		}, ".summon.creature_keys", ReasonLimit},
		{"a creature that does not exist", func(f *TableFeature) { f.Summon.CreatureKeys = []string{packLeader, "creature:fantasma@mesa"} }, ".summon.creature_keys[1]", ReasonReference},
		{"a creature twice", func(f *TableFeature) { f.Summon.CreatureKeys = []string{wolfKey, packLeader, wolfKey} }, ".summon.creature_keys[2]", ReasonValue},
		{"a limit above 40", func(f *TableFeature) { f.Summon.MaxLive = MaxTableSummonLive + 1 }, ".summon.max_live", ReasonValue},
		{"concentration without a timed duration", func(f *TableFeature) { f.Summon.Duration = TableDuration{} }, ".summon.concentration", ReasonValue},
		{"an unknown casting time", func(f *TableFeature) { f.Summon.CastingTime.Unit = "week" }, ".summon.casting_time.unit", ReasonValue},
		{"an option that summons", func(f *TableFeature) {
			f.Effects = append(f.Effects, Effect{Type: "choice", Choice: "feature", Count: 1})
			f.Options = []TableFeature{{Key: "feature:opcao-da-matilha" + tableSuffix, NamePT: "Opção", Summon: f.Summon}}
		}, ".summon", ReasonValue},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := loadForTest(t).With(featureSummonOverlay(tc.edit))
			var oe *OverlayError
			if !errors.As(err, &oe) {
				t.Fatalf("With() error = %v, want a refusal", err)
			}
			for _, v := range oe.Violations() {
				if strings.HasSuffix(v.Field, tc.field) && v.Reason == tc.more {
					if !strings.Contains(v.Field, ".levels[0].features[0]") && !strings.Contains(v.Field, "options[0]") {
						t.Errorf("the path %q does not name the feature", v.Field)
					}
					return
				}
			}
			t.Errorf("no %s violation at %s: %v", tc.more, tc.field, err)
		})
	}
}

func TestASpellSummonStillTakesNothingOfAFeatureSummon(t *testing.T) {
	t.Parallel()
	for name, edit := range map[string]func(*TableSummon){
		"a resource":    func(s *TableSummon) { s.ResourceKey = "chamados" },
		"a casting":     func(s *TableSummon) { s.CastingTime = TableCastingTime{Unit: CastMinute, Amount: 1} },
		"a duration":    func(s *TableSummon) { s.Duration = TableDuration{Kind: DurationInstantaneous} },
		"concentration": func(s *TableSummon) { s.Concentration = true },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ts := summonSpell()
			edit(ts.Summon)
			if _, err := loadForTest(t).With(Overlay{Revision: 1, Creatures: []TableCreature{packLeaderCreature()}, Spells: []TableSpell{ts}}); err == nil {
				t.Error("a spell's summon with a feature's field was accepted")
			}
		})
	}
}

func TestTheSpellOfAFeatureSummonIsNoChoiceOfASheet(t *testing.T) {
	t.Parallel()
	c := withOverlayOn(t, loadForTest(t), featureSummonOverlay(nil))
	spell := FeatureSummonSpellKey(packFeature)
	// A Bard 10 has Magical Secrets: two spells off the list. The feature's spell is not one a sheet lists, in any list.
	base := levelUpCharAt(t, c, "class:bard", "subclass:lore", 10)
	other := spellsOff(c, 5, "class:bard")
	for _, tc := range []struct {
		name, field, want string
		edit              func(b *Build)
		last              func(b Build) int // the index of the spell just added
	}{
		{"known", "full.known_spell_keys", IssueUnknownKey, func(b *Build) { b.SpellsKnown = append(slices.Clone(b.SpellsKnown), spell) }, func(b Build) int { return len(b.SpellsKnown) - 1 }},
		{"prepared", "full.prepared_spell_keys", IssueUnknownKey, func(b *Build) { b.SpellsPrepared = append(slices.Clone(b.SpellsPrepared), spell) }, func(b Build) int { return len(b.SpellsPrepared) - 1 }},
		{"cantrips", "full.cantrip_keys", IssueUnknownKey, func(b *Build) { b.Cantrips = append(slices.Clone(b.Cantrips), spell) }, func(b Build) int { return len(b.Cantrips) - 1 }},
	} {
		b := base
		tc.edit(&b)
		field := tc.field + "[" + strconv.Itoa(tc.last(b)) + "]"
		if codes := issueCodes(Derive(b, c)); !slices.Contains(codes, tc.want+" "+field) {
			t.Errorf("%s: the feature's spell in the list: issues = %v, want %q", tc.name, codes, tc.want+" "+field)
		}
	}
	if codes := issueCodes(Derive(func() Build { b := base; b.SpellsKnown = append(slices.Clone(b.SpellsKnown), spell); return b }(), c)); slices.Contains(codes, IssueSpellNotOnList+" full.known_spell_keys["+strconv.Itoa(len(base.SpellsKnown))+"]") {
		t.Error("the feature's spell was taken for a spell off the list: it must not spend a Magical Secret")
	}
	// A real spell off the list takes the Secrets (the control).
	b := base
	b.SpellsKnown = append(slices.Clone(b.SpellsKnown), other...)
	for _, code := range issueCodes(Derive(b, c)) {
		if strings.HasPrefix(code, IssueUnknownKey+" full.known_spell_keys") {
			t.Errorf("a real spell off the list (control) was refused: %s", code)
		}
	}
}

func TestAFeatureWithALongKeyKeepsASpellKeyWithinTheLimitAndATakenOneIsRefused(t *testing.T) {
	t.Parallel()
	long := "feature:" + strings.Repeat("a", MaxSlugLength-2)
	a, b := FeatureSummonSpellKey(long+"x"+tableSuffix), FeatureSummonSpellKey(long+"y"+tableSuffix)
	if a == b || checkTableKey("spell:", a) != nil || checkTableKey("spell:", b) != nil {
		t.Errorf("spell keys of two long feature keys = %q, %q; want two different valid keys", a, b)
	}
	c := withOverlayOn(t, loadForTest(t), featureSummonOverlay(func(f *TableFeature) { f.Key = long + "x" + tableSuffix }))
	if _, ok := c.SummonRule(a); !ok {
		t.Error("the summon of a feature with a long key is not registered under its spell key")
	}
	taken := featureSummonOverlay(nil)
	taken.Spells = []TableSpell{{
		TableEntry: TableEntry{Key: FeatureSummonSpellKey(packFeature), NamePT: "Outra"}, Level: 1, School: "school:evocation",
		CastingTime: TableCastingTime{Unit: CastAction}, Range: TableRange{Kind: RangeSelf}, Duration: TableDuration{Kind: DurationInstantaneous}, Target: SpellTarget{Kind: TargetSelf},
	}}
	if _, err := loadForTest(t).With(taken); err == nil || !strings.Contains(err.Error(), "taken by another entry") {
		t.Errorf("a table spell on the feature's summon key: err = %v, want a refusal that says the key is taken", err)
	}
}

func TestTheHiddenSpellOfAFeatureSummonIsNotSwitchable(t *testing.T) {
	t.Parallel()
	c := withOverlayOn(t, loadForTest(t), featureSummonOverlay(nil))
	if spell := FeatureSummonSpellKey(packFeature); c.Switchable(spell) {
		t.Errorf("Switchable(%s) = true, want false: the hidden spell goes with its feature", spell)
	}
	if !c.Switchable("spell:fireball") {
		t.Error("Switchable(spell:fireball) = false, want true (the control)")
	}
}

// issuesAt are the issues of a derived sheet at a field, as code and message.
func issuesAt(d Derived, field string) []string {
	var out []string
	for _, i := range d.Issues {
		if i.Field == field {
			out = append(out, i.Code+": "+i.Message)
		}
	}
	return out
}

// A key typed in a spell list or a pick reads exactly like a key the content does not have: the same issue, field and
// message, so a sheet cannot tell a hidden spell from a missing one.
func TestTheSpellOfAFeatureSummonReadsAsAnUnknownKeyInEveryListOfASheet(t *testing.T) {
	t.Parallel()
	c := withOverlayOn(t, loadForTest(t), featureSummonOverlay(nil))
	hidden, unknown := FeatureSummonSpellKey(packFeature), "spell:nao-existe"+tableSuffix
	if _, ok := c.c.spells[hidden]; !ok {
		t.Fatal("the hidden spell is not registered: the probe proves nothing")
	}
	bard := levelUpCharAt(t, c, "class:bard", "subclass:lore", 10)
	warlock := Build{
		BaseScores: map[Ability]int{STR: 10, DEX: 14, CON: 14, INT: 12, WIS: 14, CHA: 12},
		Race:       "race:human", Background: "background:acolyte", Languages: testLanguages,
		Classes: []ClassLevel{{Class: "class:warlock", Level: 3}}, FeatureChoices: []string{"feature:pact-of-the-tome"},
	}
	pick := "feature:pact-of-the-tome#cantrips"
	for _, tc := range []struct {
		name, field string
		build       func(key string) Build
	}{
		{"cantrips", "full.cantrip_keys[0]", func(k string) Build { b := bard; b.Cantrips = []string{k}; return b }},
		{"known spells", "full.known_spell_keys[0]", func(k string) Build { b := bard; b.SpellsKnown = []string{k}; return b }},
		{"prepared spells", "full.prepared_spell_keys[0]", func(k string) Build { b := bard; b.SpellsPrepared = []string{k}; return b }},
		{"picks", "full.feature_choice_keys[1]", func(k string) Build {
			b := warlock
			b.FeatureChoices = []string{"feature:pact-of-the-tome", ScopedChoice(pick, k)}
			return b
		}},
	} {
		got, want := issuesAt(Derive(tc.build(hidden), c), tc.field), issuesAt(Derive(tc.build(unknown), c), tc.field)
		if len(want) == 0 {
			t.Errorf("%s: an unknown key raised no issue at %s (the control)", tc.name, tc.field)
		}
		if !slices.Equal(got, want) {
			t.Errorf("%s: the hidden spell raised %q at %s; an unknown key raised %q", tc.name, got, tc.field, want)
		}
		for _, i := range Derive(tc.build(hidden), c).Issues {
			if strings.Contains(i.Message, "feature") && strings.Contains(i.Message, "truque") {
				t.Errorf("%s: the message gives the hidden spell away: %s", tc.name, i.Message)
			}
		}
	}
}
