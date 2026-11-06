package rules

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

const resilientKey = "feat:resiliente" + tableSuffix

// resilientOverlay is a feat in the shape of Resilient: the player picks an ability, which rises by 1 (up to 20),
// whose saving throw becomes a proficiency, and a note shows the DC from it.
func resilientOverlay(from ...string) Overlay {
	e := resilientEffects()
	e[0].From = from
	o := featWith(resilientKey, e...)
	o.Strict = nil
	return o
}

func resilientBuild(base int, picks ...string) Build {
	b := pensantus()
	b.BaseScores[DEX] = base
	b.Feats = []string{resilientKey}
	b.FeatureChoices = picks
	return b
}

func resilientPick(value string) string {
	return ScopedChoice(PickChoiceKey(resilientKey, "ability"), value)
}

func hintWith(d Derived, text string) bool {
	return slices.ContainsFunc(d.Hints, func(h Hint) bool { return strings.Contains(h.TextPT, text) })
}

func TestAPickedAbilityReachesTheSheet(t *testing.T) {
	t.Parallel()
	c := withOverlay(t, resilientOverlay())
	for name, tc := range map[string]struct {
		base, want int
	}{"a raise": {16, 17}, "capped at 20": {20, 20}, "to 20": {19, 20}} {
		d := Derive(resilientBuild(tc.base, resilientPick("ability:dex")), c)
		if got := abilityOf(d, DEX).Score; got != tc.want {
			t.Errorf("%s: Dexterity = %d, want %d", name, got, tc.want)
		}
		if !saveOf(d, DEX).Proficient {
			t.Errorf("%s: no proficiency in the Dexterity save", name)
		}
		if tc.base == 20 {
			for _, is := range d.Issues {
				if is.Code == IssueScoreAbove20 {
					t.Errorf("a score of 20 raised by a capped modifier is reported: %+v", is)
				}
			}
		}
	}

	// The DC of the note follows the picked ability: 8 + prof 2 + Dexterity modifier (17 is +3).
	d := Derive(resilientBuild(16, resilientPick("ability:dex")), c)
	if !hintWith(d, "CD 13") {
		t.Errorf("hints = %+v, want the DC 13", d.Hints)
	}
	d = Derive(resilientBuild(16, resilientPick("ability:int")), c)
	if abilityOf(d, DEX).Score != 16 || abilityOf(d, INT).Score != 19 || !saveOf(d, INT).Proficient || !hintWith(d, "CD 14") {
		t.Errorf("Intelligence pick: dex %d int %d hints %+v", abilityOf(d, DEX).Score, abilityOf(d, INT).Score, d.Hints)
	}
}

func TestWithoutAPickTheEffectsThatReadItDoNotApply(t *testing.T) {
	t.Parallel()
	c := withOverlay(t, resilientOverlay("str", "dex"))
	for name, picks := range map[string][]string{
		"none":                {},
		"not in the list":     {resilientPick("ability:cha")},
		"not an ability":      {resilientPick("damage-type:fire")},
		"another feat's pick": {ScopedChoice(PickChoiceKey("feat:outro", "ability"), "ability:dex")},
	} {
		b := resilientBuild(16, picks...)
		d := Derive(b, c)
		if abilityOf(d, DEX).Score != 16 || abilityOf(d, CHA).Score != 12 || saveOf(d, DEX).Proficient || saveOf(d, CHA).Proficient || hintWith(d, "CD ") {
			t.Errorf("%s: an effect applied without a valid pick: %+v", name, d.Hints)
		}
		if j, _ := json.Marshal(d); strings.Contains(string(j), "@ability") {
			t.Errorf("%s: a token reached the sheet: %s", name, j)
		}
		problems := c.Choices(b).ChoiceProblems(true)
		key := PickChoiceKey(resilientKey, "ability")
		if !slices.ContainsFunc(problems, func(p ChoiceProblem) bool { return p.Code == ChoiceProblemMissing && p.ChoiceKey == key }) {
			t.Errorf("%s: problems = %+v, want the choice missing", name, problems)
		}
	}

	// A pick outside the list is also refused as not offered.
	problems := c.Choices(resilientBuild(16, resilientPick("ability:cha"))).ChoiceProblems(false)
	if !slices.ContainsFunc(problems, func(p ChoiceProblem) bool { return p.Code == ChoiceProblemNotOffered }) {
		t.Errorf("problems = %+v, want a pick not offered", problems)
	}
	// And a valid one leaves nothing open.
	if problems := c.Choices(resilientBuild(16, resilientPick("ability:str"))).ChoiceProblems(true); len(problems) != 0 {
		t.Errorf("problems = %+v", problems)
	}
	ch := choiceOf(t, c.Choices(resilientBuild(16)), PickChoiceKey(resilientKey, "ability"))
	if got := optionKeys(ch); !slices.Equal(got, []string{"ability:str", "ability:dex"}) || ch.Picks != 1 || ch.Missing() != 1 {
		t.Errorf("choice = %+v", ch)
	}
	if n := len(choiceOf(t, withOverlay(t, resilientOverlay()).Choices(resilientBuild(16)), PickChoiceKey(resilientKey, "ability")).Options); n != 6 {
		t.Errorf("a choice with no list offers %d abilities, want 6", n)
	}
}

func TestADamageTypePickReachesTheEffects(t *testing.T) {
	t.Parallel()
	const key = "feat:elemental" + tableSuffix
	o := featWith(key,
		Effect{Type: "choice", Choice: "damage_type", Name: "tipo", Count: 1, From: []string{"damage-type:fire", "damage-type:cold"}},
		Effect{Type: "resistance", Kind: "resist", DamageTypes: []string{"damage-type:@tipo"}},
		Effect{Type: "extra_damage", Applies: ExtraDamageAnyTarget, Value: "2", Weapon: ExtraDamageAnyWeapon, Frequency: ExtraDamageEachHit, DamageType: "damage-type:@tipo"},
	)
	o.Strict = nil
	c := withOverlay(t, o)
	build := func(picks ...string) Build {
		b := pensantus()
		b.Feats, b.FeatureChoices = []string{key}, picks
		return b
	}
	pick := ScopedChoice(PickChoiceKey(key, "tipo"), "damage-type:cold")
	d := Derive(build(pick), c)
	rs := c.Resistances(d)
	if len(rs) != 1 || !slices.Equal(rs[0].DamageTypes, []string{"damage-type:cold"}) {
		t.Errorf("resistances = %+v", rs)
	}
	if len(d.ExtraDamages) != 1 || d.ExtraDamages[0].DamageType != "damage-type:cold" {
		t.Errorf("extra damage = %+v", d.ExtraDamages)
	}
	// Not offered by the list, or not made: nothing applies and no token shows.
	for _, picks := range [][]string{{}, {ScopedChoice(PickChoiceKey(key, "tipo"), "damage-type:acid")}} {
		d := Derive(build(picks...), c)
		if rs := c.Resistances(d); len(rs) != 0 || len(d.ExtraDamages) != 0 {
			t.Errorf("picks %v: resistances %+v, extra damage %+v", picks, rs, d.ExtraDamages)
		}
	}
	ch := choiceOf(t, c.Choices(build()), PickChoiceKey(key, "tipo"))
	if !slices.Equal(optionKeys(ch), []string{"damage-type:fire", "damage-type:cold"}) {
		t.Errorf("options = %v", optionKeys(ch))
	}
	if !c.c.choiceValueOK(pick) {
		t.Error("a damage type pick is not a choice pick")
	}
	if err := validate(build(pick), c.c); err != nil {
		t.Errorf("validate() error = %v", err)
	}
}

// A feat with 60 options: the one picked applies, the others never do.
func TestAFeatWithSixtyOptions(t *testing.T) {
	t.Parallel()
	o := featWithOptions(MaxFeatureOptions, Effect{Type: "choice", Choice: "feature", Count: 1})
	o.Strict = nil
	const pickedOption = "feature:opcao-41" + tableSuffix
	o.Feats[0].Options[41].Effects = []Effect{{Type: "modifier", Target: "initiative", Mode: "add", Value: "3"}}
	c := withOverlay(t, o)
	const key = "feat:opcoes" + tableSuffix
	build := func(picks ...string) Build {
		b := pensantus()
		b.Feats, b.FeatureChoices = []string{key}, picks
		return b
	}
	base := Derive(build(), c).Initiative
	if got := Derive(build(pickedOption), c).Initiative; got != base+3 {
		t.Errorf("Initiative = %d with the option, %d without", got, base)
	}
	if got := Derive(build("feature:opcao-3"+tableSuffix), c).Initiative; got != base {
		t.Errorf("another option changed the initiative: %d, want %d", got, base)
	}
	ch := choiceOf(t, c.Choices(build()), key)
	if len(ch.Options) != MaxFeatureOptions || ch.Picks != 1 || ch.Missing() != 1 {
		t.Errorf("choice has %d options, picks %d, missing %d", len(ch.Options), ch.Picks, ch.Missing())
	}
	if problems := c.Choices(build(pickedOption)).ChoiceProblems(true); len(problems) != 0 {
		t.Errorf("problems = %+v", problems)
	}
	// A second option is past what the feat offers.
	problems := c.Choices(build(pickedOption, "feature:opcao-3"+tableSuffix)).ChoiceProblems(true)
	if !slices.ContainsFunc(problems, func(p ChoiceProblem) bool { return p.Code == ChoiceProblemNotOffered }) {
		t.Errorf("problems = %+v", problems)
	}
}

// A feat that offers an option set offers the SRD's options and the table's that joined it.
func TestAFeatOffersAnOptionSet(t *testing.T) {
	t.Parallel()
	const key = "feat:estilista" + tableSuffix
	o := optionsOverlay(doubleGuardOption())
	feat := featWith(key, Effect{Type: "choice", Choice: "feature", Count: 1, Set: "fighting_style"})
	o.Feats, o.Strict = feat.Feats, nil
	c := withOverlay(t, o)
	b := pensantus()
	b.Feats = []string{key}
	ch := choiceOf(t, c.Choices(b), key)
	if keys := optionKeys(ch); !slices.Contains(keys, doubleGuard) || !slices.Contains(keys, "feature:fighter-fighting-style-defense") {
		t.Fatalf("options = %v", keys)
	}
	baseAC := Derive(b, c).ArmorClass
	b.FeatureChoices = []string{"feature:fighter-fighting-style-defense"}
	d := Derive(b, c)
	if len(c.Choices(b).ChoiceProblems(true)) != 0 {
		t.Errorf("problems = %+v", c.Choices(b).ChoiceProblems(true))
	}
	if !slices.ContainsFunc(d.Features, func(f Feature) bool { return f.Key == "feature:fighter-fighting-style-defense" }) {
		t.Errorf("the picked style is not on the sheet: %+v", d.Features)
	}
	_ = baseAC
	b.FeatureChoices = []string{doubleGuard}
	if d := Derive(b, c); !slices.ContainsFunc(d.Features, func(f Feature) bool { return f.Key == doubleGuard }) {
		t.Errorf("the table's style is not on the sheet: %+v", d.Features)
	}
}

// In a player's view a hidden option is not offered, and a pick the sheet holds stays named.
func TestAHiddenOptionOfAFeatSet(t *testing.T) {
	t.Parallel()
	const key = "feat:estilista" + tableSuffix
	o := optionsOverlay(doubleGuardOption())
	o.Feats = featWith(key, Effect{Type: "choice", Choice: "feature", Count: 1, Set: "fighting_style"}).Feats
	o.Strict, o.Off = nil, []string{doubleGuard}
	c := withOverlay(t, o)
	player := c.ForPlayer()
	b := pensantus()
	b.Feats = []string{key}
	if keys := optionKeys(choiceOf(t, player.Choices(b), key)); slices.Contains(keys, doubleGuard) || !slices.Contains(keys, "feature:fighter-fighting-style-defense") {
		t.Errorf("a player is offered %v", keys)
	}
	b.FeatureChoices = []string{doubleGuard}
	ch := choiceOf(t, player.Choices(b), key)
	if !slices.Contains(optionKeys(ch), doubleGuard) || !slices.Equal(ch.Picked, []string{doubleGuard}) {
		t.Errorf("the held pick is not named: options %v picked %v", optionKeys(ch), ch.Picked)
	}
	if player.Hidden(doubleGuard) != true || !player.SheetHas(b, doubleGuard) {
		t.Error("the hidden option should be hidden and held")
	}
}

// The options of a feat go with the feat's hidden state.
func TestTheOptionsOfAFeatAreHiddenWithIt(t *testing.T) {
	t.Parallel()
	o := featWithOptions(2, Effect{Type: "choice", Choice: "feature", Count: 1})
	o.Strict, o.Off = nil, []string{"feat:opcoes" + tableSuffix}
	c := withOverlay(t, o)
	if !c.Hidden("feat:opcoes"+tableSuffix) || !c.Hidden("feature:opcao-0"+tableSuffix) {
		t.Error("an option of a switched-off feat is offered")
	}
	if c := withOverlay(t, featWithOptions(2, Effect{Type: "choice", Choice: "feature", Count: 1})); c.Hidden("feature:opcao-0" + tableSuffix) {
		t.Error("an option of a live feat is hidden")
	}
}

// Taking a feat with picks at a level-up asks the picks in the same step.
func TestLevelUpTakingAFeatWithPicks(t *testing.T) {
	t.Parallel()
	c := withOverlay(t, resilientOverlay("str", "dex"))
	before := torenLevelUp()
	avg := LevelUpHitPoints{Average: true}
	take := func(picks ...string) (Build, error) {
		after, err := ApplyLevelUp(before, LevelUpChoices{Class: "class:fighter", Feat: resilientKey, HitPoints: avg, FeatureChoices: picks}, c)
		if err != nil {
			t.Fatal(err)
		}
		return after, CheckLevelUp(before, after, c)
	}

	after, err := take(resilientPick("ability:dex"))
	if err != nil {
		t.Fatalf("with the pick: %v", err)
	}
	if got, was := abilityOf(Derive(after, c), DEX).Score, abilityOf(Derive(before, c), DEX).Score; got != was+1 {
		t.Errorf("Dexterity %d after the feat, %d before", got, was)
	}
	if _, err := take(); err == nil {
		t.Error("a feat with picks was taken with none")
	} else {
		wantRefusal(t, err, LevelUpReasonFeatureChoice, levelUpChoicesField)
	}
	if _, err := take(resilientPick("ability:wis")); err == nil {
		t.Error("a pick the feat does not offer was taken")
	} else {
		wantRefusal(t, err, LevelUpReasonFeatureChoice, levelUpChoicesField)
	}
}

// An Elemental Adept-shaped feat: the damage type of the roll_minimum follows the pick.
func TestARollMinimumFollowsTheDamageTypePick(t *testing.T) {
	t.Parallel()
	const key = "feat:adepto" + tableSuffix
	o := featWith(key,
		Effect{Type: "choice", Choice: "damage_type", Name: "type", Count: 1, From: []string{"damage-type:fire", "damage-type:cold"}},
		Effect{Type: "roll_minimum", Rolls: []string{RollDamage}, Value: "2", DamageType: "damage-type:@type"},
	)
	o.Strict = nil
	c := withOverlay(t, o)
	build := func(picks ...string) Build {
		b := pensantus()
		b.Feats, b.FeatureChoices = []string{key}, picks
		return b
	}
	for _, dt := range []string{"damage-type:fire", "damage-type:cold"} {
		d := Derive(build(ScopedChoice(PickChoiceKey(key, "type"), dt)), c)
		if len(d.RollMinimums) != 1 || d.RollMinimums[0].DamageType != dt || d.RollMinimums[0].Value != 2 {
			t.Errorf("pick %s: roll minimums = %+v", dt, d.RollMinimums)
		}
	}
	for _, picks := range [][]string{{}, {ScopedChoice(PickChoiceKey(key, "type"), "damage-type:acid")}} {
		if d := Derive(build(picks...), c); len(d.RollMinimums) != 0 {
			t.Errorf("picks %v: roll minimums = %+v", picks, d.RollMinimums)
		}
	}
}
