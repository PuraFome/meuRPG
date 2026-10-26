package rules

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// fieldsOf are the fields the refusal err names.
func refusedFields(err error) []string {
	var oe *OverlayError
	if !errors.As(err, &oe) {
		return nil
	}
	var out []string
	for _, v := range oe.Violations() {
		out = append(out, v.Field)
	}
	return out
}

// namesField says the refusal err has a violation at a field that ends with suffix.
func namesField(err error, suffix string) bool {
	for _, f := range refusedFields(err) {
		if strings.HasSuffix(f, suffix) {
			return true
		}
	}
	return false
}

// tableFeatOn applies the feats to the SRD and returns the content or the error.
func tableFeatsWith(t *testing.T, feats ...TableFeat) (*Content, error) {
	t.Helper()
	return loadForTest(t).With(Overlay{Revision: 1, Feats: feats})
}

func inventedFeat(slug, name string, effects ...Effect) TableFeat {
	return TableFeat{
		TableEntry: TableEntry{Key: "feat:" + slug + tableSuffix, NamePT: name}, DescPT: []string{"Texto de " + name + "."},
		Effects: effects,
	}
}

// A table feat with 3 uses per long rest and an action that spends them.
func TestFeatActionSpendsTheFeatsOwnResource(t *testing.T) {
	t.Parallel()
	feat := inventedFeat("sopro-gelado", "Sopro gelado",
		Effect{Type: "resource", Resource: "sopro_gelado", Max: "3", Recharge: "long_rest"},
		Effect{Type: "grant_action", Economy: "action", Resource: "sopro_gelado"},
	)
	c, err := tableFeatsWith(t, feat)
	if err != nil {
		t.Fatalf("the feat with a resource and an action is refused: %v", err)
	}
	b := torenLevelUp()
	b.Feats = []string{feat.Key}
	d := Derive(b, c)
	var spends bool
	for _, a := range d.Actions {
		spends = spends || (a.Source == feat.Key && a.Resource == "sopro_gelado")
	}
	if !spends {
		t.Errorf("the sheet has no action of the feat that spends sopro_gelado: %+v", d.Actions)
	}
	var uses int
	for _, r := range d.Resources {
		if r.Key == "sopro_gelado" {
			uses = r.Max
		}
	}
	if uses != 3 {
		t.Errorf("sopro_gelado has %d uses, want 3", uses)
	}
}

func TestFeatActionOfAResourceNobodyDefinesIsRefused(t *testing.T) {
	t.Parallel()
	for _, e := range []Effect{
		{Type: "grant_action", Economy: "action", Resource: "fantasma"},
		{Type: "superiority_die", Resource: "fantasma", Value: "6", Applies: ManeuverAttack},
		{Type: "extra_damage", Resource: "fantasma", Value: "1d6", Applies: ExtraDamageAnyTarget},
	} {
		_, err := tableFeatsWith(t, inventedFeat("sem-recurso", "Sem recurso", e))
		if err == nil || !strings.Contains(err.Error(), "fantasma") {
			t.Errorf("%s of a resource nobody defines: err = %v", e.Type, err)
		}
	}
}

// A resource of one feat is not another feat's.
func TestFeatResourceIsNotShared(t *testing.T) {
	t.Parallel()
	a := inventedFeat("a", "A", Effect{Type: "resource", Resource: "so_da_a", Max: "1", Recharge: "long_rest"})
	b := inventedFeat("b", "B", Effect{Type: "grant_action", Economy: "action", Resource: "so_da_a"})
	if _, err := tableFeatsWith(t, a, b); err == nil {
		t.Error("a feat spends the resource of another feat")
	}
}

// ---- resistance, immunity and vulnerability ----

// resistanceFeat is a table feat with one resistance effect.
func resistanceFeat(slug string, e Effect) TableFeat {
	e.Type = "resistance"
	return inventedFeat(slug, "Proteção "+slug, e)
}

func TestResistanceRefusalsNameTheField(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		effect Effect
		field  string
	}{
		{"unknown damage type", Effect{DamageTypes: []string{"damage-type:sparkle"}}, ".damage_types"},
		{"not a damage type key", Effect{DamageTypes: []string{"cold"}}, ".damage_types"},
		{"empty", Effect{}, ".damage_types"},
		{"except and damage_types", Effect{Except: "damage-type:fire", DamageTypes: []string{"damage-type:cold"}}, ".damage_types"},
		{"all with another", Effect{DamageTypes: []string{"all", "damage-type:cold"}}, ".damage_types"},
		{"a type twice", Effect{DamageTypes: []string{"damage-type:cold", "damage-type:cold"}}, ".damage_types"},
		{"unknown kind", Effect{DamageTypes: []string{"damage-type:cold"}, Kind: "absorb"}, ".kind"},
		{"unknown while", Effect{DamageTypes: []string{"damage-type:cold"}, While: "night"}, ".while"},
		{"except unknown", Effect{Except: "damage-type:sparkle"}, ".except"},
		{"a when", Effect{DamageTypes: []string{"damage-type:cold"}, When: "true"}, ".when"},
	} {
		_, err := tableFeatsWith(t, resistanceFeat("r", tc.effect))
		if !namesField(err, tc.field) {
			t.Errorf("%s: fields = %v (err %v), want one ending with %s", tc.name, refusedFields(err), err, tc.field)
		}
	}
}

// The race with resistance to cold, the feat with immunity to poison damage, the feature with resistance
// to everything, the trait that makes nonmagical weapon damage weaker: the sheet lists each as it is.
func TestResistancesOfTableEntries(t *testing.T) {
	t.Parallel()
	cold := resistanceFeat("frio", Effect{DamageTypes: []string{"damage-type:cold"}})
	poison := resistanceFeat("veneno", Effect{DamageTypes: []string{"damage-type:poison"}, Kind: "immune"})
	all := resistanceFeat("tudo", Effect{DamageTypes: []string{"all"}})
	weak := resistanceFeat("pele", Effect{DamageTypes: []string{"nonmagical_bps", "damage-type:fire"}, Kind: "vulnerable"})
	c, err := tableFeatsWith(t, cold, poison, all, weak)
	if err != nil {
		t.Fatal(err)
	}
	b := torenLevelUp()
	b.Feats = []string{cold.Key, poison.Key, all.Key, weak.Key}
	got := c.Resistances(Derive(b, c))
	by := map[string][]Resistance{}
	for _, r := range got {
		by[r.Source] = append(by[r.Source], r)
	}
	if r := by[cold.Key]; len(r) != 1 || r[0].Kind != ResistanceKindResist || len(r[0].DamageTypes) != 1 || r[0].DamageTypes[0] != "damage-type:cold" {
		t.Errorf("cold = %+v", r)
	}
	if r := by[poison.Key]; len(r) != 1 || r[0].Kind != ResistanceKindImmune {
		t.Errorf("poison = %+v", r)
	}
	if r := by[all.Key]; len(r) != 1 || len(r[0].DamageTypes) < 13 {
		t.Errorf("all covers %d types: %+v", len(r), r)
	}
	r := by[weak.Key]
	if len(r) != 2 || r[0].Nonmagical || r[0].DamageTypes[0] != "damage-type:fire" || !r[1].Nonmagical || len(r[1].DamageTypes) != 3 || r[1].Kind != ResistanceKindVulnerable {
		t.Errorf("weak = %+v", r)
	}
}

func TestResistanceActiveIsTheOneGate(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		while  string
		raging bool
		want   bool
	}{
		{ResistanceAlways, false, true}, {ResistanceAlways, true, true}, {ResistanceWhileRage, true, true}, {ResistanceWhileRage, false, false}, {"night", true, false},
	} {
		if got := ResistanceActive(tc.while, tc.raging); got != tc.want {
			t.Errorf("ResistanceActive(%q, %v) = %v, want %v", tc.while, tc.raging, got, tc.want)
		}
	}
}

// ---- condition immunity and advantage against a condition ----

func TestConditionImmunityRefusals(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		effect Effect
	}{
		{"none", Effect{Type: "condition_immunity"}},
		{"unknown", Effect{Type: "condition_immunity", Conditions: []string{"condition:sleepy"}}},
		{"not a key", Effect{Type: "condition_immunity", Conditions: []string{"poisoned"}}},
		{"twice", Effect{Type: "condition_immunity", Conditions: []string{"condition:poisoned", "condition:poisoned"}}},
	} {
		_, err := tableFeatsWith(t, inventedFeat("i", "I", tc.effect))
		if !namesField(err, ".conditions") {
			t.Errorf("%s: fields = %v (err %v), want .conditions", tc.name, refusedFields(err), err)
		}
	}
	o := Overlay{Revision: 1, Feats: []TableFeat{inventedFeat("j", "J", Effect{Type: "condition_immunity", Conditions: []string{"condition:poisoned"}, Kind: "immune"})}}
	o.Strict = []string{"feat:j" + tableSuffix}
	if _, err := loadForTest(t).With(o); !namesField(err, ".kind") {
		t.Errorf("a condition_immunity with a stray kind: fields = %v", refusedFields(err))
	}
}

func TestRollModeAgainstConditionRefusals(t *testing.T) {
	t.Parallel()
	ok := Effect{Type: "roll_mode", Roll: "advantage", Targets: []string{"save.wis"}, AgainstCondition: "condition:frightened"}
	if _, err := tableFeatsWith(t, inventedFeat("v", "V", ok)); err != nil {
		t.Fatalf("advantage on saves against being frightened is refused: %v", err)
	}
	for name, mutate := range map[string]func(e *Effect){
		"not a condition": func(e *Effect) { e.AgainstCondition = "frightened" },
		"a skill target":  func(e *Effect) { e.Targets = []string{"skill:stealth"} },
		"with a tag":      func(e *Effect) { e.Tags = []string{"against:magic"} },
		"unknown mode":    func(e *Effect) { e.Roll = "always" },
	} {
		e := ok
		mutate(&e)
		if _, err := tableFeatsWith(t, inventedFeat("w", "W", e)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

// The hint of a roll mode against a condition carries the condition the play module reads.
// A resistance with no "while" holds always; with "rage" only while raging.
func TestResistanceWhileEmptyIsAlways(t *testing.T) {
	t.Parallel()
	if _, err := tableFeatsWith(t, resistanceFeat("sempre", Effect{DamageTypes: []string{"damage-type:cold"}})); err != nil {
		t.Errorf("a resistance with no while is refused: %v", err)
	}
	if _, err := tableFeatsWith(t, resistanceFeat("furia", Effect{DamageTypes: []string{"damage-type:cold"}, While: ResistanceWhileRage})); err != nil {
		t.Errorf("a resistance while raging is refused: %v", err)
	}
}

func TestRollModeAgainstConditionIsAHintWithTheCondition(t *testing.T) {
	t.Parallel()
	feat := inventedFeat("destemido", "Destemido", Effect{Type: "roll_mode", Roll: "advantage", Targets: []string{"save.wis"}, AgainstCondition: "condition:frightened"})
	c, err := tableFeatsWith(t, feat)
	if err != nil {
		t.Fatal(err)
	}
	b := torenLevelUp()
	b.Feats = []string{feat.Key}
	var found bool
	for _, h := range Derive(b, c).Hints {
		if h.Source == feat.Key && h.AgainstCondition == "condition:frightened" && h.Mode == "advantage" && len(h.Tags) == 0 {
			found = true
		}
	}
	if !found {
		t.Errorf("no hint against being frightened: %+v", Derive(b, c).Hints)
	}
}

func TestConditionImmunitiesOfTableEntries(t *testing.T) {
	t.Parallel()
	feat := inventedFeat("sangue-frio", "Sangue frio",
		Effect{Type: "resistance", DamageTypes: []string{"damage-type:poison"}, Kind: "immune"},
		Effect{Type: "condition_immunity", Conditions: []string{"condition:poisoned"}},
	)
	c, err := tableFeatsWith(t, feat)
	if err != nil {
		t.Fatal(err)
	}
	b := torenLevelUp()
	b.Feats = []string{feat.Key}
	d := Derive(b, c)
	got := c.ConditionImmunities(d)
	if len(got) != 1 || got[0].Source != feat.Key || len(got[0].Conditions) != 1 || got[0].Conditions[0] != "condition:poisoned" {
		t.Errorf("ConditionImmunities = %+v", got)
	}
	if got := c.ConditionImmunities(Derive(torenLevelUp(), c)); len(got) != 0 {
		t.Errorf("a character without the feat is immune: %+v", got)
	}
}

// ---- a race's alternative sets of bonuses and its sizes ----

// inventedRace is a race of the table with +2/+1 or +1/+1/+1 to place and Small or Medium to pick.
func inventedRace(edit func(r *TableRace)) TableRace {
	r := TableRace{
		TableEntry: TableEntry{Key: "race:gelideo" + tableSuffix, NamePT: "Gelídeo"}, Size: "Medium", SpeedFt: 30,
		ChoiceBonusSets: [][]int{{2, 1}, {1, 1, 1}}, Sizes: []string{"Small", "Medium"},
	}
	if edit != nil {
		edit(&r)
	}
	return r
}

func raceWith(t *testing.T, r TableRace) (*Content, error) {
	t.Helper()
	return loadForTest(t).With(Overlay{Revision: 1, Races: []TableRace{r}})
}

func TestRaceSetsAndSizesRefusalsNameTheField(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		edit  func(r *TableRace)
		field string
	}{
		{"both choice_bonuses and sets", func(r *TableRace) { r.ChoiceBonuses = []int{2, 1} }, ".choice_bonus_sets"},
		{"a bonus above +2", func(r *TableRace) { r.ChoiceBonusSets = [][]int{{3, 1}} }, ".choice_bonus_sets[0].bonuses[0]"},
		{"a bonus of 0", func(r *TableRace) { r.ChoiceBonusSets = [][]int{{2, 0}} }, ".choice_bonus_sets[0].bonuses[1]"},
		{"more than 6 bonuses", func(r *TableRace) { r.ChoiceBonusSets = [][]int{{1, 1, 1, 1, 1, 1, 1}} }, ".choice_bonus_sets[0].bonuses"},
		{"an empty set", func(r *TableRace) { r.ChoiceBonusSets = [][]int{{2, 1}, {}} }, ".choice_bonus_sets[1].bonuses"},
		{"the same set twice", func(r *TableRace) { r.ChoiceBonusSets = [][]int{{2, 1}, {1, 2}} }, ".choice_bonus_sets[1].bonuses"},
		{"too many sets", func(r *TableRace) { r.ChoiceBonusSets = [][]int{{1}, {2}, {1, 1}, {2, 1}, {1, 1, 1}} }, ".choice_bonus_sets"},
		{"a size twice", func(r *TableRace) { r.Sizes = []string{"Small", "Small"} }, ".sizes[1]"},
		{"a size that cannot be picked", func(r *TableRace) { r.Sizes = []string{"Medium", "Large"} }, ".sizes[1]"},
		{"the default is not among the sizes", func(r *TableRace) { r.Size = "Large"; r.Sizes = []string{"Small", "Medium"} }, ".sizes"},
	} {
		_, err := raceWith(t, inventedRace(tc.edit))
		if !namesField(err, tc.field) {
			t.Errorf("%s: fields = %v (err %v), want one ending with %s", tc.name, refusedFields(err), err, tc.field)
		}
	}
}

func TestRaceWithSetsAndSizesIsInTheCatalog(t *testing.T) {
	t.Parallel()
	c, err := raceWith(t, inventedRace(nil))
	if err != nil {
		t.Fatal(err)
	}
	var entry RaceEntry
	for _, r := range c.Catalog().Races {
		if r.Key == "race:gelideo"+tableSuffix {
			entry = r
		}
	}
	if len(entry.ChoiceBonusSets) != 2 || entry.ChoiceBonusSets[0][0] != 2 || len(entry.ChoiceBonusSets[1]) != 3 || len(entry.Sizes) != 2 || entry.Size != "Medium" || len(entry.ChoiceBonuses) != 0 {
		t.Errorf("catalog entry = %+v", entry)
	}
}

// The sheet needs the placed bonuses to cover one of the sets, whichever the player picked.
func TestRaceBonusSetsAreCoveredByAnyOne(t *testing.T) {
	t.Parallel()
	c, err := raceWith(t, inventedRace(nil))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		placed map[Ability]int
		issue  bool
	}{
		{"nothing placed", nil, true},
		{"+2 and +1", map[Ability]int{STR: 2, DEX: 1}, false},
		{"+1, +1 and +1", map[Ability]int{STR: 1, DEX: 1, CON: 1}, false},
		{"+1 and +1 only", map[Ability]int{STR: 1, DEX: 1}, true},
		{"+2 alone", map[Ability]int{STR: 2}, true},
	} {
		b := torenLevelUp()
		b.Race = "race:gelideo" + tableSuffix
		b.Subrace = ""
		b.ExtraAbilityBonuses = tc.placed
		var got bool
		for _, is := range Derive(b, c).Issues {
			got = got || is.Code == IssueRaceBonus
		}
		if got != tc.issue {
			t.Errorf("%s: race_bonus issue = %v, want %v", tc.name, got, tc.issue)
		}
	}
	// One set is also the free points: the largest of the sets.
	b := torenLevelUp()
	b.Race = "race:gelideo" + tableSuffix
	if got := c.FreeAbilityPoints(b); got < 3 {
		t.Errorf("FreeAbilityPoints = %d, want at least the 3 points of a set", got)
	}
}

func TestRaceSizePickedOnTheSheet(t *testing.T) {
	t.Parallel()
	c, err := raceWith(t, inventedRace(nil))
	if err != nil {
		t.Fatal(err)
	}
	size := func(pick string) (string, bool) {
		b := torenLevelUp()
		b.Race, b.Subrace, b.Size = "race:gelideo"+tableSuffix, "", pick
		d := Derive(b, c)
		var issue bool
		for _, is := range d.Issues {
			issue = issue || is.Code == IssueRaceSize
		}
		return d.Size, issue
	}
	for _, tc := range []struct {
		pick  string
		want  string
		issue bool
	}{{"", "Medium", false}, {"Small", "Small", false}, {"Medium", "Medium", false}, {"Large", "Medium", true}} {
		if got, issue := size(tc.pick); got != tc.want || issue != tc.issue {
			t.Errorf("pick %q: size %q, issue %v; want %q, %v", tc.pick, got, issue, tc.want, tc.issue)
		}
	}
	// A Small character gets the heavy weapon reminder; the same race picked Medium does not.
	b := torenLevelUp()
	b.Race, b.Subrace, b.Size = "race:gelideo"+tableSuffix, "", "Small"
	b.Weapons = []string{"equipment:greatsword"}
	heavy := func(d Derived) bool {
		for _, h := range d.Hints {
			if strings.Contains(h.TextPT, "Pequenas") {
				return true
			}
		}
		return false
	}
	if !heavy(Derive(b, c)) {
		t.Error("a Small character gets no heavy weapon reminder")
	}
	b.Size = "Medium"
	if heavy(Derive(b, c)) {
		t.Error("a Medium character gets the heavy weapon reminder")
	}
}

// A race with a single set in choice_bonuses behaves as it did, and a size on the sheet of a race that
// offers none is an issue.
func TestRaceWithoutSetsKeepsItsOneSet(t *testing.T) {
	t.Parallel()
	c, err := raceWith(t, inventedRace(func(r *TableRace) { r.ChoiceBonusSets, r.ChoiceBonuses, r.Sizes = nil, []int{2, 1}, nil }))
	if err != nil {
		t.Fatal(err)
	}
	b := torenLevelUp()
	b.Race, b.Subrace, b.Size = "race:gelideo"+tableSuffix, "", "Small"
	d := Derive(b, c)
	var bonus, size bool
	for _, is := range d.Issues {
		bonus = bonus || is.Code == IssueRaceBonus
		size = size || is.Code == IssueRaceSize
	}
	if !bonus || !size || d.Size != "Medium" {
		t.Errorf("issues = %+v, size %q; want a race_bonus and a race_size issue and Medium", d.Issues, d.Size)
	}
}

// ---- the picks of a race's sets of bonuses ----

// racePending is how many selections of the race's own choices are left.
func racePending(set ChoiceSet, race string) int {
	n := 0
	for _, p := range set.Pending() {
		if strings.HasPrefix(p.ChoiceKey, race+"#") {
			n += p.Missing
		}
	}
	return n
}

func scoreOf(d Derived, a Ability) int {
	for _, ab := range d.Abilities {
		if ab.Ability == a {
			return ab.Score
		}
	}
	return 0
}

// A race with a single "+2 and +1 to your choice" has the picker the half-elf's +1s have: in "Escolhas", a choice of
// abilities for each bonus, the picks make the scores, no ability takes two bonuses, and no manual bonus is asked.
func TestSingleSetOfBonusesIsPickedInTheChoices(t *testing.T) {
	t.Parallel()
	c, err := raceWith(t, inventedRace(func(r *TableRace) { r.ChoiceBonusSets, r.ChoiceBonuses, r.Sizes = nil, []int{2, 1}, nil }))
	if err != nil {
		t.Fatal(err)
	}
	race := "race:gelideo" + tableSuffix
	build := func(picks ...string) Build {
		b := torenLevelUp()
		b.Race, b.Subrace, b.ExtraAbilityBonuses, b.FeatureChoices = race, "", nil, picks
		return b
	}
	pick := func(bonus int, a Ability) string { return ScopedChoice(BonusChoiceKey(race, bonus), abilityValue(a)) }

	set := c.Choices(build())
	plus2, plus1 := choiceOf(t, set, BonusChoiceKey(race, 2)), choiceOf(t, set, BonusChoiceKey(race, 1))
	if plus2.Kind != ChoiceKindAbilities || plus2.Picks != 1 || plus1.Picks != 1 || len(plus2.Options) != 6 || plus2.Options[0].SummaryPT != "+2" {
		t.Fatalf("the choices = %+v and %+v, want one pick of an ability each, +2 and +1", plus2, plus1)
	}
	if racePending(set, race) != 2 {
		t.Errorf("pending = %d, want 2: nothing is picked yet", racePending(set, race))
	}
	if !hasIssue(Derive(build(), c), IssueRaceBonus) {
		t.Error("no race_bonus issue while nothing is picked and no manual bonus placed")
	}

	picked := build(pick(2, STR), pick(1, DEX))
	d := Derive(picked, c)
	base := Derive(build(), c)
	if got, want := scoreOf(d, STR)-scoreOf(base, STR), 2; got != want {
		t.Errorf("Strength rose by %d, want %d", got, want)
	}
	if got, want := scoreOf(d, DEX)-scoreOf(base, DEX), 1; got != want {
		t.Errorf("Dexterity rose by %d, want %d", got, want)
	}
	if hasIssue(d, IssueRaceBonus) || racePending(c.Choices(picked), race) != 0 {
		t.Errorf("issues %+v, pending %d; want none: the picks fill the set", d.Issues, racePending(c.Choices(picked), race))
	}
	if ch := choiceOf(t, c.Choices(picked), BonusChoiceKey(race, 2)); ch.ResultPT != fmt.Sprintf("Força %d → %d.", scoreOf(base, STR), scoreOf(d, STR)) {
		t.Errorf("the result reads %q", ch.ResultPT)
	}

	// The ability that took +2 is not offered for +1, and picking it again is refused and gives nothing.
	again := build(pick(2, STR), pick(1, STR))
	ch := choiceOf(t, c.Choices(again), BonusChoiceKey(race, 1))
	if len(ch.Unmet) != 1 || len(ch.Picked) != 0 {
		t.Errorf("the repeated pick: unmet %v, picked %v; want it refused", ch.Unmet, ch.Picked)
	}
	if o := choiceOf(t, c.Choices(picked), BonusChoiceKey(race, 1)).Options[0]; !o.Blocked() {
		t.Errorf("Strength is offered for +1 after it took +2: %+v", o)
	}
	if got := scoreOf(Derive(again, c), STR) - scoreOf(base, STR); got != 2 {
		t.Errorf("Strength rose by %d with the repeated pick, want 2: the repeat gives nothing", got)
	}
}

// Several sets: the set is a choice of its own, the abilities of its bonuses come after it, and changing the set
// leaves the old picks of the other bonuses unoffered.
func TestSeveralSetsOfBonusesAreAChoiceOfTheSetThenOfTheAbilities(t *testing.T) {
	t.Parallel()
	c, err := raceWith(t, inventedRace(nil))
	if err != nil {
		t.Fatal(err)
	}
	race := "race:gelideo" + tableSuffix
	build := func(picks ...string) Build {
		b := torenLevelUp()
		b.Race, b.Subrace, b.ExtraAbilityBonuses, b.FeatureChoices = race, "", nil, picks
		return b
	}
	set := c.Choices(build())
	pickSet := choiceOf(t, set, BonusSetChoiceKey(race))
	if len(pickSet.Options) != 2 || pickSet.Options[0].NamePT != "+2 e +1" || pickSet.Options[1].NamePT != "+1, +1 e +1" || pickSet.Picks != 1 {
		t.Fatalf("the set choice = %+v", pickSet)
	}
	if racePending(set, race) != 1 {
		t.Errorf("pending = %d, want 1: the abilities wait for the set", racePending(set, race))
	}
	three := build(ScopedChoice(BonusSetChoiceKey(race), bonusSetValue(1)))
	ones := choiceOf(t, c.Choices(three), BonusChoiceKey(race, 1))
	if ones.Picks != 3 || ones.TitlePT != "3 bônus de +1" {
		t.Errorf("the +1 choice = %+v, want three picks", ones)
	}
	full := build(ScopedChoice(BonusSetChoiceKey(race), bonusSetValue(1)),
		ScopedChoice(BonusChoiceKey(race, 1), abilityValue(STR)), ScopedChoice(BonusChoiceKey(race, 1), abilityValue(CON)), ScopedChoice(BonusChoiceKey(race, 1), abilityValue(WIS)))
	d, base := Derive(full, c), Derive(build(), c)
	for _, a := range []Ability{STR, CON, WIS} {
		if scoreOf(d, a)-scoreOf(base, a) != 1 {
			t.Errorf("%s rose by %d, want 1", a, scoreOf(d, a)-scoreOf(base, a))
		}
	}
	if hasIssue(d, IssueRaceBonus) || racePending(c.Choices(full), race) != 0 {
		t.Errorf("issues %+v, pending %d; want none", d.Issues, racePending(c.Choices(full), race))
	}
	// The other set: the +1 picks no longer fit and are not offered.
	switched := build(append([]string{ScopedChoice(BonusSetChoiceKey(race), bonusSetValue(0))}, full.FeatureChoices[1:]...)...)
	if n := len(c.Choices(switched).NotOffered); n == 0 {
		// +1 is still a bonus of the first set, with one pick: the two extra ones are refused as past the number it takes.
		if ch := choiceOf(t, c.Choices(switched), BonusChoiceKey(race, 1)); len(ch.Overflow) != 2 {
			t.Errorf("the extra +1 picks: overflow %v, want 2", ch.Overflow)
		}
	}
}

// A sheet that placed the bonuses in the manual ones, before they were picks, keeps them: they stand for the picks.
func TestManualBonusesStandForThePicksOfASheetThatPlacedThem(t *testing.T) {
	t.Parallel()
	c, err := raceWith(t, inventedRace(func(r *TableRace) { r.ChoiceBonusSets, r.ChoiceBonuses, r.Sizes = nil, []int{2, 1}, nil }))
	if err != nil {
		t.Fatal(err)
	}
	b := torenLevelUp()
	b.Race, b.Subrace = "race:gelideo"+tableSuffix, ""
	b.ExtraAbilityBonuses = map[Ability]int{STR: 2, DEX: 1}
	set := c.Choices(b)
	if racePending(set, "race:gelideo"+tableSuffix) != 0 || hasIssue(Derive(b, c), IssueRaceBonus) {
		t.Errorf("pending %d, issues %+v; want none: the manual +2 and +1 stand", racePending(set, "race:gelideo"+tableSuffix), Derive(b, c).Issues)
	}
}
