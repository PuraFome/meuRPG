package rules

import (
	"fmt"
	"slices"
	"testing"
)

// TestLevelUpSweep levels every class from 1 to 20 with every subclass the SRD
// has, at each level satisfying exactly what LevelUpOptions asks for with the
// first valid picks. Each step must pass CheckLevelUp, Validate and leave the
// sheet with no Issue: the options and the check never disagree about a count,
// and the engine agrees with both.
func TestLevelUpSweep(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	for _, classKey := range sortedKeys(c.c.classes) {
		class := c.c.classes[classKey]
		for _, subKey := range class.Subclasses {
			t.Run(classKey+"/"+subKey, func(t *testing.T) {
				t.Parallel()
				b := sweepBase(t, c, classKey, subKey)
				if issues := Derive(b, c).Issues; len(issues) != 0 {
					t.Fatalf("the level 1 base has issues: %v", issues)
				}
				for level := 2; level <= MaxLevel; level++ {
					at := fmt.Sprintf("%s (%s) %d to %d", classKey, subKey, level-1, level)
					ch := satisfy(t, c, b, classKey, subKey)
					after, err := ApplyLevelUp(b, ch, c)
					if err != nil {
						t.Fatalf("%s: ApplyLevelUp: %v", at, err)
					}
					if err := CheckLevelUp(b, after, c); err != nil {
						t.Fatalf("%s: CheckLevelUp: %v\nchoices: %+v", at, err, ch)
					}
					if err := Validate(after, c); err != nil {
						t.Fatalf("%s: Validate: %v", at, err)
					}
					if issues := Derive(after, c).Issues; len(issues) != 0 {
						t.Fatalf("%s: the new sheet has issues: %v\nchoices: %+v", at, issues, ch)
					}
					b = after
				}
			})
		}
	}
}

// sweepBase is a level 1 character of the class that has everything its first
// level asks for, and the subclass when the class picks it at level 1.
func sweepBase(t *testing.T, c *Content, classKey, subKey string) Build {
	t.Helper()
	class := c.c.classes[classKey]
	b := Build{
		BaseScores: map[Ability]int{STR: 15, DEX: 14, CON: 13, INT: 12, WIS: 10, CHA: 8},
		Race:       "race:human", Background: "background:acolyte",
		Classes: []ClassLevel{{Class: classKey, Level: 1}},
	}
	if class.SubclassLevel == 1 {
		b.Classes[0].Subclass = subKey
	}
	taken := map[string]bool{}
	for _, s := range c.c.backgrounds["background:acolyte"].Skills {
		taken[s] = true
	}
	for _, s := range class.SkillChoices.From {
		if len(b.SkillProficiencies) < class.SkillChoices.Choose && !taken[s] {
			b.SkillProficiencies = append(b.SkillProficiencies, s)
			taken[s] = true
		}
	}
	gains := c.c.classGains(classKey, 1)
	if sub := c.c.subclasses[b.Classes[0].Subclass]; sub != nil {
		gains = gains.plus(c.c.subclassGains(sub, 1))
	}
	for _, d := range gains.choices {
		b.FeatureChoices = append(b.FeatureChoices, firstOptions(c, d.options, d.choose)...)
	}
	for range gains.skills {
		b.SkillProficiencies = append(b.SkillProficiencies, nextSkill(c, b))
	}
	b.Expertise = pickExpertise(c, b, gains.expertise)

	if sc := spellcastingOf(Derive(b, c), classKey); sc != nil {
		b.Cantrips = pickSpells(c, classKey, 0, 0, b.Cantrips, sc.CantripsKnown+gains.cantrips, false)
		switch preparation(c.c.casting[classKey].effect) {
		case PreparationSpellbook:
			b.SpellsKnown = pickSpells(c, classKey, 1, sc.MaxSpellLevel, nil, 6, false)
		case PreparationKnown:
			b.SpellsKnown = pickSpells(c, classKey, 1, sc.MaxSpellLevel, nil, sc.SpellsKnownMax, false)
		}
		b.SpellsPrepared = pickPrepared(c, b, classKey)
	}
	return b
}

// nextSkill is the first skill the character is not proficient in.
func nextSkill(c *Content, b Build) string {
	have := map[string]bool{}
	for _, s := range b.SkillProficiencies {
		have[s] = true
	}
	for _, s := range c.c.backgrounds[b.Background].Skills {
		have[s] = true
	}
	for _, s := range c.c.skillOrder {
		if !have[s] {
			return s
		}
	}
	return ""
}

// pickExpertise is n skills the character is proficient in, without expertise.
func pickExpertise(c *Content, b Build, n int) []string {
	out := slices.Clone(b.Expertise)
	pool := slices.Concat(b.SkillProficiencies, c.c.backgrounds[b.Background].Skills)
	for _, s := range pool {
		if n > 0 && !slices.Contains(out, s) {
			out = append(out, s)
			n--
		}
	}
	return out
}

// pickSpells adds to have the first spells of the class's list (or, with
// anyList, of every other class's list) from minLevel to maxLevel, until it
// has n new ones.
func pickSpells(c *Content, classKey string, minLevel, maxLevel int, have []string, n int, anyList bool) []string {
	out := slices.Clone(have)
	for _, key := range sortedKeys(c.c.spells) {
		s := c.c.spells[key]
		if n <= 0 {
			break
		}
		if s.Level < minLevel || s.Level > maxLevel || slices.Contains(out, key) || slices.Contains(s.Classes, classKey) == anyList {
			continue
		}
		out = append(out, key)
		n--
	}
	return out
}

// pickPrepared prepares up to the class's maximum: from the book for the
// Wizard, from the whole list for the others.
func pickPrepared(c *Content, b Build, classKey string) []string {
	sc := spellcastingOf(Derive(b, c), classKey)
	if sc == nil || !sc.PreparesSpells {
		return b.SpellsPrepared
	}
	out := slices.Clone(b.SpellsPrepared)
	need := sc.PreparedMax - len(out)
	if preparation(c.c.casting[classKey].effect) == PreparationSpellbook {
		for _, k := range b.SpellsKnown {
			if need > 0 && !slices.Contains(out, k) {
				out = append(out, k)
				need--
			}
		}
		return out
	}
	return pickSpells(c, classKey, 1, sc.MaxSpellLevel, out, max(need, 0), false)
}

// satisfy makes the choices that LevelUpOptions asks for, the first valid ones.
func satisfy(t *testing.T, c *Content, b Build, classKey, subKey string) LevelUpChoices {
	t.Helper()
	o, err := LevelUpOptions(b, classKey, c)
	if err != nil {
		t.Fatalf("LevelUpOptions: %v", err)
	}
	ch := LevelUpChoices{Class: classKey, HitPoints: LevelUpHitPoints{Average: true}}
	choices, skills, expertise, cantrips := o.FeatureChoices, o.SkillChoices, o.ExpertiseChoices, o.Cantrips
	if o.SubclassDue {
		for _, s := range o.Subclasses {
			if s.Key == subKey {
				ch.Subclass = s.Key
				choices = slices.Concat(choices, s.FeatureChoices)
				skills, expertise, cantrips = skills+s.SkillChoices, expertise+s.ExpertiseChoices, cantrips+s.Cantrips
			}
		}
		if ch.Subclass == "" {
			t.Fatalf("subclass %s is not among the options", subKey)
		}
	}
	if o.AbilityScoreImprovement {
		d := Derive(b, c)
		scores := slices.Clone(d.Abilities)
		slices.SortStableFunc(scores, func(a, b AbilityScore) int { return a.Score - b.Score })
		ch.AbilityIncrease = map[Ability]int{scores[0].Ability: 1, scores[1].Ability: 1}
	}
	for _, fc := range choices {
		var keys []string
		for _, o := range fc.Options {
			keys = append(keys, o.Key)
		}
		ch.FeatureChoices = append(ch.FeatureChoices, firstOptions(c, keys, fc.Choose)...)
	}
	tmp := b.clone()
	for range skills {
		tmp.SkillProficiencies = append(tmp.SkillProficiencies, nextSkill(c, tmp))
	}
	ch.SkillProficiencies = tmp.SkillProficiencies[len(b.SkillProficiencies):]
	ch.Expertise = pickExpertise(c, b, expertise)[len(b.Expertise):]
	if o.SpellList != "" {
		ch.Cantrips = pickSpells(c, classKey, 0, 0, b.Cantrips, cantrips, false)[len(b.Cantrips):]
		anyList := pickSpells(c, classKey, 1, o.MaxSpellLevel, b.SpellsKnown, o.AnyClassSpells, true)[len(b.SpellsKnown):]
		ownList := pickSpells(c, classKey, 1, o.MaxSpellLevel, slices.Concat(b.SpellsKnown, anyList), o.Spells-o.AnyClassSpells, false)[len(b.SpellsKnown)+len(anyList):]
		ch.Spells = slices.Concat(anyList, ownList)
	}
	// The prepared spells depend on the maximum with the other choices made.
	after, err := ApplyLevelUp(b, ch, c)
	if err != nil {
		t.Fatalf("ApplyLevelUp: %v", err)
	}
	if o.Prepares {
		ch.Prepared = pickPrepared(c, after, classKey)[len(after.SpellsPrepared):]
	}
	return ch
}

// firstOptions are the first n options that give no proficiency of their own
// (an invocation that grants skills would turn a chosen skill into an
// automatic one, which is the sheet editor's business, not the level-up's).
func firstOptions(c *Content, options []string, n int) []string {
	var out []string
	for _, o := range options {
		plain := !slices.ContainsFunc(c.c.effects[o], func(e *Effect) bool { return e.Type == "proficiency" })
		if len(out) < n && plain {
			out = append(out, o)
		}
	}
	return out
}
