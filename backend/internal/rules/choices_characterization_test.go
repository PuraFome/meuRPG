package rules

import (
	"encoding/json"
	"maps"
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/rules/formula"
	"github.com/PuraFome/meuRPG/backend/internal/rules/srd51"
)

// The characterization tests of the choice checks split in E5: what Derive
// writes from the choices (the issues of checkChoices and checkOptionCounts, the
// open choices, the hints of effectHints, the breath weapon and resistances of
// choiceEffects) for a corpus of sheets, and what SpellSources answers. The
// goldens freeze the exact sentences and their order.

// choicesDeriveDump is what the choice steps of Derive leave on a sheet.
func choicesDeriveDump(d Derived) (string, error) {
	out := struct {
		Issues       []Issue
		OpenChoices  []OpenChoice
		Hints        []Hint
		Resistances  []ChoiceResistance
		BreathWeapon *BreathWeapon
	}{d.Issues, d.OpenChoices, d.Hints, d.Resistances, d.BreathWeapon}
	raw, err := json.Marshal(out)
	return string(raw), err
}

// choicesFighter is a human fighter 1 (acolyte: Insight and Religion come with
// the background) with the two class skills chosen.
func choicesFighter() Build {
	return Build{
		BaseScores: map[Ability]int{STR: 15, DEX: 13, CON: 14, INT: 13, WIS: 10, CHA: 13},
		Race:       "race:human", Background: "background:acolyte",
		Classes:            []ClassLevel{{Class: "class:fighter", Level: 1}},
		SkillProficiencies: []string{"skill:athletics", "skill:perception"},
		FeatureChoices:     []string{"feature:fighter-fighting-style-defense"},
	}
}

// choicesDeriveCases are the sheets of the golden: each branch of the skill,
// expertise, option, custom background and multiclass checks.
func choicesDeriveCases(t *testing.T) map[string]func() (Build, *Content) {
	t.Helper()
	srd := loadForTest(t)
	with := func(edit func(b *Build)) func() (Build, *Content) {
		return func() (Build, *Content) {
			b := choicesFighter()
			edit(&b)
			return b, srd
		}
	}
	two := func(b *Build, c1, c2 string) {
		b.Classes = []ClassLevel{{Class: c1, Level: 1}, {Class: c2, Level: 1}}
	}
	return map[string]func() (Build, *Content){
		"skills: as many as the class gives": with(func(*Build) {}),
		"skills: one too many":               with(func(b *Build) { b.SkillProficiencies = append(b.SkillProficiencies, "skill:intimidation") }),
		"skills: one missing":                with(func(b *Build) { b.SkillProficiencies = b.SkillProficiencies[:1] }),
		"skills: two missing":                with(func(b *Build) { b.SkillProficiencies = nil }),
		"skills: an unknown key": with(func(b *Build) {
			b.SkillProficiencies = []string{"skill:nope", "skill:athletics", "skill:perception"}
		}),
		"skills: a background skill does not count": with(func(b *Build) {
			b.SkillProficiencies = append(b.SkillProficiencies, "skill:insight")
		}),
		"skills: no class": with(func(b *Build) { b.Classes = nil }),
		"skills: the half-elf's versatility, two missing": with(func(b *Build) {
			b.Race = "race:half-elf"
		}),
		"skills: the half-elf's versatility, one too many": with(func(b *Build) {
			b.Race = "race:half-elf"
			b.SkillProficiencies = append(b.SkillProficiencies, "skill:history", "skill:stealth", "skill:survival")
		}),
		"skills: the dwarf's tool choice is no skill": with(func(b *Build) {
			b.Race, b.Subrace = "race:dwarf", "subrace:hill-dwarf"
		}),
		"skills: a multiclass rogue adds one": with(func(b *Build) {
			two(b, "class:fighter", "class:rogue")
			b.SkillProficiencies = append(b.SkillProficiencies, "skill:stealth")
		}),
		"skills: a multiclass wizard adds none": with(func(b *Build) {
			two(b, "class:fighter", "class:wizard")
			b.SkillProficiencies = append(b.SkillProficiencies, "skill:arcana")
		}),
		"skills: the lore bard's bonus proficiencies": with(func(b *Build) {
			b.Classes = []ClassLevel{{Class: "class:bard", Subclass: "subclass:lore", Level: 3}}
			b.FeatureChoices = nil
			b.SkillProficiencies = []string{"skill:athletics", "skill:perception", "skill:stealth", "skill:arcana"}
		}),
		"expertise: the rogue's two": with(func(b *Build) {
			b.Classes = []ClassLevel{{Class: "class:rogue", Level: 1}}
			b.FeatureChoices = nil
			b.SkillProficiencies = []string{"skill:athletics", "skill:perception", "skill:stealth", "skill:acrobatics"}
			b.Expertise = []string{"skill:stealth", "skill:perception"}
		}),
		"expertise: the rogue's three": with(func(b *Build) {
			b.Classes = []ClassLevel{{Class: "class:rogue", Level: 1}}
			b.FeatureChoices = nil
			b.SkillProficiencies = []string{"skill:athletics", "skill:perception", "skill:stealth", "skill:acrobatics"}
			b.Expertise = []string{"skill:stealth", "skill:perception", "skill:athletics"}
		}),
		"expertise: a fighter has none": with(func(b *Build) { b.Expertise = []string{"skill:athletics"} }),
		"custom background: a name only": with(func(b *Build) {
			b.Background, b.CustomBackgroundName = "", "Marinheiro"
		}),
		"custom background: two skills, not started": with(func(b *Build) {
			b.Background, b.CustomBackgroundName = "", "Marinheiro"
			b.CustomBackgroundSkills = []string{"skill:insight", "skill:religion"}
		}),
		"custom background: started with one tool": with(func(b *Build) {
			b.Background = ""
			b.CustomBackgroundSkills = []string{"skill:insight"}
			b.CustomBackgroundProficiencies = []string{"proficiency:thieves-tools"}
		}),
		"custom background: a feature name without its text": with(func(b *Build) {
			b.Background = ""
			b.CustomBackgroundSkills = []string{"skill:insight", "skill:religion"}
			b.CustomBackgroundProficiencies = []string{"proficiency:thieves-tools", "language:elvish"}
			b.CustomBackgroundFeatureName = "Porto"
		}),
		"custom background: a feature text without its name": with(func(b *Build) {
			b.Background = ""
			b.CustomBackgroundSkills = []string{"skill:insight", "skill:religion"}
			b.CustomBackgroundProficiencies = []string{"proficiency:thieves-tools", "language:elvish"}
			b.CustomBackgroundFeature = "Conhece o porto."
			b.CustomBackgroundEquipment = "Corda."
		}),
		"custom background: complete": with(func(b *Build) {
			b.Background = ""
			b.CustomBackgroundSkills = []string{"skill:insight", "skill:religion"}
			b.CustomBackgroundProficiencies = []string{"proficiency:thieves-tools", "language:elvish"}
			b.CustomBackgroundFeatureName, b.CustomBackgroundFeature = "Porto", "Conhece o porto."
			b.CustomBackgroundEquipment = "Corda."
		}),
		"custom background: a language the race gives": with(func(b *Build) {
			b.Background = ""
			b.CustomBackgroundSkills = []string{"skill:insight", "skill:religion"}
			b.CustomBackgroundProficiencies = []string{"language:common", "language:elvish"}
			b.CustomBackgroundFeatureName, b.CustomBackgroundFeature = "Porto", "Conhece o porto."
			b.CustomBackgroundEquipment = "Corda."
		}),
		"custom background: no race": with(func(b *Build) {
			b.Race = ""
			b.Background = ""
			b.CustomBackgroundSkills = []string{"skill:insight", "skill:religion"}
			b.CustomBackgroundProficiencies = []string{"language:common"}
		}),
		"multiclass: every prerequisite met": with(func(b *Build) { two(b, "class:fighter", "class:paladin") }),
		"multiclass: the wizard's Intelligence": with(func(b *Build) {
			two(b, "class:fighter", "class:wizard")
			b.BaseScores[INT] = 8
		}),
		"multiclass: the fighter's any of, and the paladin's two": with(func(b *Build) {
			two(b, "class:paladin", "class:fighter")
			b.BaseScores[STR], b.BaseScores[DEX] = 10, 10
		}),
		"options: the champion's second style repeats the first by name": with(func(b *Build) {
			b.Classes = []ClassLevel{{Class: "class:fighter", Subclass: "subclass:champion", Level: 10}}
			b.FeatureChoices = []string{"feature:fighter-fighting-style-defense", "feature:fighter-fighting-style-archery", "feature:fighter-fighting-style-dueling"}
		}),
		"options: a fighter and a paladin take Defense twice": with(func(b *Build) {
			b.Classes = []ClassLevel{{Class: "class:fighter", Level: 1}, {Class: "class:paladin", Level: 2}}
			b.FeatureChoices = []string{"feature:fighter-fighting-style-defense", "feature:fighting-style-defense"}
		}),
		"options: too many invocations": with(func(b *Build) {
			b.Classes = []ClassLevel{{Class: "class:warlock", Subclass: "subclass:fiend", Level: 2}}
			b.FeatureChoices = []string{"feature:eldritch-invocation-agonizing-blast", "feature:eldritch-invocation-armor-of-shadows", "feature:eldritch-invocation-devils-sight"}
		}),
		"options: metamagic over the count": with(func(b *Build) {
			b.Classes = []ClassLevel{{Class: "class:sorcerer", Subclass: "subclass:draconic", Level: 3}}
			b.FeatureChoices = []string{"feature:metamagic-careful-spell", "feature:metamagic-distant-spell", "feature:metamagic-empowered-spell"}
		}),
		"effects: the dragonborn's breath and resistance": with(func(b *Build) {
			b.Race = "race:dragonborn"
			b.FeatureChoices = append(b.FeatureChoices, "trait:draconic-ancestry-red")
		}),
		"effects: a dragonborn level 11": with(func(b *Build) {
			b.Race = "race:dragonborn"
			b.Classes[0].Level = 11
			b.FeatureChoices = append(b.FeatureChoices, "trait:draconic-ancestry-black")
		}),
		"effects: an ancestry the sheet does not have": with(func(b *Build) {
			b.FeatureChoices = append(b.FeatureChoices, "trait:draconic-ancestry-red")
		}),
		"effects: a draconic sorcerer's affinity": with(func(b *Build) {
			b.Classes = []ClassLevel{{Class: "class:sorcerer", Subclass: "subclass:draconic", Level: 6}}
			b.FeatureChoices = []string{"feature:dragon-ancestor-gold---fire-damage", "feature:metamagic-careful-spell", "feature:metamagic-distant-spell"}
		}),
		"effects: a dragonborn draconic sorcerer of the same color": with(func(b *Build) {
			b.Race = "race:dragonborn"
			b.Classes = []ClassLevel{{Class: "class:sorcerer", Subclass: "subclass:draconic", Level: 6}}
			b.FeatureChoices = []string{"trait:draconic-ancestry-gold", "feature:dragon-ancestor-gold---fire-damage"}
		}),
		"effects: the tiefling's resistance": with(func(b *Build) { b.Race = "race:tiefling" }),
		"hints: a dwarf's stonecunning":      with(func(b *Build) { b.Race, b.Subrace = "race:dwarf", "subrace:hill-dwarf" }),
		"hints: a barbarian's rage": with(func(b *Build) {
			b.Classes = []ClassLevel{{Class: "class:barbarian", Level: 9}}
			b.FeatureChoices = nil
		}),
	}
}

// TestChoicesDeriveCharacterization freezes what Derive writes from the
// choices of each sheet of the corpus.
func TestChoicesDeriveCharacterization(t *testing.T) {
	t.Parallel()
	got := map[string]string{}
	for name, mk := range choicesDeriveCases(t) {
		b, c := mk()
		text, err := choicesDeriveDump(Derive(b, c))
		if err != nil {
			t.Fatal(err)
		}
		got[name] = text
	}
	goldenText(t, "choices-derive", got)
}

// choicesInjected runs Derive's steps as deriveWith does, with extra effects
// active from the start of the hints, and dumps what the choice steps leave:
// effects the content cannot carry (an unknown skill, a failing formula, a
// condition that does not hold, a choice of expertise) reach every branch.
func choicesInjected(b Build, c *content, extra func(x *deriver) []activeEffect) (string, error) {
	d := &Derived{ContentVersion: c.version}
	x := &deriver{b: b, c: c, d: d, proficient: map[string]bool{}, conditions: map[*Effect]bool{}, appliedTagged: map[*Effect]bool{}}
	x.resolve()
	x.resolveArmor()
	x.names()
	x.abilities()
	x.levelAndProficiency()
	x.collectEffects()
	x.buildEnv()
	x.abilityEffects()
	x.proficiencies()
	x.savingThrows()
	x.skills()
	x.armorClass()
	x.hitPoints()
	x.speedAndSenses()
	x.languages()
	x.spellcasting()
	x.rangedAttacks()
	x.attacks()
	x.resourcesAndActions()
	d.Issues, d.Hints = nil, nil
	x.active = append(x.active, extra(x)...)
	x.effectHints()
	x.choiceEffects()
	x.checkChoices()
	return choicesDeriveDump(*d)
}

// choicesEffect is an effect with its formulas compiled, failing the test on a
// formula that does not compile.
func choicesEffect(t *testing.T, c *content, e Effect, value, when string) *Effect {
	t.Helper()
	var err error
	if value != "" {
		e.Value = value
		if e.value, err = c.compileFormula(value, formula.Int); err != nil {
			t.Fatal(err)
		}
	}
	if when != "" {
		if e.when, err = c.compileFormula(when, formula.Bool); err != nil {
			t.Fatal(err)
		}
	}
	return &e
}

// TestChoicesInjectedEffectsCharacterization freezes effectHints and the
// effect-driven counts of checkChoices on effects the content cannot carry.
func TestChoicesInjectedEffectsCharacterization(t *testing.T) {
	t.Parallel()
	c := loadForTest(t).c
	const owner = "feature:second-wind"
	tags := []string{"about:test"}
	eff := func(e Effect, value, when string) *Effect { return choicesEffect(t, c, e, value, when) }
	failing := `level() % (classLevel("monk"))`
	cases := map[string][]*Effect{
		"roll modes": {
			eff(Effect{Type: "roll_mode", Roll: "advantage", Targets: []string{"skill.stealth", "save.int"}, TextPT: "Vantagem."}, "", ""),
			eff(Effect{Type: "roll_mode", Roll: "disadvantage"}, "", ""),
		},
		"a condition that does not hold": {
			eff(Effect{Type: "roll_mode", Roll: "advantage", Targets: []string{"attack"}}, "", "level() > 99"),
		},
		"a condition that fails": {
			eff(Effect{Type: "roll_mode", Roll: "advantage", Targets: []string{"attack"}}, "", `level() % (classLevel("monk")) == 0`),
		},
		"tagged proficiencies": {
			eff(Effect{Type: "proficiency", Proficiency: "skill:arcana", Level: "expertise", Tags: tags, TextPT: "Arcanismo {value} ({n})."}, "", ""),
			eff(Effect{Type: "proficiency", Proficiency: "skill:athletics", Level: "half", Tags: tags}, "", ""),
			eff(Effect{Type: "proficiency", Proficiency: "skill:nope", Tags: tags}, "", ""),
			eff(Effect{Type: "proficiency", Proficiency: "proficiency:thieves-tools", Tags: tags}, "", ""),
			eff(Effect{Type: "proficiency", Proficiency: "skill:history"}, "", ""),
		},
		"tagged modifiers": {
			eff(Effect{Type: "modifier", Target: "skill.stealth", Mode: "add", Tags: tags, TextPT: "Furtividade {value}."}, "2", ""),
			eff(Effect{Type: "modifier", Target: "ac", Mode: "add", Tags: tags}, "0 - 3", ""),
			eff(Effect{Type: "modifier", Target: "ac", Mode: "add", Tags: tags}, failing, ""),
			eff(Effect{Type: "modifier", Target: "ac", Mode: "add"}, "1", ""),
		},
		"notes": {
			eff(Effect{Type: "note", TextPT: "Só texto."}, "", ""),
			eff(Effect{Type: "note", TextPT: "Valor {value} e {n}."}, "0 - 2", ""),
			eff(Effect{Type: "note", TextPT: "Falha {value}."}, failing, ""),
			eff(Effect{Type: "note"}, "", ""),
		},
		"another type": {
			eff(Effect{Type: "speed", TextPT: "Nada."}, "", ""),
		},
		"choices of skills and expertise": {
			eff(Effect{Type: "choice", Choice: "skill", Count: 2}, "", ""),
			eff(Effect{Type: "choice", Choice: "expertise", Count: 1}, "", ""),
			eff(Effect{Type: "choice", Choice: "cantrip", Count: 1}, "", ""),
		},
	}
	got := map[string]string{}
	for name, effects := range cases {
		b := choicesFighter()
		b.Expertise = []string{"skill:athletics", "skill:perception"}
		text, err := choicesInjected(b, c, func(x *deriver) []activeEffect {
			var out []activeEffect
			for _, e := range effects {
				out = append(out, activeEffect{owner: owner, effect: e})
			}
			if name == "tagged modifiers" {
				x.appliedTagged[effects[0]] = true
			}
			return out
		})
		if err != nil {
			t.Fatal(err)
		}
		got[name] = text
	}
	// The same tagged modifier, applied elsewhere first and not.
	b := choicesFighter()
	for _, applied := range []bool{false, true} {
		e := eff(Effect{Type: "modifier", Target: "ac", Mode: "add", Tags: tags, TextPT: "CA {value}."}, "1", "")
		text, err := choicesInjected(b, c, func(x *deriver) []activeEffect {
			x.appliedTagged[e] = applied
			return []activeEffect{{owner: owner, effect: e}}
		})
		if err != nil {
			t.Fatal(err)
		}
		got[map[bool]string{false: "a tagged modifier not applied", true: "a tagged modifier already applied"}[applied]] = text
	}
	// Content without the choices file: no breath weapon, no resistance.
	noChoices := *c
	noChoices.choices = nil
	b.Race = "race:dragonborn"
	b.FeatureChoices = append(b.FeatureChoices, "trait:draconic-ancestry-red")
	text, err := choicesInjected(b, &noChoices, func(*deriver) []activeEffect { return nil })
	if err != nil {
		t.Fatal(err)
	}
	got["no choices file"] = text
	// A trait whose own resistance repeats the ancestry's: listed once.
	repeat := *c
	data := *c.choices
	data.resistances = maps.Clone(c.choices.resistances)
	data.resistances["trait:breath-weapon"] = racialResistance{Trait: "trait:breath-weapon", DamageType: "damage-type:fire"}
	repeat.choices = &data
	if text, err = choicesInjected(b, &repeat, func(*deriver) []activeEffect { return nil }); err != nil {
		t.Fatal(err)
	}
	got["a resistance twice"] = text
	goldenText(t, "choices-injected", got)
}

// optionCountScenario is checkOptionCounts on its own: the features the sheet
// owns (in order, repeats included), the picks and the classes.
type optionCountScenario struct {
	owned   []string
	picks   []string
	classes []ownedClass
}

// optionCountContent is the SRD with made-up features that offer options:
// pools p (o1, o2), q (o3, o4), r (o5), a bridge of p and q, one of p, q and r,
// a feature that offers two options of the same name (d1 and d2, "Defesa") and
// one whose count is zero.
func optionCountContent(t *testing.T) *content {
	t.Helper()
	base := loadForTest(t).c
	c := *base
	c.features = maps.Clone(base.features)
	c.namesPT = maps.Clone(base.namesPT)
	add := func(key, name string, choose int, options ...string) {
		c.features[key] = &srd51.Feature{Key: key, Name: name, OptionsChoose: choose, Options: options}
		c.namesPT[key] = name + " (PT)"
	}
	for _, o := range []string{"o1", "o2", "o3", "o4", "o5"} {
		add("feature:oc-"+o, "Opção "+o, 0)
	}
	add("feature:oc-d1", "Defesa", 0)
	add("feature:oc-d2", "Defesa", 0)
	add("feature:oc-p", "P", 1, "feature:oc-o1", "feature:oc-o2")
	add("feature:oc-q", "Q", 1, "feature:oc-o3", "feature:oc-o4")
	add("feature:oc-r", "R", 2, "feature:oc-o5")
	add("feature:oc-bridge", "Ponte", 1, "feature:oc-o2", "feature:oc-o3")
	add("feature:oc-bridge3", "Ponte tripla", 1, "feature:oc-o1", "feature:oc-o3", "feature:oc-o5")
	add("feature:oc-styles", "Estilos", 1, "feature:oc-d1", "feature:oc-d2")
	add("feature:oc-none", "Nenhum", 0, "feature:oc-o1")
	return &c
}

func optionCountScenarios() map[string]optionCountScenario {
	o := func(n ...string) []string {
		out := make([]string, len(n))
		for i, s := range n {
			out[i] = "feature:oc-" + s
		}
		return out
	}
	warlock := func(level int) []ownedClass { return []ownedClass{{key: "class:warlock", level: level}} }
	inv := []string{"feature:eldritch-invocation-agonizing-blast", "feature:eldritch-invocation-armor-of-shadows", "feature:eldritch-invocation-devils-sight", "feature:eldritch-invocation-mask-of-many-faces"}
	return map[string]optionCountScenario{
		"nothing":                            {},
		"one pool within its count":          {owned: o("p"), picks: o("o1")},
		"one pool over its count":            {owned: o("p"), picks: o("o1", "o2")},
		"a feature owned twice counts once":  {owned: o("p", "p"), picks: o("o1", "o2")},
		"a pick that is no feature":          {owned: o("p"), picks: []string{"feature:nope", "feature:oc-o1"}},
		"a pick of another pool":             {owned: o("p"), picks: o("o1", "o3")},
		"two pools apart":                    {owned: o("p", "q"), picks: o("o1", "o2", "o3", "o4")},
		"a bridge joins two pools":           {owned: o("p", "q", "bridge"), picks: o("o1", "o2", "o3", "o4")},
		"a bridge within the sum":            {owned: o("p", "q", "bridge"), picks: o("o1", "o2", "o3")},
		"a bridge owned first":               {owned: o("bridge", "p", "q"), picks: o("o1", "o2", "o3", "o4")},
		"a bridge of three pools":            {owned: o("p", "q", "r", "bridge3"), picks: o("o1", "o2", "o3", "o4", "o5")},
		"two options of one name count once": {owned: o("styles"), picks: o("d1", "d2")},
		"a count of zero offers nothing":     {owned: o("none"), picks: o("o1")},
		"invocations of a warlock 2": {
			owned: []string{invocationsFeature}, picks: inv[:3], classes: warlock(2),
		},
		"invocations within the count": {
			owned: []string{invocationsFeature}, picks: inv[:2], classes: warlock(2),
		},
		"invocations of two warlock entries": {
			owned: []string{invocationsFeature}, picks: inv, classes: append(warlock(2), warlock(5)...),
		},
		"invocations at level 0": {
			owned: []string{invocationsFeature}, picks: inv[:1], classes: warlock(0),
		},
		"invocations past the table": {
			owned: []string{invocationsFeature}, picks: inv[:1], classes: warlock(MaxLevel + 1),
		},
		"invocations of a class without them": {
			owned: []string{invocationsFeature}, picks: inv[:1], classes: []ownedClass{{key: "class:fighter", level: 5}},
		},
		"invocations not owned": {picks: inv, classes: warlock(2)},
		"invocations joined by a pool of their options": {
			owned: []string{"feature:oc-p", invocationsFeature}, picks: inv[:3], classes: warlock(2),
		},
	}
}

// TestCheckOptionCountsCharacterization freezes the issues checkOptionCounts
// raises: the pools, their bridges, the names counted once, the invocations.
func TestCheckOptionCountsCharacterization(t *testing.T) {
	t.Parallel()
	c := optionCountContent(t)
	got := map[string]string{}
	for name, s := range optionCountScenarios() {
		x := &deriver{c: c, b: Build{FeatureChoices: s.picks}, d: &Derived{}, classes: s.classes}
		for _, k := range s.owned {
			x.d.Features = append(x.d.Features, Feature{Key: k})
		}
		x.checkOptionCounts()
		raw, err := json.Marshal(x.d.Issues)
		if err != nil {
			t.Fatal(err)
		}
		got[name] = string(raw)
	}
	goldenText(t, "choices-optioncounts", got)
}

// TestSpellSourcesCharacterization freezes SpellSources: the patron's expanded
// list up to the class level, the Magical Secrets of the bard and the lore
// college, a table key that only looks like Magical Secrets, and the classes
// that answer nothing.
func TestSpellSourcesCharacterization(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)
	build := func(classes ...ClassLevel) Build {
		return Build{
			BaseScores: map[Ability]int{STR: 10, DEX: 14, CON: 14, INT: 13, WIS: 13, CHA: 16},
			Race:       "race:human", Background: "background:acolyte", Classes: classes,
		}
	}
	// A table copy of the bard whose level 2 gives a feature named like Magical
	// Secrets: it is not one.
	tableBard := func() *Content {
		base := srd.c
		c := *base
		c.classLevels = maps.Clone(base.classLevels)
		rows := append([]*srd51.Level(nil), base.classLevels["class:bard"]...)
		row := *rows[1]
		row.Features = append(append([]string(nil), row.Features...), "feature:magical-secrets-9"+tableSuffix)
		rows[1] = &row
		c.classLevels["class:bard"] = rows
		return &Content{c: &c}
	}()
	cases := map[string]struct {
		c *Content
		b Build
	}{
		"nothing":                    {srd, build()},
		"a fighter":                  {srd, build(ClassLevel{Class: "class:fighter", Level: 5})},
		"a wizard":                   {srd, build(ClassLevel{Class: "class:wizard", Subclass: "subclass:evocation", Level: 5})},
		"a fiend 1":                  {srd, build(ClassLevel{Class: "class:warlock", Subclass: "subclass:fiend", Level: 1})},
		"a fiend 5":                  {srd, build(ClassLevel{Class: "class:warlock", Subclass: "subclass:fiend", Level: 5})},
		"a fiend 20":                 {srd, build(ClassLevel{Class: "class:warlock", Subclass: "subclass:fiend", Level: 20})},
		"a warlock without a patron": {srd, build(ClassLevel{Class: "class:warlock", Level: 1})},
		"a bard 9":                   {srd, build(ClassLevel{Class: "class:bard", Level: 9})},
		"a bard 10":                  {srd, build(ClassLevel{Class: "class:bard", Level: 10})},
		"a bard 18":                  {srd, build(ClassLevel{Class: "class:bard", Level: 18})},
		"a lore bard 6":              {srd, build(ClassLevel{Class: "class:bard", Subclass: "subclass:lore", Level: 6})},
		"a lore bard 14":             {srd, build(ClassLevel{Class: "class:bard", Subclass: "subclass:lore", Level: 14})},
		"a bard 10 and a fiend 3": {srd, build(
			ClassLevel{Class: "class:bard", Level: 10}, ClassLevel{Class: "class:warlock", Subclass: "subclass:fiend", Level: 3})},
		"a table key like Magical Secrets": {tableBard, build(ClassLevel{Class: "class:bard", Level: 2})},
	}
	got := map[string]string{}
	for name, tc := range cases {
		raw, err := json.Marshal(tc.c.SpellSources(tc.b))
		if err != nil {
			t.Fatal(err)
		}
		got[name] = string(raw)
	}
	goldenText(t, "choices-spellsources", got)
}
