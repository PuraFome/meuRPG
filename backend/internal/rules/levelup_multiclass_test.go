package rules

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// doranFighter5 is a human Champion, Fighter 5, with the scores of the guided
// level-up's reference: Strength 16, Dexterity 14, Constitution 14, Intelligence
// 13, Wisdom 11, Charisma 9 (the human's +1 included), 44 hit points.
func doranFighter5() Build {
	return Build{
		BaseScores: map[Ability]int{STR: 15, DEX: 13, CON: 13, INT: 12, WIS: 10, CHA: 8},
		Race:       "race:human", Background: "background:acolyte",
		Classes:            []ClassLevel{{Class: "class:fighter", Subclass: "subclass:champion", Level: 5}},
		SkillProficiencies: []string{"skill:athletics", "skill:perception"},
		Armor:              "equipment:chain-mail", Shield: true,
		Weapons:        []string{"equipment:longsword"},
		FeatureChoices: []string{"feature:fighter-fighting-style-defense"},
	}
}

// wizardOne is what Doran picks to enter the Wizard: the cantrips and the
// spellbook of a first level, two spells prepared.
func wizardOneChoices() LevelUpChoices {
	return LevelUpChoices{
		Class:     "class:wizard",
		Cantrips:  []string{"spell:fire-bolt", "spell:mage-hand", "spell:minor-illusion"},
		Spells:    []string{"spell:magic-missile", "spell:sleep", "spell:shield", "spell:mage-armor", "spell:detect-magic", "spell:identify"},
		Prepared:  []string{"spell:magic-missile", "spell:shield"},
		HitPoints: LevelUpHitPoints{Average: true},
	}
}

// TestMulticlassFighterFiveTakesWizardOne is the reference multiclass level-up
// (SRD 5.1, "Multiclassing"): a Fighter 5 enters the Wizard. The experience
// follows the total level, the new class's die gives an average level (never the
// maximum), the proficiency bonus stays the total level's, the Wizard's reduced
// proficiencies are none, and the spellcasting is the Wizard's alone.
func TestMulticlassFighterFiveTakesWizardOne(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	before := doranFighter5()

	o, err := LevelUpOptions(before, "class:wizard", c)
	if err != nil {
		t.Fatalf("LevelUpOptions() error = %v", err)
	}
	if !o.NewClass || o.FromLevel != 0 || o.ToLevel != 1 || o.TotalFrom != 5 || o.TotalTo != 6 {
		t.Errorf("levels = new %v, class %d to %d, total %d to %d; want a new class 0 to 1, total 5 to 6", o.NewClass, o.FromLevel, o.ToLevel, o.TotalFrom, o.TotalTo)
	}
	if o.HitDie != 6 || o.HitPointAverage != 4 {
		t.Errorf("hit die d%d, average %d; want d6, 4 (never the maximum)", o.HitDie, o.HitPointAverage)
	}
	if o.Cantrips != 3 || o.Spells != 6 || o.SpellsKind != PreparationSpellbook || !o.Prepares || o.PreparedMaxAfter != 2 || o.MaxSpellLevel != 1 || o.SpellList != "class:wizard" {
		t.Errorf("spells = %d cantrips, %d %s, prepares %v up to %d, circle %d, list %s; want the Wizard's level 1: 3, 6 spellbook, 2, circle 1",
			o.Cantrips, o.Spells, o.SpellsKind, o.Prepares, o.PreparedMaxAfter, o.MaxSpellLevel, o.SpellList)
	}
	if o.SlotsBefore != [9]int{} || o.SlotsAfter != [9]int{2} {
		t.Errorf("slots %v to %v, want none to two 1st-circle", o.SlotsBefore, o.SlotsAfter)
	}
	if o.ProficiencyBonusBefore != 3 || o.ProficiencyBonusAfter != 3 {
		t.Errorf("proficiency bonus %d to %d, want +3 either way (total level 6)", o.ProficiencyBonusBefore, o.ProficiencyBonusAfter)
	}
	if !o.SubclassDue && o.AbilityScoreImprovement {
		t.Errorf("a first level has no Ability Score Improvement")
	}
	if o.SubclassDue || len(o.Subclasses) != 0 {
		t.Errorf("the Wizard picks its school at level 2, not 1: subclass due = %v", o.SubclassDue)
	}
	if mc := o.Multiclass; mc == nil || len(mc.Proficiencies) != 0 || mc.Skills != nil || mc.Instruments != nil || mc.CasterLevelBefore != 0 || mc.CasterLevelAfter != 1 || mc.SlotsByTable {
		t.Errorf("multiclass offer = %+v, want no proficiencies or picks, caster level 0 to 1, slots from the Wizard's own table", mc)
	}
	if !slices.ContainsFunc(o.NewFeatures, func(k NamedKey) bool { return k.Key == "feature:arcane-recovery" }) {
		t.Errorf("new features = %v, want Arcane Recovery", o.NewFeatures)
	}

	after := mustApply(t, c, before, wizardOneChoices())
	if err := CheckLevelUp(before, after, c); err != nil {
		t.Fatalf("CheckLevelUp() error = %v", err)
	}
	if err := Validate(after, c); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	d0, d1 := Derive(before, c), Derive(after, c)
	if len(d0.Issues) != 0 || len(d1.Issues) != 0 {
		t.Fatalf("issues = %v / %v", issueCodes(d0), issueCodes(d1))
	}
	if d0.HitPointsMax != 44 || d1.HitPointsMax != 50 {
		t.Errorf("hit points %d to %d, want 44 to 50: Fighter 10+2 then four levels of 6+2, and the d6 average 4+2", d0.HitPointsMax, d1.HitPointsMax)
	}
	if d1.TotalLevel != 6 || d1.ProficiencyBonus != 3 {
		t.Errorf("total level %d, proficiency bonus %d; want 6, +3", d1.TotalLevel, d1.ProficiencyBonus)
	}
	if !slices.Equal(d1.HitDice, []HitDice{{Die: 10, Count: 5}, {Die: 6, Count: 1}}) {
		t.Errorf("hit dice = %v, want 5d10 and 1d6 apart", d1.HitDice)
	}
	wiz := spellcastingOf(d1, "class:wizard")
	if wiz == nil || wiz.SaveDC != 12 || wiz.AttackBonus != 4 || wiz.PreparedMax != 2 || wiz.CantripsKnown != 3 {
		t.Errorf("Wizard casting = %+v, want DC 12, +4, 2 prepared, 3 cantrips", wiz)
	}
	if spellcastingOf(d1, "class:fighter") != nil {
		t.Error("the Champion does not cast")
	}
	if !slices.Equal(d1.SpellSlots, []int{2, 0, 0, 0, 0, 0, 0, 0, 0}) || d1.PactMagic != nil {
		t.Errorf("slots = %v, pact %v; want two 1st-circle slots", d1.SpellSlots, d1.PactMagic)
	}
	if !slices.Equal(d0.Proficiencies, d1.Proficiencies) {
		t.Errorf("proficiencies changed: %v to %v; the Wizard's multiclass table gives none", d0.Proficiencies, d1.Proficiencies)
	}
	if got := d1.Classes; len(got) != 2 || got[0].Level != 5 || got[1].ClassKey != "class:wizard" || got[1].Level != 1 {
		t.Errorf("classes = %+v, want Fighter 5 and Wizard 1, in that order", got)
	}
	// The saving throws are the starting class's only.
	for _, s := range d1.SavingThrows {
		if want := s.Ability == STR || s.Ability == CON; s.Proficient != want {
			t.Errorf("save %s proficient = %v, want %v (the Fighter's)", s.Ability, s.Proficient, want)
		}
	}
	// The input is never changed.
	if len(before.Classes) != 1 || before.Classes[0].Level != 5 {
		t.Error("ApplyLevelUp changed the Build it was given")
	}
	// Nothing but the Wizard's level 1 changed.
	if !slices.Equal(after.FeatureChoices, before.FeatureChoices) || !slices.Equal(after.SkillProficiencies, before.SkillProficiencies) || after.Armor != before.Armor || !slices.Equal(after.Weapons, before.Weapons) {
		t.Error("the level changed what is locked: equipment, skills or options")
	}

	// The summary: only the Wizard counts for the caster level, and nothing is new
	// among the proficiencies.
	sum, ok := MulticlassSummaryOf(before, after, "class:wizard", c)
	if !ok || sum.NewClass != "class:wizard" || sum.CasterLevelBefore != 0 || sum.CasterLevelAfter != 1 || sum.SlotsByTable || len(sum.ProficienciesGained) != 0 || len(sum.Exceptions) != 0 {
		t.Errorf("summary = %+v (%v)", sum, ok)
	}
	if _, ok := MulticlassSummaryOf(before, before, "", c); ok {
		t.Error("a single-class sheet has no multiclass summary")
	}
}

// TestMulticlassGolden keeps the Fighter 5 and the Fighter 5 / Wizard 1 sheets
// of the reference level-up, whole, in testdata/golden/multiclass-doran.json.
// After a deliberate change, rewrite it with
// `go test ./internal/rules -run Golden -update` and review the diff.
func TestMulticlassGolden(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	before := doranFighter5()
	after := mustApply(t, c, before, wizardOneChoices())
	sheet := func(b Build) Derived {
		d := Derive(b, c)
		for i := range d.Features {
			d.Features[i].Description = nil
		}
		return d
	}
	got, err := json.MarshalIndent(map[string]Derived{"before": sheet(before), "after": sheet(after)}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	path := filepath.Join("testdata", "golden", "multiclass-doran.json")
	if *update {
		if err := os.WriteFile(path, got, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run with -update to create it)", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("the Fighter 5 / Wizard 1 sheets differ from %s; if the change is right, run with -update and review the diff.\ngot:\n%s", path, got)
	}
}

// prerequisiteTable is SRD 5.1 "Multiclassing", "Prerequisites": the ability score
// a class asks for, 13 in each, all of them unless anyOf.
var prerequisiteTable = []struct {
	class     string
	abilities []Ability
	anyOf     bool
}{
	{"class:barbarian", []Ability{STR}, false},
	{"class:bard", []Ability{CHA}, false},
	{"class:cleric", []Ability{WIS}, false},
	{"class:druid", []Ability{WIS}, false},
	{"class:fighter", []Ability{STR, DEX}, true},
	{"class:monk", []Ability{DEX, WIS}, false},
	{"class:paladin", []Ability{STR, CHA}, false},
	{"class:ranger", []Ability{DEX, WIS}, false},
	{"class:rogue", []Ability{DEX}, false},
	{"class:sorcerer", []Ability{CHA}, false},
	{"class:warlock", []Ability{CHA}, false},
	{"class:wizard", []Ability{INT}, false},
}

// asRefusal is the *LevelUpError an error holds.
func asRefusal(err error) *LevelUpError {
	le, _ := errors.AsType[*LevelUpError](err)
	return le
}

// wizardWith is a Wizard 1 (the character already has a class whose prerequisite
// is Intelligence 13) with these final scores: the human's +1 is taken out of the
// base scores.
func wizardWith(scores map[Ability]int) Build {
	base := map[Ability]int{}
	for _, a := range AllAbilities() {
		base[a] = scores[a] - 1
	}
	return Build{
		BaseScores: base, Race: "race:human", Background: "background:acolyte",
		Classes:            []ClassLevel{{Class: "class:wizard", Level: 1}},
		SkillProficiencies: []string{"skill:arcana", "skill:history"},
		Cantrips:           []string{"spell:fire-bolt", "spell:mage-hand", "spell:light"},
		SpellsKnown:        []string{"spell:magic-missile", "spell:sleep", "spell:shield", "spell:mage-armor", "spell:detect-magic", "spell:identify"},
		SpellsPrepared:     []string{"spell:magic-missile"},
	}
}

// TestMulticlassPrerequisites: every class of the SRD as a new class, with the
// score 1 below the minimum (refused) and equal to it (accepted), the Fighter's
// "or", and the "and" of the Monk, the Paladin and the Ranger. The value judged is
// the final score of the sheet: the race's bonus counts.
func TestMulticlassPrerequisites(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	const minimum = 13
	for _, row := range prerequisiteTable {
		t.Run(row.class, func(t *testing.T) {
			t.Parallel()
			// The class the character has meets its own prerequisite: the Wizard's
			// Intelligence, or the Rogue's Dexterity when the Wizard is the one tested.
			have := map[Ability]int{INT: minimum}
			if row.class == "class:wizard" {
				have = map[Ability]int{DEX: minimum}
			}
			scores := func(over map[Ability]int) map[Ability]int {
				s := map[Ability]int{STR: 8, DEX: 8, CON: 8, INT: 8, WIS: 8, CHA: 8}
				for a, v := range have {
					s[a] = v
				}
				for a, v := range over {
					s[a] = v
				}
				return s
			}
			build := func(over map[Ability]int) Build {
				b := wizardWith(scores(over))
				if row.class == "class:wizard" {
					b.Classes = []ClassLevel{{Class: "class:rogue", Level: 1}}
				}
				return b
			}
			// Every ability at the minimum: accepted.
			all := map[Ability]int{}
			for _, a := range row.abilities {
				all[a] = minimum
			}
			if _, err := LevelUpOptions(build(all), row.class, c); err != nil {
				t.Errorf("all at %d: LevelUpOptions() error = %v", minimum, err)
			}
			// One below the minimum in each ability: refused when that ability is
			// needed (every ability of an "and", the only one of an "or"... the "or"
			// needs both to be low).
			for _, low := range row.abilities {
				over := map[Ability]int{}
				for _, a := range row.abilities {
					over[a] = minimum
				}
				over[low] = minimum - 1
				_, err := LevelUpOptions(build(over), row.class, c)
				if row.anyOf {
					if err != nil {
						t.Errorf("%s at %d with the other at %d: error = %v, want accepted (an \"or\")", low, minimum-1, minimum, err)
					}
					continue
				}
				wantRefusal(t, err, LevelUpReasonMulticlassPrerequisite, "class_key")
				le := asRefusal(err)
				if le.ClassKey != row.class || le.Ability != low || le.Minimum != minimum || le.Have != minimum-1 {
					t.Errorf("%s one below: refusal = %+v, want class %s, %s, minimum %d, have %d", low, le, row.class, low, minimum, minimum-1)
				}
			}
			if row.anyOf {
				over := map[Ability]int{}
				for _, a := range row.abilities {
					over[a] = minimum - 1
				}
				_, err := LevelUpOptions(build(over), row.class, c)
				wantRefusal(t, err, LevelUpReasonMulticlassPrerequisite, "class_key")
			}
		})
	}
	// The final score is what counts: a base 12 with the human's +1 is 13, and an
	// increase of the sheet counts too.
	b := wizardWith(map[Ability]int{STR: 12, DEX: 8, CON: 8, INT: 13, WIS: 8, CHA: 8})
	if _, err := LevelUpOptions(b, "class:barbarian", c); err == nil {
		t.Error("Strength 12 entered the Barbarian")
	}
	b.ExtraAbilityBonuses = map[Ability]int{STR: 1}
	if _, err := LevelUpOptions(b, "class:barbarian", c); err != nil {
		t.Errorf("Strength 12 with an increase of 1: %v", err)
	}
}

// TestMulticlassPrerequisitesOfTheClassesTheCharacterHas: the main ability of every
// class the character has counts, not only the new class's (SRD 5.1,
// "Multiclassing", "Prerequisites": the current class and the new one).
func TestMulticlassPrerequisitesOfTheClassesTheCharacterHas(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	fighter := func(str, dex int) Build {
		b := doranFighter5()
		b.BaseScores[STR], b.BaseScores[DEX] = str-1, dex-1
		return b
	}
	// A Fighter with Strength 12 and Dexterity 12 enters no class, though it
	// has the Intelligence for the Wizard; it still levels the Fighter.
	_, err := LevelUpOptions(fighter(12, 12), "class:wizard", c)
	wantRefusal(t, err, LevelUpReasonMulticlassPrerequisiteCurrent, "class_key")
	if le := asRefusal(err); le.ClassKey != "class:fighter" || le.Minimum != 13 || le.Have != 12 {
		t.Errorf("refusal = %+v, want the Fighter's 13, have 12", le)
	}
	if _, err := LevelUpOptions(fighter(12, 12), "class:fighter", c); err != nil {
		t.Errorf("the Fighter's next level was refused: %v", err)
	}
	// Strength 13 is enough ("or"), and so is Dexterity 13 alone.
	for name, b := range map[string]Build{"Strength 13": fighter(13, 8), "Dexterity 13": fighter(8, 13)} {
		b.BaseScores[INT] = 12
		if _, err := LevelUpOptions(b, "class:wizard", c); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}

	// A third class asks for the prerequisite of the three: Fighter 5 / Wizard 1
	// takes the Rogue with Dexterity 13, Intelligence 13, and fails when the
	// Intelligence falls to 12.
	b := mustApply(t, c, doranFighter5(), wizardOneChoices())
	if _, err := LevelUpOptions(b, "class:rogue", c); err != nil {
		t.Fatalf("Fighter 5 / Wizard 1 to the Rogue: %v", err)
	}
	b.BaseScores[INT] = 11
	_, err = LevelUpOptions(b, "class:rogue", c)
	wantRefusal(t, err, LevelUpReasonMulticlassPrerequisiteCurrent, "class_key")
	if le := asRefusal(err); le.ClassKey != "class:wizard" || le.Ability != INT || le.Have != 12 {
		t.Errorf("refusal = %+v, want the Wizard's Intelligence, have 12", le)
	}

	// The level cap: 20 in all, and a class at 20.
	top := doranFighter5()
	top.Classes[0].Level = 19
	top.Classes = append(top.Classes, ClassLevel{Class: "class:wizard", Level: 1})
	if _, err := LevelUpOptions(top, "class:rogue", c); err == nil {
		t.Error("a level 20 character entered a third class")
	} else {
		wantRefusal(t, err, LevelUpReasonMaxLevel, "full.classes")
	}
	top.Classes = top.Classes[:1]
	top.Classes[0].Level = 20
	if _, err := LevelUpOptions(top, "class:wizard", c); err == nil {
		t.Error("a level 20 Fighter entered the Wizard")
	}
}

// TestMulticlassClassChoices: the class step lists the classes the character has
// (their next level), then the other eleven, with what each asks for and why it
// is closed.
func TestMulticlassClassChoices(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	got := LevelUpClasses(doranFighter5(), c)
	if len(got) != 12 || got[0].Class != "class:fighter" || got[0].New || got[0].FromLevel != 5 || got[0].ToLevel != 6 || !got[0].Available || got[0].SubclassNamePT != "Campeão" {
		t.Fatalf("classes = %+v", got)
	}
	open := map[string]bool{"class:barbarian": true, "class:rogue": true, "class:wizard": true}
	for _, ch := range got[1:] {
		if !ch.New || ch.FromLevel != 0 || ch.ToLevel != 1 {
			t.Errorf("%s = %+v, want a new class from 0 to 1", ch.Class, ch)
		}
		if ch.Available != open[ch.Class] {
			t.Errorf("%s available = %v (%q), want %v: Doran has Strength 16, Dexterity 14, Intelligence 13 and nothing else at 13", ch.Class, ch.Available, ch.Unavailable, open[ch.Class])
		}
		if !ch.Available && ch.Unavailable != UnavailablePrerequisite {
			t.Errorf("%s unavailable = %q, want %q", ch.Class, ch.Unavailable, UnavailablePrerequisite)
		}
	}
	// In Portuguese order after the classes the character has.
	var names []string
	for _, ch := range got[1:] {
		names = append(names, ch.ClassNamePT)
	}
	if !slices.IsSortedFunc(names, comparePT) {
		t.Errorf("new classes = %v, want them sorted", names)
	}
	// The monk asks for two abilities, and says which one is short.
	for _, ch := range got {
		if ch.Class != "class:monk" {
			continue
		}
		if ch.Prerequisite.AnyOf || len(ch.Prerequisite.Requirements) != 2 || ch.Prerequisite.Met {
			t.Errorf("monk prerequisite = %+v", ch.Prerequisite)
		}
		wis := ch.Prerequisite.Requirements[1]
		if wis.Ability != WIS || wis.Minimum != 13 || wis.Have != 11 || wis.Met {
			t.Errorf("monk Wisdom = %+v, want 13, have 11, not met", wis)
		}
	}

	// A class the character has that does not meet its own prerequisite closes every
	// new class, and says so; its own next level stays open.
	marlo := doranFighter5()
	marlo.BaseScores[STR], marlo.BaseScores[DEX] = 11, 11
	for _, ch := range LevelUpClasses(marlo, c) {
		switch {
		case ch.Class == "class:fighter" && (!ch.Available || ch.Prerequisite.Met):
			t.Errorf("the Fighter = %+v, want its next level open and its prerequisite unmet", ch)
		case ch.New && (ch.Available || ch.Unavailable != UnavailablePrerequisiteCurrent):
			t.Errorf("%s = available %v (%q), want closed by the class the character has", ch.Class, ch.Available, ch.Unavailable)
		}
	}

	// At level 20 nothing is open.
	top := doranFighter5()
	top.Classes[0].Level = 20
	for _, ch := range LevelUpClasses(top, c) {
		if ch.Available || ch.Unavailable != UnavailableMaxLevel {
			t.Errorf("%s at level 20 = available %v (%q)", ch.Class, ch.Available, ch.Unavailable)
		}
	}
	// A subclass due is announced, for a class the character has and for a new one.
	wiz := mustApply(t, c, doranFighter5(), wizardOneChoices())
	for _, ch := range LevelUpClasses(wiz, c) {
		if ch.Class == "class:wizard" && (!ch.SubclassDue || ch.FromLevel != 1) {
			t.Errorf("the Wizard 1 = %+v, want its school due at level 2", ch)
		}
		if ch.Class == "class:cleric" && !ch.SubclassDue {
			t.Errorf("the Cleric as a new class = %+v, want its domain due at level 1", ch)
		}
	}
}

// multiclassSlotTable is SRD 5.1 "Multiclassing", "Multiclass Spellcaster: Spell
// Slots per Spell Level": the slots of each spell level (1st to 9th) by caster
// level, 1 to 20.
var multiclassSlotTable = [20][9]int{
	{2},
	{3},
	{4, 2},
	{4, 3},
	{4, 3, 2},
	{4, 3, 3},
	{4, 3, 3, 1},
	{4, 3, 3, 2},
	{4, 3, 3, 3, 1},
	{4, 3, 3, 3, 2},
	{4, 3, 3, 3, 2, 1},
	{4, 3, 3, 3, 2, 1},
	{4, 3, 3, 3, 2, 1, 1},
	{4, 3, 3, 3, 2, 1, 1},
	{4, 3, 3, 3, 2, 1, 1, 1},
	{4, 3, 3, 3, 2, 1, 1, 1},
	{4, 3, 3, 3, 2, 1, 1, 1, 1},
	{4, 3, 3, 3, 3, 1, 1, 1, 1},
	{4, 3, 3, 3, 3, 2, 1, 1, 1},
	{4, 3, 3, 3, 3, 2, 2, 1, 1},
}

// pactTable is SRD 5.1 Warlock, "Pact Magic": the slots and their level by warlock
// level.
var pactTable = [20][2]int{
	{1, 1},
	{2, 1},
	{2, 2},
	{2, 2},
	{2, 3},
	{2, 3},
	{2, 4},
	{2, 4},
	{2, 5},
	{2, 5},
	{3, 5},
	{3, 5},
	{3, 5},
	{3, 5},
	{3, 5},
	{3, 5},
	{4, 5},
	{4, 5},
	{4, 5},
	{4, 5},
}

// slotCaster is a class that has the Spellcasting feature, by how much of its
// levels count for the caster level.
type slotCaster struct {
	class    string
	subclass string // the third casters need theirs to cast
	divisor  int    // 1 full, 2 half, 3 third
	first    int    // the class level at which it casts
}

var slotCasters = []slotCaster{
	{"class:bard", "", 1, 1},
	{"class:cleric", "", 1, 1},
	{"class:druid", "", 1, 1},
	{"class:sorcerer", "", 1, 1},
	{"class:wizard", "", 1, 1},
	{"class:paladin", "", 2, 2},
	{"class:ranger", "", 2, 2},
	{"class:fighter", "subclass:cavaleiro-runico@mesa", 3, 3},
	{"class:rogue", "subclass:trapaceiro-mistico@mesa", 3, 3},
}

// slotsBuild is a Build with these classes at these levels, for the slots only.
func slotsBuild(classes ...ClassLevel) Build {
	return Build{
		BaseScores: map[Ability]int{STR: 10, DEX: 10, CON: 10, INT: 10, WIS: 10, CHA: 10},
		Race:       "race:human", Background: "background:acolyte", Classes: classes,
	}
}

func casterLevel(cs slotCaster, level int) ClassLevel {
	return ClassLevel{Class: cs.class, Subclass: cs.subclass, Level: level}
}

// thirdCasterContent is the SRD with the table subclasses that cast as a third
// caster: the SRD 5.1 has no Eldritch Knight or Arcane Trickster, so they are table
// content.
func thirdCasterContent(t testing.TB) *Content {
	t.Helper()
	srd := loadForTest(t)
	return withOverlayOn(t, srd, genOverlay(t, srd))
}

// TestMulticlassSpellSlots: the multiclass spellcaster table for every pair of
// casting classes, at every pair of levels with a total up to 20, and for the
// three kinds together. Each class counts all its levels (full), half rounded
// down (half) or a third rounded down (third, with the subclass that casts); a
// class that does not cast yet adds nothing. With one casting class the slots
// are that class's own table, which is not this one.
func TestMulticlassSpellSlots(t *testing.T) {
	t.Parallel()
	c := thirdCasterContent(t)
	check := func(t *testing.T, classes ...ClassLevel) {
		t.Helper()
		level, casting := 0, 0
		for _, cl := range classes {
			for _, cs := range slotCasters {
				if cs.class == cl.Class && cl.Level >= cs.first {
					casting++
					level += cl.Level / cs.divisor
				}
			}
		}
		if casting < 2 {
			return
		}
		got := Derive(slotsBuild(classes...), c).SpellSlots
		if want := multiclassSlotTable[level-1]; !slices.Equal(got, want[:]) {
			t.Errorf("%v: caster level %d: slots = %v, want %v", classes, level, got, want)
		}
	}
	for i, a := range slotCasters {
		for _, b := range slotCasters[i+1:] {
			t.Run(a.class+"+"+b.class, func(t *testing.T) {
				t.Parallel()
				for la := 1; la <= 19; la++ {
					for lb := 1; la+lb <= 20; lb++ {
						check(t, casterLevel(a, la), casterLevel(b, lb))
					}
				}
			})
		}
	}
	// One of each kind: the Wizard (full), the Paladin (half) and the Eldritch Knight (third).
	t.Run("full+half+third", func(t *testing.T) {
		t.Parallel()
		for w := 1; w <= 18; w++ {
			for p := 1; w+p <= 19; p++ {
				for k := 1; w+p+k <= 20; k++ {
					check(t, ClassLevel{Class: "class:wizard", Level: w}, ClassLevel{Class: "class:paladin", Level: p}, casterLevel(slotCasters[7], k))
				}
			}
		}
	})
	// The cases the table rows of the guided level-up show.
	for name, tt := range map[string]struct {
		classes []ClassLevel
		want    [9]int
	}{
		"Champion 5 and Wizard 3: the Champion does not cast": {[]ClassLevel{{Class: "class:fighter", Subclass: "subclass:champion", Level: 5}, {Class: "class:wizard", Level: 3}}, [9]int{4, 2}},
		"Eldritch Knight 5 and Wizard 3: 1 + 3":               {[]ClassLevel{casterLevel(slotCasters[7], 5), {Class: "class:wizard", Level: 3}}, [9]int{4, 3}},
		"Paladin 4 and Wizard 3: 2 + 3":                       {[]ClassLevel{{Class: "class:paladin", Level: 4}, {Class: "class:wizard", Level: 3}}, [9]int{4, 3, 2}},
		"Cleric 3 and Wizard 3: 3 + 3":                        {[]ClassLevel{{Class: "class:cleric", Level: 3}, {Class: "class:wizard", Level: 3}}, [9]int{4, 3, 3}},
		"Paladin 1 and Wizard 2: the Paladin adds nothing":    {[]ClassLevel{{Class: "class:paladin", Level: 1}, {Class: "class:wizard", Level: 2}}, [9]int{3}},
		"Paladin 3 and Ranger 3: one each":                    {[]ClassLevel{{Class: "class:paladin", Level: 3}, {Class: "class:ranger", Level: 3}}, [9]int{3}},
	} {
		if got := Derive(slotsBuild(tt.classes...), c).SpellSlots; !slices.Equal(got, tt.want[:]) {
			t.Errorf("%s: slots = %v, want %v", name, got, tt.want)
		}
	}
	// Level 20 is the table's last row: two full casters past 20 are capped by the
	// total level, so 11 and 9 give the row of caster level 20.
	top := Derive(slotsBuild(ClassLevel{Class: "class:wizard", Level: 11}, ClassLevel{Class: "class:cleric", Level: 9}), c).SpellSlots
	if want := multiclassSlotTable[19]; !slices.Equal(top, want[:]) {
		t.Errorf("Wizard 11 and Cleric 9: slots = %v, want %v", top, want)
	}
}

// TestMulticlassPactMagicIsApart: the Warlock's slots are its own, whatever else
// the character casts, and the multiclass table never counts its levels (SRD 5.1,
// "Multiclassing", "Spellcasting" and "Pact Magic").
func TestMulticlassPactMagicIsApart(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	for w := 1; w <= 19; w++ {
		warlock := ClassLevel{Class: "class:warlock", Level: w}
		pact := pactTable[w-1]
		// Alone, and with a non-caster: the pact slots and no others.
		for _, other := range []ClassLevel{{}, {Class: "class:barbarian", Level: 1}} {
			classes := []ClassLevel{warlock}
			if other.Class != "" {
				classes = append(classes, other)
			}
			d := Derive(slotsBuild(classes...), c)
			if d.PactMagic == nil || d.PactMagic.Slots != pact[0] || d.PactMagic.SlotLevel != pact[1] {
				t.Errorf("Warlock %d: pact = %+v, want %d slots of level %d", w, d.PactMagic, pact[0], pact[1])
			}
			if !slices.Equal(d.SpellSlots, make([]int, 9)) {
				t.Errorf("Warlock %d alone: slots = %v, want none", w, d.SpellSlots)
			}
		}
		// With a Wizard of the rest of the levels: the table sees only the Wizard.
		d := Derive(slotsBuild(warlock, ClassLevel{Class: "class:wizard", Level: 20 - w}), c)
		if want := multiclassSlotTable[20-w-1]; !slices.Equal(d.SpellSlots, want[:]) && 20-w >= 1 {
			t.Errorf("Warlock %d / Wizard %d: slots = %v, want the Wizard's %v", w, 20-w, d.SpellSlots, want)
		}
		if d.PactMagic == nil || d.PactMagic.Slots != pact[0] || d.PactMagic.SlotLevel != pact[1] {
			t.Errorf("Warlock %d / Wizard %d: pact = %+v, want %v", w, 20-w, d.PactMagic, pact)
		}
	}
	// A Warlock and a single half caster: the half caster's own table, and the pact.
	d := Derive(slotsBuild(ClassLevel{Class: "class:warlock", Level: 2}, ClassLevel{Class: "class:paladin", Level: 5}), c)
	if want := [9]int{4, 2}; !slices.Equal(d.SpellSlots, want[:]) || d.PactMagic == nil || d.PactMagic.Slots != 2 || d.PactMagic.SlotLevel != 1 {
		t.Errorf("Warlock 2 / Paladin 5: slots = %v, pact = %+v, want the Paladin's 4, 2 and two 1st-level pact slots", d.SpellSlots, d.PactMagic)
	}
	// A Warlock with two casters: the table of the two, the pact apart.
	d = Derive(slotsBuild(ClassLevel{Class: "class:warlock", Level: 3}, ClassLevel{Class: "class:cleric", Level: 3}, ClassLevel{Class: "class:wizard", Level: 3}), c)
	if want := [9]int{4, 3, 3}; !slices.Equal(d.SpellSlots, want[:]) || d.PactMagic == nil || d.PactMagic.SlotLevel != 2 {
		t.Errorf("Warlock 3 / Cleric 3 / Wizard 3: slots = %v, pact = %+v", d.SpellSlots, d.PactMagic)
	}
}

// TestMulticlassCasterLevel: the level the summary shows, by the same rule.
func TestMulticlassCasterLevel(t *testing.T) {
	t.Parallel()
	c := thirdCasterContent(t)
	for name, tt := range map[string]struct {
		classes        []ClassLevel
		level, casters int
	}{
		"Champion 5 and Wizard 1":        {[]ClassLevel{{Class: "class:fighter", Subclass: "subclass:champion", Level: 5}, {Class: "class:wizard", Level: 1}}, 1, 1},
		"Paladin 5 and Wizard 3":         {[]ClassLevel{{Class: "class:paladin", Level: 5}, {Class: "class:wizard", Level: 3}}, 5, 2},
		"Eldritch Knight 8 and Ranger 3": {[]ClassLevel{casterLevel(slotCasters[7], 8), {Class: "class:ranger", Level: 3}}, 3, 2},
		"Warlock 5 and Bard 2":           {[]ClassLevel{{Class: "class:warlock", Level: 5}, {Class: "class:bard", Level: 2}}, 2, 1},
		"Fighter 4 without a subclass":   {[]ClassLevel{{Class: "class:fighter", Level: 4}, {Class: "class:cleric", Level: 1}}, 1, 1},
	} {
		if level, casters := CasterLevel(Build{Classes: tt.classes}, c); level != tt.level || casters != tt.casters {
			t.Errorf("%s: caster level %d with %d casters, want %d with %d", name, level, casters, tt.level, tt.casters)
		}
	}
}

// TestMulticlassExceptions: the four exceptions of SRD 5.1, "Multiclassing",
// "Class Features".
func TestMulticlassExceptions(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	with := func(classes ...ClassLevel) Build {
		b := slotsBuild(classes...)
		b.BaseScores = map[Ability]int{STR: 14, DEX: 14, CON: 16, INT: 10, WIS: 18, CHA: 14}
		return b
	}

	t.Run("Extra Attack does not add", func(t *testing.T) {
		t.Parallel()
		for name, tt := range map[string]struct {
			classes []ClassLevel
			want    int
		}{
			"Fighter 5":                         {[]ClassLevel{{Class: "class:fighter", Level: 5}}, 2},
			"Fighter 5 and Paladin 5":           {[]ClassLevel{{Class: "class:fighter", Level: 5}, {Class: "class:paladin", Level: 5}}, 2},
			"Barbarian 5 and Monk 5":            {[]ClassLevel{{Class: "class:barbarian", Level: 5}, {Class: "class:monk", Level: 5}}, 2},
			"Ranger 5 and Paladin 5":            {[]ClassLevel{{Class: "class:ranger", Level: 5}, {Class: "class:paladin", Level: 5}}, 2},
			"Fighter 4 and Paladin 4":           {[]ClassLevel{{Class: "class:fighter", Level: 4}, {Class: "class:paladin", Level: 4}}, 1},
			"Fighter 11 and Paladin 5":          {[]ClassLevel{{Class: "class:fighter", Level: 11}, {Class: "class:paladin", Level: 5}}, 3},
			"Paladin 5 and Fighter 11":          {[]ClassLevel{{Class: "class:paladin", Level: 5}, {Class: "class:fighter", Level: 11}}, 3},
			"Fighter 5 and Monk 5 and Ranger 5": {[]ClassLevel{{Class: "class:fighter", Level: 5}, {Class: "class:monk", Level: 5}, {Class: "class:ranger", Level: 5}}, 2},
		} {
			if got := Derive(with(tt.classes...), c).AttacksPerAction; got != tt.want {
				t.Errorf("%s: attacks per action = %d, want %d", name, got, tt.want)
			}
		}
	})

	t.Run("Channel Divinity gives effects, not uses", func(t *testing.T) {
		t.Parallel()
		uses := func(classes ...ClassLevel) int {
			for _, r := range Derive(with(classes...), c).Resources {
				if r.Key == "channel_divinity" {
					return r.Max
				}
			}
			return 0
		}
		for name, tt := range map[string]struct {
			classes []ClassLevel
			want    int
		}{
			"Cleric 6 and Paladin 4":  {[]ClassLevel{{Class: "class:cleric", Subclass: "subclass:life", Level: 6}, {Class: "class:paladin", Level: 4}}, 2},
			"Paladin 4 and Cleric 6":  {[]ClassLevel{{Class: "class:paladin", Level: 4}, {Class: "class:cleric", Subclass: "subclass:life", Level: 6}}, 2},
			"Cleric 2 and Paladin 3":  {[]ClassLevel{{Class: "class:cleric", Subclass: "subclass:life", Level: 2}, {Class: "class:paladin", Level: 3}}, 1},
			"Cleric 5 and Paladin 6":  {[]ClassLevel{{Class: "class:cleric", Subclass: "subclass:life", Level: 5}, {Class: "class:paladin", Level: 6}}, 1},
			"Cleric 18 and Paladin 2": {[]ClassLevel{{Class: "class:cleric", Subclass: "subclass:life", Level: 18}, {Class: "class:paladin", Level: 2}}, 3},
			"Paladin 3":               {[]ClassLevel{{Class: "class:paladin", Level: 3}}, 1},
		} {
			if got := uses(tt.classes...); got != tt.want {
				t.Errorf("%s: Channel Divinity uses = %d, want %d", name, got, tt.want)
			}
		}
	})

	t.Run("Unarmored Defense is gained once", func(t *testing.T) {
		t.Parallel()
		ac := func(shield bool, classes ...ClassLevel) int {
			b := with(classes...)
			b.Shield = shield
			return Derive(b, c).ArmorClass
		}
		barbarian, monk := ClassLevel{Class: "class:barbarian", Level: 1}, ClassLevel{Class: "class:monk", Level: 1}
		// Dexterity +2, Constitution +3, Wisdom +4.
		for name, tt := range map[string]struct {
			shield  bool
			classes []ClassLevel
			want    int
		}{
			"Barbarian alone: 10 + Dex + Con":        {false, []ClassLevel{barbarian}, 15},
			"Monk alone: 10 + Dex + Wis":             {false, []ClassLevel{monk}, 16},
			"Barbarian first, then Monk: the first":  {false, []ClassLevel{barbarian, monk}, 15},
			"Monk first, then Barbarian: the first":  {false, []ClassLevel{monk, barbarian}, 16},
			"Barbarian first keeps it with a shield": {true, []ClassLevel{barbarian, monk}, 17},
			"Monk first with a shield has none":      {true, []ClassLevel{monk, barbarian}, 14},
		} {
			if got := ac(tt.shield, tt.classes...); got != tt.want {
				t.Errorf("%s: armor class = %d, want %d", name, got, tt.want)
			}
		}
	})
}

// multiclassProficiencies is SRD 5.1 "Multiclassing", "Proficiencies": what a class
// gives when it is not the first, by proficiency key. The picks (a skill, the
// Bard's instrument) are apart.
var multiclassProficiencies = map[string][]string{
	"class:barbarian": {"proficiency:shields", "proficiency:simple-weapons", "proficiency:martial-weapons"},
	"class:bard":      {"proficiency:light-armor"},
	"class:cleric":    {"proficiency:light-armor", "proficiency:medium-armor", "proficiency:shields"},
	"class:druid":     {"proficiency:light-armor", "proficiency:medium-armor", "proficiency:shields"},
	"class:fighter":   {"proficiency:light-armor", "proficiency:medium-armor", "proficiency:shields", "proficiency:simple-weapons", "proficiency:martial-weapons"},
	"class:monk":      {"proficiency:simple-weapons", "proficiency:shortswords"},
	"class:paladin":   {"proficiency:light-armor", "proficiency:medium-armor", "proficiency:shields", "proficiency:simple-weapons", "proficiency:martial-weapons"},
	"class:ranger":    {"proficiency:light-armor", "proficiency:medium-armor", "proficiency:shields", "proficiency:simple-weapons", "proficiency:martial-weapons"},
	"class:rogue":     {"proficiency:light-armor", "proficiency:thieves-tools"},
	"class:sorcerer":  nil,
	"class:warlock":   {"proficiency:light-armor", "proficiency:simple-weapons"},
	"class:wizard":    nil,
}

// multiclassSkillPicks is how many skills a class asks the player to pick when it
// is not the first, and from where ("" is any skill).
var multiclassSkillPicks = map[string]struct {
	count int
	from  []string
}{
	"class:bard":   {1, nil},
	"class:ranger": {1, []string{"skill:animal-handling", "skill:athletics", "skill:insight", "skill:investigation", "skill:nature", "skill:perception", "skill:stealth", "skill:survival"}},
	"class:rogue": {1, []string{
		"skill:acrobatics", "skill:athletics", "skill:deception", "skill:insight", "skill:intimidation", "skill:investigation",
		"skill:perception", "skill:performance", "skill:persuasion", "skill:sleight-of-hand", "skill:stealth",
	}},
}

// proficiencyKeys lists the keys of a Derived.Proficiencies.
func proficiencyKeys(d Derived) []string {
	var keys []string
	for _, p := range d.Proficiencies {
		keys = append(keys, p.Key)
	}
	return keys
}

// TestMulticlassProficiencies: every class as the second class gives only the
// reduced list of the multiclass table, whoever the first class is, and never the
// first class's saving throws; as the first class it gives its whole list.
func TestMulticlassProficiencies(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	for _, first := range []string{"class:wizard", "class:fighter"} {
		base := Derive(slotsBuild(ClassLevel{Class: first, Level: 1}), c)
		have := map[string]bool{}
		for _, k := range proficiencyKeys(base) {
			have[k] = true
		}
		for _, second := range sortedKeys(c.c.classes) {
			if second == first {
				continue
			}
			t.Run(first+"+"+second, func(t *testing.T) {
				t.Parallel()
				d := Derive(slotsBuild(ClassLevel{Class: first, Level: 1}, ClassLevel{Class: second, Level: 1}), c)
				got := map[string]bool{}
				for _, k := range proficiencyKeys(d) {
					if !have[k] {
						got[k] = true
					}
				}
				want := map[string]bool{}
				for _, k := range multiclassProficiencies[second] {
					if !have[k] {
						want[k] = true
					}
				}
				if len(got) != len(want) {
					t.Errorf("new proficiencies = %v, want %v", got, want)
				}
				for k := range want {
					if !got[k] {
						t.Errorf("missing %s; new proficiencies = %v", k, got)
					}
				}
				// The saving throws stay the first class's.
				for i, s := range d.SavingThrows {
					if s.Proficient != base.SavingThrows[i].Proficient {
						t.Errorf("save %s proficient = %v after the second class, was %v", s.Ability, s.Proficient, base.SavingThrows[i].Proficient)
					}
				}
			})
		}
	}
	// The table covers all twelve classes, with the choices the SRD gives.
	for key := range c.c.classes {
		if _, ok := multiclassProficiencies[key]; !ok {
			t.Errorf("%s has no row in the test's multiclass table", key)
		}
		mc := c.c.classes[key].Multiclass
		pick := multiclassSkillPicks[key]
		if pick.count == 0 && mc.SkillChoices != nil || pick.count != 0 && (mc.SkillChoices == nil || mc.SkillChoices.Choose != pick.count) {
			t.Errorf("%s skill choices = %+v, want %d", key, mc.SkillChoices, pick.count)
		}
		if pick.from != nil && !slices.Equal(mc.SkillChoices.From, pick.from) {
			t.Errorf("%s skill list = %v, want %v", key, mc.SkillChoices.From, pick.from)
		}
		if (key == "class:bard") != (mc.InstrumentChoices != nil) {
			t.Errorf("%s instrument choice = %+v: only the Bard picks an instrument", key, mc.InstrumentChoices)
		}
	}
	bard := c.c.classes["class:bard"].Multiclass.InstrumentChoices
	if bard.Choose != 1 || len(bard.From) != 10 {
		t.Errorf("bard instruments = %+v, want one of the SRD's ten", bard)
	}
}

// richFighter5 is Doran with every score 14 or more, so that he enters any class.
func richFighter5() Build {
	b := doranFighter5()
	b.BaseScores = map[Ability]int{STR: 15, DEX: 15, CON: 14, INT: 15, WIS: 15, CHA: 15}
	return b
}

// TestMulticlassPicksOfTheNewClass: the skill and the instrument that a class asks
// for as a later class are checked, with their own refusals: missing, repeated, off
// the class's list, and an instrument the Bard does not offer.
func TestMulticlassPicksOfTheNewClass(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	before := richFighter5()
	satisfied := func(class string) LevelUpChoices {
		sub := ""
		if s := c.c.classes[class].Subclasses; len(s) > 0 {
			sub = s[0]
		}
		return satisfy(t, c, before, class, sub)
	}
	check := func(ch LevelUpChoices) error {
		after, err := ApplyLevelUp(before, ch, c)
		if err != nil {
			t.Fatalf("ApplyLevelUp() error = %v", err)
		}
		return CheckLevelUp(before, after, c)
	}

	// The Rogue: one skill from its list. Doran has Athletics and Perception from
	// Fighter and Insight and Religion from the Acolyte.
	good := satisfied("class:rogue")
	if err := check(good); err != nil {
		t.Fatalf("the Rogue's own picks: %v", err)
	}
	if len(good.SkillProficiencies) != 1 || !slices.Contains(multiclassSkillPicks["class:rogue"].from, good.SkillProficiencies[0]) {
		t.Errorf("the helper picked %v", good.SkillProficiencies)
	}
	for name, tt := range map[string]struct {
		skills []string
		reason string
	}{
		"none":                    {nil, LevelUpReasonProficiencyChoice},
		"two":                     {[]string{"skill:stealth", "skill:acrobatics"}, LevelUpReasonProficiencyChoice},
		"one the fighter gave":    {[]string{"skill:athletics"}, LevelUpReasonProficiencyChoice},
		"one the background gave": {[]string{"skill:insight"}, LevelUpReasonProficiencyChoice},
		"off the Rogue's list":    {[]string{"skill:arcana"}, LevelUpReasonProficiencyChoice},
		"an unknown skill":        {[]string{"skill:juggling"}, LevelUpReasonProficiencyChoice},
	} {
		ch := good
		ch.SkillProficiencies = tt.skills
		wantRefusal(t, check(ch), tt.reason, "full.skill_proficiency_keys")
		_ = name
	}

	// The Ranger: one skill from the list of the Ranger, not the Rogue's.
	ranger := satisfied("class:ranger")
	if err := check(ranger); err != nil {
		t.Fatalf("the Ranger's own picks: %v", err)
	}
	ranger.SkillProficiencies = []string{"skill:acrobatics"}
	wantRefusal(t, check(ranger), LevelUpReasonProficiencyChoice, "full.skill_proficiency_keys")

	// The Bard: a skill from any list and an instrument.
	bard := satisfied("class:bard")
	if err := check(bard); err != nil {
		t.Fatalf("the Bard's own picks: %v", err)
	}
	if bard.Instrument == "" || len(bard.SkillProficiencies) != 1 {
		t.Fatalf("the helper picked %q and %v", bard.Instrument, bard.SkillProficiencies)
	}
	after := mustApply(t, c, before, bard)
	if len(after.ToolProficiencies) != 1 || after.ToolProficiencies[0] != c.c.proficiencyNamePT(bard.Instrument) {
		t.Errorf("tool proficiencies = %v, want the instrument's name", after.ToolProficiencies)
	}
	anyList := bard
	anyList.SkillProficiencies = []string{"skill:arcana"}
	if err := check(anyList); err != nil {
		t.Errorf("the Bard's skill is from any list: %v", err)
	}
	for name, mod := range map[string]func(ch *LevelUpChoices){
		"no instrument":      func(ch *LevelUpChoices) { ch.Instrument = "" },
		"an unknown one":     func(ch *LevelUpChoices) { ch.Instrument = "proficiency:kazoo" },
		"a tool, not a lyre": func(ch *LevelUpChoices) { ch.Instrument = "proficiency:thieves-tools" },
	} {
		ch := bard
		mod(&ch)
		after, err := ApplyLevelUp(before, ch, c)
		if err != nil {
			t.Fatalf("%s: ApplyLevelUp() error = %v", name, err)
		}
		wantRefusal(t, CheckLevelUp(before, after, c), LevelUpReasonInstrumentChoice, "full.tool_proficiencies")
	}
	// An instrument the character already has is not a new one.
	has := before.clone()
	has.ToolProficiencies = []string{c.c.proficiencyNamePT(bard.Instrument)}
	after2, err := ApplyLevelUp(has, bard, c)
	if err != nil {
		t.Fatal(err)
	}
	wantRefusal(t, CheckLevelUp(has, after2, c), LevelUpReasonInstrumentChoice, "full.tool_proficiencies")
	if o, _ := LevelUpOptions(has, "class:bard", c); !o.Multiclass.Instruments.From[slices.IndexFunc(o.Multiclass.Instruments.From, func(i MulticlassOption) bool { return i.Key == bard.Instrument })].AlreadyHave {
		t.Error("the offer does not mark the instrument the character already has")
	}
	// A class that asks for no instrument refuses one.
	fighterLike := satisfied("class:barbarian")
	fighterLike.Instrument = bard.Instrument
	wantRefusal(t, check(fighterLike), LevelUpReasonLocked, "full.tool_proficiencies")
	// And no level of a class the character has changes the list.
	own := satisfy(t, c, before, "class:fighter", "")
	ownAfter := mustApply(t, c, before, own)
	ownAfter.ToolProficiencies = []string{"Alaúde"}
	wantRefusal(t, CheckLevelUp(before, ownAfter, c), LevelUpReasonLocked, "full.tool_proficiencies")

	// The offer says what the class gives, marking what the character has.
	o, err := LevelUpOptions(before, "class:rogue", c)
	if err != nil {
		t.Fatal(err)
	}
	var tools []string
	for _, p := range o.Multiclass.Proficiencies {
		tools = append(tools, p.Key)
		if p.Key == "proficiency:light-armor" && !p.AlreadyHave {
			t.Error("Doran has light armor from the Fighter")
		}
		if p.Key == "proficiency:thieves-tools" && p.AlreadyHave {
			t.Error("Doran has no thieves' tools")
		}
	}
	if !slices.Equal(tools, multiclassProficiencies["class:rogue"]) {
		t.Errorf("the Rogue's proficiencies = %v", tools)
	}
	for _, opt := range o.Multiclass.Skills.From {
		if want := slices.Contains([]string{"skill:athletics", "skill:perception", "skill:insight"}, opt.Key); opt.AlreadyHave != want {
			t.Errorf("%s already have = %v, want %v", opt.Key, opt.AlreadyHave, want)
		}
	}

	// The summary lists the proficiencies the level gives, apart from what was had.
	rogueAfter := mustApply(t, c, before, satisfied("class:rogue"))
	sum, _ := MulticlassSummaryOf(before, rogueAfter, "class:rogue", c)
	var gained []string
	for _, g := range sum.ProficienciesGained {
		gained = append(gained, g.Key)
	}
	if len(gained) != 2 || gained[0] != "proficiency:thieves-tools" || !slices.Contains(multiclassSkillPicks["class:rogue"].from, gained[1]) {
		t.Errorf("proficiencies gained = %v, want the thieves' tools and the skill picked", gained)
	}
}

// TestMulticlassHitPoints: the new class's die gives its average as a level after
// the first, never the maximum, with the Constitution modifier and at least 1
// (SRD 5.1, "Multiclassing", "Hit Points and Hit Dice"); a rolled sheet takes the
// roll in the order of the classes.
func TestMulticlassHitPoints(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	for name, tt := range map[string]struct {
		con     int
		class   string
		average int // the hit points the level adds
	}{
		"d6 with +2":               {14, "class:wizard", 6},    // 4 + 2, not 6 + 2
		"d12 with +2":              {14, "class:barbarian", 9}, // 7 + 2, not 12 + 2
		"d8 with +0":               {10, "class:rogue", 5},
		"d6 with -1 is at least 1": {8, "class:wizard", 3},
		"d6 with -4 is at least 1": {3, "class:wizard", 1},
	} {
		before := richFighter5()
		before.BaseScores[CON] = tt.con - 1
		hp0 := Derive(before, c).HitPointsMax
		sub := ""
		if s := c.c.classes[tt.class].Subclasses; len(s) > 0 {
			sub = s[0]
		}
		ch := satisfy(t, c, before, tt.class, sub)
		after := mustApply(t, c, before, ch)
		if err := CheckLevelUp(before, after, c); err != nil {
			t.Fatalf("%s: CheckLevelUp: %v", name, err)
		}
		if got := Derive(after, c).HitPointsMax - hp0; got != tt.average {
			t.Errorf("%s: the level adds %d hit points, want %d", name, got, tt.average)
		}
	}

	// A rolled sheet: the roll goes in the class order, and is 1 to the new die.
	before := richFighter5()
	before.HitPoints = HitPoints{Method: HitPointsRolled, Rolls: []int{6, 7, 8, 9}}
	ch := wizardOneChoices()
	ch.HitPoints = LevelUpHitPoints{Roll: 5}
	after := mustApply(t, c, before, ch)
	if err := CheckLevelUp(before, after, c); err != nil {
		t.Fatalf("a roll of 5 on the d6: %v", err)
	}
	if after.HitPoints.Method != HitPointsRolled || !slices.Equal(after.HitPoints.Rolls, []int{6, 7, 8, 9, 5}) {
		t.Errorf("hit points = %+v, want the new roll last", after.HitPoints)
	}
	if got, want := Derive(after, c).HitPointsMax, Derive(before, c).HitPointsMax+5+2; got != want {
		t.Errorf("hit points = %d, want %d", got, want)
	}
	for _, roll := range []int{0, 7, 12} {
		ch.HitPoints = LevelUpHitPoints{Roll: roll}
		wantRefusal(t, CheckLevelUp(before, mustApply(t, c, before, ch), c), LevelUpReasonHitPoints, "full.hit_points.rolls[4]")
	}
	// A sheet at the average that rolls now takes the averages for the earlier levels.
	plain := richFighter5()
	ch.HitPoints = LevelUpHitPoints{Roll: 3}
	rolled := mustApply(t, c, plain, ch)
	if err := CheckLevelUp(plain, rolled, c); err != nil {
		t.Fatalf("a fixed sheet that rolls: %v", err)
	}
	if want := []int{6, 6, 6, 6, 3}; !slices.Equal(rolled.HitPoints.Rolls, want) {
		t.Errorf("rolls = %v, want %v", rolled.HitPoints.Rolls, want)
	}
	// The level's roll is not inserted before the first class's: with the Wizard
	// first and the Fighter second, the Fighter's level goes after.
	wf := Build{
		BaseScores: map[Ability]int{STR: 14, DEX: 14, CON: 13, INT: 14, WIS: 10, CHA: 10},
		Race:       "race:human", Background: "background:acolyte",
		Classes:            []ClassLevel{{Class: "class:wizard", Level: 2}},
		SkillProficiencies: []string{"skill:arcana", "skill:history"},
		Cantrips:           []string{"spell:fire-bolt", "spell:mage-hand", "spell:light"},
		SpellsKnown:        []string{"spell:magic-missile", "spell:sleep", "spell:shield", "spell:mage-armor", "spell:detect-magic", "spell:identify", "spell:find-familiar", "spell:burning-hands"},
		SpellsPrepared:     []string{"spell:magic-missile"},
		HitPoints:          HitPoints{Method: HitPointsRolled, Rolls: []int{2}},
	}
	fighter := satisfy(t, c, wf, "class:fighter", "")
	fighter.HitPoints = LevelUpHitPoints{Roll: 9}
	fa := mustApply(t, c, wf, fighter)
	if err := CheckLevelUp(wf, fa, c); err != nil {
		t.Fatalf("Wizard 2 to the Fighter: %v", err)
	}
	if want := []int{2, 9}; !slices.Equal(fa.HitPoints.Rolls, want) {
		t.Errorf("rolls = %v, want %v", fa.HitPoints.Rolls, want)
	}
}

// TestMulticlassTableClass: a class of the table's content as a new class asks for
// its own minimums ("e") or any_of ("ou"), gives its own multiclass proficiencies
// and skill, and is judged on the final scores like the SRD's.
func TestMulticlassTableClass(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)
	both := genClass(srd, genKind{name: "ambos"})
	both.NamePT = "Ambos"
	both.Minimums, both.AnyOf = map[Ability]int{STR: 14, DEX: 12}, nil
	both.MulticlassProficiencies = []string{"proficiency:heavy-armor", "proficiency:martial-weapons"}
	both.MulticlassSkillChoose = 1
	either := genClass(srd, genKind{name: "ou"})
	either.NamePT = "Um ou outro"
	either.Minimums, either.AnyOf = nil, map[Ability]int{STR: 15, INT: 15}
	c := withOverlayOn(t, srd, Overlay{Revision: 3, Classes: []TableClass{both, either}, Subclasses: slices.Concat(genSubclasses(both), genSubclasses(either))})

	// A Wizard 1 with Strength 14, Dexterity 12 (final) meets "Ambos" and nothing else.
	b := wizardWith(map[Ability]int{STR: 14, DEX: 12, CON: 10, INT: 13, WIS: 10, CHA: 10})
	o, err := LevelUpOptions(b, "class:gen-ambos@mesa", c)
	if err != nil {
		t.Fatalf("Ambos with Strength 14 and Dexterity 12: %v", err)
	}
	if mc := o.Multiclass; mc == nil || len(mc.Proficiencies) != 2 || mc.Skills == nil || mc.Skills.Choose != 1 || len(mc.Skills.From) != 6 {
		t.Errorf("the class's multiclass offer = %+v, want its two proficiencies and one skill of six", mc)
	}
	if o.HitDie != 8 || o.HitPointAverage != 5 {
		t.Errorf("hit die d%d, average %d; want the table class's d8 and 5", o.HitDie, o.HitPointAverage)
	}
	for name, scores := range map[string]map[Ability]int{
		"Strength 13":  {STR: 13, DEX: 12, CON: 10, INT: 13, WIS: 10, CHA: 10},
		"Dexterity 11": {STR: 14, DEX: 11, CON: 10, INT: 13, WIS: 10, CHA: 10},
	} {
		_, err := LevelUpOptions(wizardWith(scores), "class:gen-ambos@mesa", c)
		wantRefusal(t, err, LevelUpReasonMulticlassPrerequisite, "class_key")
		if le := asRefusal(err); le.ClassKey != "class:gen-ambos@mesa" {
			t.Errorf("%s: refusal = %+v", name, le)
		}
	}
	// "Um ou outro": Strength 15 or Intelligence 15; neither is refused.
	if _, err := LevelUpOptions(wizardWith(map[Ability]int{STR: 15, DEX: 10, CON: 10, INT: 13, WIS: 10, CHA: 10}), "class:gen-ou@mesa", c); err != nil {
		t.Errorf("Strength 15: %v", err)
	}
	_, err = LevelUpOptions(wizardWith(map[Ability]int{STR: 14, DEX: 10, CON: 10, INT: 14, WIS: 10, CHA: 10}), "class:gen-ou@mesa", c)
	wantRefusal(t, err, LevelUpReasonMulticlassPrerequisite, "class_key")

	// The class step lists it with the rest.
	var found bool
	for _, ch := range LevelUpClasses(b, c) {
		if ch.Class == "class:gen-ambos@mesa" {
			found = true
			if !ch.Available || len(ch.Prerequisite.Requirements) != 2 || ch.Prerequisite.AnyOf || ch.ClassNamePT != "Ambos" {
				t.Errorf("the table class in the step = %+v", ch)
			}
		}
		if ch.Class == "class:gen-ou@mesa" && (ch.Available || !ch.Prerequisite.AnyOf || ch.Unavailable != UnavailablePrerequisite) {
			t.Errorf("the table class with an \"or\" in the step = %+v", ch)
		}
	}
	if !found {
		t.Error("the table class is not in the class step")
	}
	// It enters the sheet and levels to 20 without an issue.
	sweepMulticlassPair(t, c, "class:wizard", "class:gen-ambos@mesa", true)
	// The table's proficiencies are the multiclass ones, as a later class: the full
	// list is the starting class's.
	d := Derive(mustApply(t, c, b, satisfy(t, c, b, "class:gen-ambos@mesa", "")), c)
	if !slices.Contains(proficiencyKeys(d), "proficiency:heavy-armor") || !slices.Contains(proficiencyKeys(d), "proficiency:martial-weapons") {
		t.Errorf("proficiencies = %v, want the table class's multiclass list", proficiencyKeys(d))
	}
	if slices.Contains(proficiencyKeys(d), "proficiency:light-armor") {
		t.Errorf("proficiencies = %v: the starting class's light armor is not a later class's", proficiencyKeys(d))
	}
}

// TestMulticlassOffersOfEveryClass: what each class asks of the player when it is
// taken as the second class (SRD 5.1 class tables, level 1): the subclass of the
// Cleric, the Sorcerer and the Warlock, the Fighting Style, the Rogue's Expertise,
// the spells of the casters.
func TestMulticlassOffersOfEveryClass(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	before := richFighter5()
	type want struct {
		subclass            bool
		cantrips, spells    int
		spellsKind          string
		prepares            bool
		expertise, hitDie   int
		slots               [9]int
		pactSlots, pactLeve int
	}
	for class, w := range map[string]want{
		"class:barbarian": {hitDie: 12},
		"class:bard":      {cantrips: 2, spells: 4, spellsKind: PreparationKnown, hitDie: 8, slots: [9]int{2}},
		"class:cleric":    {subclass: true, cantrips: 3, prepares: true, hitDie: 8, slots: [9]int{2}},
		"class:druid":     {cantrips: 2, prepares: true, hitDie: 8, slots: [9]int{2}},
		"class:monk":      {hitDie: 8},
		"class:paladin":   {hitDie: 10},
		"class:ranger":    {hitDie: 10},
		"class:rogue":     {expertise: 2, hitDie: 8},
		"class:sorcerer":  {subclass: true, cantrips: 4, spells: 2, spellsKind: PreparationKnown, hitDie: 6, slots: [9]int{2}},
		"class:warlock":   {subclass: true, cantrips: 2, spells: 2, spellsKind: PreparationKnown, hitDie: 8, pactSlots: 1, pactLeve: 1},
		"class:wizard":    {cantrips: 3, spells: 6, spellsKind: PreparationSpellbook, prepares: true, hitDie: 6, slots: [9]int{2}},
	} {
		o, err := LevelUpOptions(before, class, c)
		if err != nil {
			t.Errorf("%s: %v", class, err)
			continue
		}
		pact := 0
		if o.PactAfter != nil {
			pact = o.PactAfter.Slots*10 + o.PactAfter.SlotLevel
		}
		if o.SubclassDue != w.subclass || o.Cantrips != w.cantrips || o.Spells != w.spells || o.SpellsKind != w.spellsKind || o.Prepares != w.prepares ||
			o.ExpertiseChoices != w.expertise || o.HitDie != w.hitDie || o.HitPointAverage != w.hitDie/2+1 || o.SlotsAfter != w.slots || pact != w.pactSlots*10+w.pactLeve ||
			o.AbilityScoreImprovement || !o.NewClass || o.FromLevel != 0 || o.TotalTo != 6 {
			t.Errorf("%s: offer = subclass %v, %d cantrips, %d %q spells, prepares %v, expertise %d, d%d, slots %v, pact %d, ASI %v; want %+v",
				class, o.SubclassDue, o.Cantrips, o.Spells, o.SpellsKind, o.Prepares, o.ExpertiseChoices, o.HitDie, o.SlotsAfter, pact, o.AbilityScoreImprovement, w)
		}
		if w.subclass && len(o.Subclasses) == 0 {
			t.Errorf("%s: no subclass to pick", class)
		}
		if (class == "class:paladin" || class == "class:ranger") && o.SpellList != "" {
			t.Errorf("%s casts at level 2, not at level 1: list %q", class, o.SpellList)
		}
	}
	// The Fighter as a second class: its fighting style, unless the character
	// already has the style (the Paladin's Defense is the Fighter's).
	paladin := richFighter5()
	paladin.Classes = []ClassLevel{{Class: "class:paladin", Subclass: "subclass:devotion", Level: 2}}
	paladin.FeatureChoices = []string{"feature:paladin-fighting-style-defense"}
	paladin.SpellsPrepared = nil
	o, err := LevelUpOptions(paladin, "class:fighter", c)
	if err != nil || len(o.FeatureChoices) != 1 {
		t.Fatalf("Paladin 2 to the Fighter: %v, %v", o.FeatureChoices, err)
	}
	for _, opt := range o.FeatureChoices[0].Options {
		if opt.NamePT == "Defesa" {
			t.Errorf("the Defense style is offered to a character who has it: %v", o.FeatureChoices[0].Options)
		}
	}
}
