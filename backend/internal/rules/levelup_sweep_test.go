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
				sweepUp(t, c, b, classKey, subKey, MaxLevel)
			})
		}
	}
}

// sweepUp levels the class of b one level at a time up to total level to,
// checking every step as TestLevelUpSweep describes, and returns the sheet.
func sweepUp(t *testing.T, c *Content, b Build, classKey, subKey string, to int) Build {
	t.Helper()
	if issues := Derive(b, c).Issues; len(issues) != 0 {
		t.Fatalf("the base has issues: %v", issues)
	}
	for b.totalLevel() < to {
		level := b.totalLevel() + 1
		at := fmt.Sprintf("%s (%s) to total level %d", classKey, subKey, level)
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
	return b
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

	return fillSpells(c, b, classKey, gains.cantrips)
}

// fillSpells gives a class of b that casts the cantrips and spells its level
// asks for, from its list.
func fillSpells(c *Content, b Build, classKey string, extraCantrips int) Build {
	sub := c.c.subclasses[subclassOf(b, classKey)]
	cast, casts := c.c.castingFor(classKey, sub)
	if sc := spellcastingOf(Derive(b, c), classKey); sc != nil && casts {
		b.Cantrips = pickSpells(c, cast.list, 0, 0, b.Cantrips, sc.CantripsKnown+extraCantrips, false)
		switch preparation(cast.effect) {
		case PreparationSpellbook:
			b.SpellsKnown = pickSpells(c, cast.list, 1, sc.MaxSpellLevel, b.SpellsKnown, 6, false)
		case PreparationKnown:
			b.SpellsKnown = pickSpells(c, cast.list, 1, sc.MaxSpellLevel, b.SpellsKnown, sc.SpellsKnownMax, false)
		}
		b.SpellsPrepared = pickPrepared(c, b, classKey)
	}
	return b
}

// subclassOf is the subclass key the character has in a class.
func subclassOf(b Build, classKey string) string {
	for _, cl := range b.Classes {
		if cl.Class == classKey {
			return cl.Subclass
		}
	}
	return ""
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

// pickSpells adds to have the first spells of the list of class classKey (or, with
// anyList, of every other class's list) from minLevel to maxLevel, until it
// has n new ones.
func pickSpells(c *Content, classKey string, minLevel, maxLevel int, have []string, n int, anyList bool) []string {
	out := slices.Clone(have)
	for _, key := range sortedKeys(c.c.spells) {
		s := c.c.spells[key]
		if n <= 0 {
			break
		}
		if s.Level < minLevel || s.Level > maxLevel || slices.Contains(out, key) || c.c.onList(s, classKey) == anyList {
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
	if cast, _ := c.c.castingFor(classKey, c.c.subclasses[subclassOf(b, classKey)]); preparation(cast.effect) == PreparationSpellbook {
		for _, k := range b.SpellsKnown {
			if need > 0 && !slices.Contains(out, k) {
				out = append(out, k)
				need--
			}
		}
		return out
	}
	cast, _ := c.c.castingFor(classKey, c.c.subclasses[subclassOf(b, classKey)])
	return pickSpells(c, cast.list, 1, sc.MaxSpellLevel, out, max(need, 0), false)
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
	spells, spellList, maxSpellLevel := o.Spells, o.SpellList, o.MaxSpellLevel
	if o.SubclassDue {
		for _, s := range o.Subclasses {
			if s.Key == subKey {
				ch.Subclass = s.Key
				choices = slices.Concat(choices, s.FeatureChoices)
				skills, expertise, cantrips = skills+s.SkillChoices, expertise+s.ExpertiseChoices, cantrips+s.Cantrips
				spells += s.Spells
				if s.SpellList != "" {
					spellList, maxSpellLevel = s.SpellList, s.MaxSpellLevel
				}
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
	// A class taken as a later class asks for its own picks first: a skill from its
	// list and a musical instrument.
	if mc := o.Multiclass; mc != nil {
		if mc.Skills != nil {
			for _, opt := range mc.Skills.From {
				if !opt.AlreadyHave && len(tmp.SkillProficiencies)-len(b.SkillProficiencies) < mc.Skills.Choose {
					tmp.SkillProficiencies = append(tmp.SkillProficiencies, opt.Key)
				}
			}
		}
		if mc.Instruments != nil {
			for _, opt := range mc.Instruments.From {
				if !opt.AlreadyHave && ch.Instrument == "" {
					ch.Instrument = opt.Key
				}
			}
		}
	}
	for range skills {
		tmp.SkillProficiencies = append(tmp.SkillProficiencies, nextSkill(c, tmp))
	}
	ch.SkillProficiencies = tmp.SkillProficiencies[len(b.SkillProficiencies):]
	ch.Expertise = pickExpertise(c, b, expertise)[len(b.Expertise):]
	if spellList != "" {
		ch.Cantrips = pickSpells(c, spellList, 0, 0, b.Cantrips, cantrips, false)[len(b.Cantrips):]
		if len(ch.Cantrips) < cantrips {
			// The sheet has one list of cantrips, so a cantrip two classes share is known
			// once: a class whose list the other class has used up has none left to give.
			t.Skipf("%s has no cantrip left that the sheet does not know (%d of %d)", spellList, len(ch.Cantrips), cantrips)
		}
		anyList := pickSpells(c, spellList, 1, maxSpellLevel, b.SpellsKnown, o.AnyClassSpells, true)[len(b.SpellsKnown):]
		ownList := pickSpells(c, spellList, 1, maxSpellLevel, slices.Concat(b.SpellsKnown, anyList), spells-o.AnyClassSpells, false)[len(b.SpellsKnown)+len(anyList):]
		ch.Spells = slices.Concat(anyList, ownList)
	}
	// The prepared spells depend on the maximum with the other choices made.
	after, err := ApplyLevelUp(b, ch, c)
	if err != nil {
		t.Fatalf("ApplyLevelUp: %v", err)
	}
	if o.Prepares || prepares(c, after, classKey) {
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

// prepares says whether the class of b prepares spells (for a third caster the
// offer says it only once the subclass is chosen).
func prepares(c *Content, b Build, classKey string) bool {
	sc := spellcastingOf(Derive(b, c), classKey)
	return sc != nil && sc.PreparesSpells
}

// TestLevelUpSweepMulticlass takes every ordered pair of classes of the SRD (12 by
// 11 = 132): a character of the first class at total level 5 enters the second at
// level 1 and then levels it to total level 20, every step passing the same checks
// as TestLevelUpSweep. It runs twice: with scores that meet every prerequisite
// (all 132 pairs are open), and with the scores of the sweep's base character,
// where the pairs whose prerequisites the scores do not meet must be refused, with
// the class of the refusal said right.
func TestLevelUpSweepMulticlass(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	for _, scores := range []string{"every prerequisite met", "the sweep's scores"} {
		for _, a := range sortedKeys(c.c.classes) {
			for _, b := range sortedKeys(c.c.classes) {
				if a == b {
					continue
				}
				t.Run(scores+"/"+a+"+"+b, func(t *testing.T) {
					t.Parallel()
					sweepMulticlassPair(t, c, a, b, scores == "every prerequisite met")
				})
			}
		}
	}
}

// TestLevelUpSweepMulticlassWithTableClasses is the same sweep for the table's
// classes of every casting kind (none, full, half and pact casters), as the first
// class and as the second, together with every class of the SRD.
func TestLevelUpSweepMulticlassWithTableClasses(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)
	c := withOverlayOn(t, srd, genOverlay(t, srd))
	var table []string
	for _, key := range sortedKeys(c.c.classes) {
		if isTableKey(key) {
			table = append(table, key)
		}
	}
	if len(table) != len(genKinds) {
		t.Fatalf("table classes = %v, want one of each casting kind", table)
	}
	for _, g := range table {
		for _, s := range sortedKeys(srd.c.classes) {
			if g == "class:gen-pact-known@mesa" && s == "class:warlock" {
				// Two classes with Pact Magic: the SRD has one, so it has no rule for
				// the slots of a second (Derive keeps the last class's).
				continue
			}
			for _, pair := range [][2]string{{g, s}, {s, g}} {
				t.Run(pair[0]+"+"+pair[1], func(t *testing.T) {
					t.Parallel()
					sweepMulticlassPair(t, c, pair[0], pair[1], true)
				})
			}
		}
	}
}

// sweepMulticlassPair is a character of class a at total level 5 entering class b
// at level 1 and leveling it to 20 in all. With strong every score is 15, so the
// prerequisites are met; otherwise the sweep's base scores are used, and a pair
// they do not qualify is checked to be refused for the right class.
func sweepMulticlassPair(t *testing.T, c *Content, a, b string, strong bool) {
	t.Helper()
	subOf := func(key string) string {
		if subs := c.c.classes[key].Subclasses; len(subs) > 0 {
			return subs[0]
		}
		return ""
	}
	base := sweepBase(t, c, a, subOf(a))
	if strong {
		base.BaseScores = map[Ability]int{STR: 14, DEX: 14, CON: 14, INT: 14, WIS: 14, CHA: 14}
	}
	base = sweepUp(t, c, base, a, subOf(a), 5)

	final := map[Ability]int{}
	for _, s := range Derive(base, c).Abilities {
		final[s.Ability] = s.Score
	}
	// The prerequisites by the SRD's table (the table's own classes ask for
	// Constitution 11), on the final scores.
	met := func(class string) bool {
		if isTableKey(class) {
			return final[CON] >= 11
		}
		for _, row := range prerequisiteTable {
			if row.class != class {
				continue
			}
			n := 0
			for _, ab := range row.abilities {
				if final[ab] >= 13 {
					n++
				}
			}
			return n == len(row.abilities) || row.anyOf && n > 0
		}
		t.Fatalf("no prerequisite row for %s", class)
		return false
	}
	if strong && (!met(a) || !met(b)) {
		t.Fatalf("the scores %v do not meet %s and %s", final, a, b)
	}
	_, err := LevelUpOptions(base, b, c)
	switch {
	case !met(a):
		wantRefusal(t, err, LevelUpReasonMulticlassPrerequisiteCurrent, "class_key")
		return
	case !met(b):
		wantRefusal(t, err, LevelUpReasonMulticlassPrerequisite, "class_key")
		return
	case err != nil:
		t.Fatalf("LevelUpOptions: %v", err)
	}

	// Enters the class, then levels it to 20 in all.
	ch := satisfy(t, c, base, b, subOf(b))
	after, err := ApplyLevelUp(base, ch, c)
	if err != nil {
		t.Fatalf("ApplyLevelUp: %v", err)
	}
	if err := CheckLevelUp(base, after, c); err != nil {
		t.Fatalf("CheckLevelUp: %v\nchoices: %+v", err, ch)
	}
	if err := Validate(after, c); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if issues := Derive(after, c).Issues; len(issues) != 0 {
		t.Fatalf("the new sheet has issues: %v\nchoices: %+v", issues, ch)
	}
	if d := Derive(after, c); d.TotalLevel != 6 || len(d.Classes) != 2 || d.Classes[1].ClassKey != b || d.Classes[1].Level != 1 {
		t.Fatalf("classes after = %+v, total %d", d.Classes, d.TotalLevel)
	}
	sweepUp(t, c, after, b, subOf(b), MaxLevel)
}
