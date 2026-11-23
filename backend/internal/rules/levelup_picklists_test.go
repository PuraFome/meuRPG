package rules

import (
	"maps"
	"slices"
	"strconv"
	"testing"
)

// The rows of the SRD 5.1 that name the skills and the expertise a level lets the player pick, read in the offer of the level
// (LevelUpOffer / LevelUpSubclass SkillsAllowed and ExpertiseAllowed). The rules are paraphrased in the comments.

// proficientIn are the skills the sheet is proficient in (expertise included); lacking are the ones it is not.
func proficientIn(c *Content, b Build) (have, lacking []string) {
	for _, s := range Derive(b, c).Skills {
		if s.Proficiency >= ProficiencyFull {
			have = append(have, s.Key)
		} else {
			lacking = append(lacking, s.Key)
		}
	}
	return have, lacking
}

// withoutExpertise are the keys without the ones the sheet has expertise in.
func withoutExpertise(keys []string, b Build) []string {
	return slices.DeleteFunc(slices.Clone(keys), func(k string) bool { return slices.Contains(b.Expertise, k) })
}

func sameKeys(t *testing.T, what string, got, want []string) {
	t.Helper()
	g, w := slices.Sorted(slices.Values(got)), slices.Sorted(slices.Values(want))
	if !slices.Equal(g, w) {
		t.Errorf("%s = %v, want %v", what, g, w)
	}
}

// A character enters a new class and the class's multiclassing row says what skill it gives: the bard one skill of any, the
// ranger and the rogue one from the class's own list, the other classes none.
func TestTheMulticlassPickOfANewClassOffersTheSkillOfItsRow(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	f := doranFighter5() // fighter 5: athletics and perception, the acolyte's insight and religion
	f.BaseScores = map[Ability]int{STR: 15, DEX: 14, CON: 13, INT: 13, WIS: 13, CHA: 13}
	_, lacking := proficientIn(c, f)
	for _, tc := range []struct {
		class  string
		from   []string // the skills of the row; nil for any skill
		asked  bool
		absent []string // skills no list of the sheet holds
	}{
		{"class:bard", nil, true, nil},
		{"class:ranger", []string{"skill:animal-handling", "skill:athletics", "skill:insight", "skill:investigation", "skill:nature", "skill:perception", "skill:stealth", "skill:survival"}, true, []string{"skill:arcana", "skill:deception", "skill:persuasion"}},
		{"class:rogue", []string{"skill:acrobatics", "skill:athletics", "skill:deception", "skill:insight", "skill:intimidation", "skill:investigation", "skill:perception", "skill:performance", "skill:persuasion", "skill:sleight-of-hand", "skill:stealth"}, true, []string{"skill:arcana", "skill:medicine", "skill:nature"}},
		{"class:wizard", nil, false, nil},
		{"class:barbarian", nil, false, nil},
		{"class:paladin", nil, false, nil},
		{"class:sorcerer", nil, false, nil},
	} {
		o, err := LevelUpOptions(f, tc.class, c)
		if err != nil {
			t.Errorf("%s: %v", tc.class, err)
			continue
		}
		if !tc.asked {
			if o.Multiclass != nil && o.Multiclass.Skills != nil || len(o.SkillsAllowed) != 0 || o.SkillChoices != 0 {
				t.Errorf("%s: a skill is offered: pick %+v, allowed %v", tc.class, o.Multiclass, o.SkillsAllowed)
			}
			continue
		}
		if o.Multiclass == nil || o.Multiclass.Skills == nil || o.Multiclass.Skills.Choose != 1 {
			t.Errorf("%s: multiclass pick = %+v, want one skill", tc.class, o.Multiclass)
			continue
		}
		var offered []string
		for _, opt := range o.Multiclass.Skills.From {
			if !opt.AlreadyHave {
				offered = append(offered, opt.Key)
			}
		}
		want := lacking
		if tc.from != nil {
			want = slices.DeleteFunc(slices.Clone(tc.from), func(k string) bool { return !slices.Contains(lacking, k) })
		}
		sameKeys(t, tc.class+" row", offered, want)
		for _, k := range want {
			if !slices.Contains(o.SkillsAllowed, k) {
				t.Errorf("%s: skills_allowed lacks %s of the row", tc.class, k)
			}
		}
		for _, k := range tc.absent {
			if slices.Contains(o.SkillsAllowed, k) {
				t.Errorf("%s: skills_allowed holds %s, which no list of the sheet gives", tc.class, k)
			}
		}
		for _, k := range []string{"skill:insight", "skill:religion"} {
			if slices.Contains(o.SkillsAllowed, k) {
				t.Errorf("%s: skills_allowed holds %s, which the background gives already", tc.class, k)
			}
		}
	}
}

// The bard college gives three skills of the player's choice at level 3: every skill the sheet lacks is offered, and the
// college is the part that carries them. A bard has expertise to place at levels 3 and 10 on skills the sheet is proficient in.
func TestTheBardCollegeOffersEverySkillTheSheetLacks(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	b2 := sweepBase(t, c, "class:bard", "subclass:lore")
	b2 = mustApply(t, c, b2, satisfy(t, c, b2, "class:bard", "subclass:lore"))
	o, err := LevelUpOptions(b2, "class:bard", c)
	if err != nil || !o.SubclassDue || len(o.Subclasses) != 1 {
		t.Fatalf("bard 2 to 3: %+v, %v", o, err)
	}
	college := o.Subclasses[0]
	have, lacking := proficientIn(c, b2)
	if college.SkillChoices != 3 || len(o.SkillsAllowed) != 0 {
		t.Errorf("skills: college %d, class %d allowed; want the college's 3 and none for the class", college.SkillChoices, len(o.SkillsAllowed))
	}
	sameKeys(t, "college skills_allowed", college.SkillsAllowed, lacking)
	// Expertise of level 3 (two skills the sheet is proficient in, or the ones the college gives at the same level).
	if o.ExpertiseChoices != 2 {
		t.Fatalf("expertise at bard 3 = %d, want 2", o.ExpertiseChoices)
	}
	sameKeys(t, "bard 3 expertise_allowed of the class alone", o.ExpertiseAllowed, withoutExpertise(have, b2))
	sameKeys(t, "bard 3 expertise_allowed with the college", college.ExpertiseAllowed, withoutExpertise(slices.Concat(have, lacking), b2))

	// Level 10: the sheet has two expertise; two more, on skills it has the proficiency in.
	b := b2
	for range 9 {
		o, err = LevelUpOptions(b, "class:bard", c)
		if err != nil {
			t.Fatal(err)
		}
		if o.ToLevel == 10 {
			break
		}
		sub := ""
		if o.SubclassDue {
			sub = "subclass:lore"
		}
		b = mustApply(t, c, b, satisfy(t, c, b, "class:bard", sub))
	}
	if o.ToLevel != 10 || o.ExpertiseChoices != 2 || len(b.Expertise) != 2 || o.SkillChoices != 0 || len(o.SkillsAllowed) != 0 {
		t.Fatalf("bard 9 to 10: level %d, expertise %d, held %v, skills %d", o.ToLevel, o.ExpertiseChoices, b.Expertise, o.SkillChoices)
	}
	have, _ = proficientIn(c, b)
	sameKeys(t, "bard 10 expertise_allowed", o.ExpertiseAllowed, withoutExpertise(have, b))
	if slices.Contains(o.ExpertiseAllowed, "skill:arcana") && !slices.Contains(have, "skill:arcana") {
		t.Error("expertise offered on a skill the sheet is not proficient in")
	}
}

// A rogue picks expertise at levels 1 and 6, on skills it is proficient in. The offer of a new rogue level 1 adds the skill
// the multiclassing row gives; level 6 offers the proficient skills without expertise and none else.
func TestTheRogueExpertiseIsOfferedAmongTheSkillsItIsProficientIn(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	f := doranFighter5()
	o, err := LevelUpOptions(f, "class:rogue", c)
	if err != nil || o.ExpertiseChoices != 2 {
		t.Fatalf("rogue 1 by multiclassing: expertise %d, %v", o.ExpertiseChoices, err)
	}
	have, _ := proficientIn(c, f)
	want := slices.Concat(have, o.SkillsAllowed)
	for _, k := range o.ExpertiseAllowed {
		if !slices.Contains(want, k) {
			t.Errorf("rogue 1: expertise_allowed holds %s: not proficient, and no pick of the level gives it", k)
		}
	}
	for _, k := range have {
		if !slices.Contains(o.ExpertiseAllowed, k) {
			t.Errorf("rogue 1: expertise_allowed lacks %s, a proficiency the sheet has", k)
		}
	}
	if slices.Contains(o.ExpertiseAllowed, "skill:arcana") {
		t.Error("rogue 1: arcana is no proficiency of the sheet and no list gives it")
	}

	b := sweepBase(t, c, "class:rogue", "")
	for range 4 {
		sub := ""
		if o, _ := LevelUpOptions(b, "class:rogue", c); o.SubclassDue {
			sub = o.Subclasses[0].Key
		}
		b = mustApply(t, c, b, satisfy(t, c, b, "class:rogue", sub))
	}
	o, err = LevelUpOptions(b, "class:rogue", c)
	if err != nil || o.ToLevel != 6 || o.ExpertiseChoices != 2 || o.SkillChoices != 0 || len(b.Expertise) != 2 {
		t.Fatalf("rogue 5 to 6: %+v, %v", o, err)
	}
	have, _ = proficientIn(c, b)
	sameKeys(t, "rogue 6 expertise_allowed", o.ExpertiseAllowed, withoutExpertise(have, b))
	for _, k := range b.Expertise {
		if slices.Contains(o.ExpertiseAllowed, k) {
			t.Errorf("rogue 6: expertise_allowed holds %s, which has expertise", k)
		}
	}
	// A new expertise on a skill without the proficiency is a sheet issue: the level refuses it.
	ch := satisfy(t, c, b, "class:rogue", "")
	ch.Expertise = []string{"skill:arcana", have[0]}
	if err := CheckLevelUp(b, mustApply(t, c, b, ch), c); err == nil {
		t.Error("expertise on a skill without the proficiency is accepted")
	}
}

// A proficiency the sheet would gain twice lets the player choose another skill instead: a list whose skills the sheet has
// by another means (the acolyte's insight) offers the other skills for the pick.
func TestASkillTheSheetHasAlreadyOffersAnotherInItsPlace(t *testing.T) {
	t.Parallel()
	traits := []TableFeature{capTrait("trait:faro@mesa", "skill", 1, "skill:insight", "skill:religion")} // both held by the acolyte
	c, b := capSheet(t, traits, nil, nil)
	skills, _ := c.LevelPickKeys(b)
	_, lacking := proficientIn(c, b)
	// The fighter's own two picks are open on this sheet too: the trait's pick can go on any skill the sheet lacks.
	for _, k := range lacking {
		if k != "skill:insight" && !slices.Contains(skills, k) {
			t.Errorf("skills_allowed lacks %s, a skill that stands in for the duplicate", k)
		}
	}
	for _, k := range []string{"skill:insight", "skill:religion"} {
		if slices.Contains(skills, k) {
			t.Errorf("skills_allowed holds %s, which the sheet has already", k)
		}
	}
	if len(skills) < 12 {
		t.Errorf("skills_allowed = %v, want every other skill", skills)
	}
}

// A level that gives no pick offers none: no skill and no expertise to choose.
func TestALevelWithoutAPickOffersNoSkillAndNoExpertise(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	cases := map[string]struct {
		b     Build
		class string
	}{
		"fighter 5 to 6": {doranFighter5(), "class:fighter"},
		"cleric 3 to 4":  {clericBuild(3), "class:cleric"},
	}
	cleric := cases["cleric 3 to 4"]
	cleric.b.SkillProficiencies = []string{"skill:medicine", "skill:persuasion"}
	cases["cleric 3 to 4"] = cleric
	for name, tc := range cases {
		o, err := LevelUpOptions(tc.b, tc.class, c)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if o.SkillChoices != 0 || o.ExpertiseChoices != 0 || len(o.SkillsAllowed) != 0 || len(o.ExpertiseAllowed) != 0 {
			t.Errorf("%s: skills %d/%v, expertise %d/%v; want none", name, o.SkillChoices, o.SkillsAllowed, o.ExpertiseChoices, o.ExpertiseAllowed)
		}
	}
	for _, class := range []string{"class:bard", "class:rogue"} {
		b := sweepBase(t, c, class, "")
		b = mustApply(t, c, b, satisfy(t, c, b, class, "")) // level 2: no pick
		if o, err := LevelUpOptions(b, class, c); err != nil || o.ToLevel != 3 {
			t.Errorf("%s: %v", class, err)
		}
	}
}

// fighterForEveryClass is Doran with the scores every class's multiclass prerequisite asks for.
func fighterForEveryClass() Build {
	f := doranFighter5()
	f.BaseScores = map[Ability]int{STR: 15, DEX: 14, CON: 13, INT: 13, WIS: 13, CHA: 13}
	return f
}

// Cases of the sweep below: the sheet before the level and the class that gains it.
type pickLevel struct {
	name   string
	before Build
	class  string
}

// levelPickCases are the level-ups the sweep reads: every class of the SRD taken as a later class by a Fighter 5, and every
// level of every class at its own levels.
func levelPickCases(t *testing.T, c *Content) []pickLevel {
	t.Helper()
	var out []pickLevel
	for _, class := range slices.Sorted(maps.Keys(c.c.classes)) {
		out = append(out, pickLevel{"fighter 5 takes " + class, fighterForEveryClass(), class})
		b := sweepBase(t, c, class, subclassKeyAtOne(c, class))
		for lvl := 2; lvl <= 20; lvl++ {
			sub := ""
			if o, err := LevelUpOptions(b, class, c); err != nil {
				break
			} else if o.SubclassDue {
				sub = o.Subclasses[0].Key
			}
			out = append(out, pickLevel{class + " to " + strconv.Itoa(lvl), b, class})
			b = mustApply(t, c, b, satisfy(t, c, b, class, sub))
		}
	}
	return out
}

// subclassKeyAtOne is the first subclass of a class that picks it at level 1, "" for the others.
func subclassKeyAtOne(c *Content, class string) string {
	if cl := c.c.classes[class]; cl.SubclassLevel == 1 && len(cl.Subclasses) > 0 {
		return cl.Subclasses[0]
	}
	return ""
}

// askedOf is what the level asks in all, with the first subclass's part when one is due: the skills (the multiclass pick
// included), the expertise, and the lists the offer sends.
func askedOf(o LevelUpOffer) (skills, expertise int, allowed, expertiseAllowed []string, sub string) {
	skills, expertise, allowed, expertiseAllowed = o.SkillChoices, o.ExpertiseChoices, o.SkillsAllowed, o.ExpertiseAllowed
	if o.Multiclass != nil && o.Multiclass.Skills != nil {
		skills += o.Multiclass.Skills.Choose
	}
	if o.SubclassDue {
		s := o.Subclasses[0]
		sub = s.Key
		skills, expertise = skills+s.SkillChoices, expertise+s.ExpertiseChoices
		allowed, expertiseAllowed = slices.Concat(allowed, s.SkillsAllowed), slices.Concat(expertiseAllowed, s.ExpertiseAllowed)
	}
	return skills, expertise, allowed, expertiseAllowed, sub
}

// What the server sends as allowed is what the write accepts. At a level that asks one skill, the level-up with one new
// skill is accepted exactly when the skill is in skills_allowed; every expertise the write accepts is in expertise_allowed.
// A class added by multiclassing gives one skill from its own row (SRD 5.1, "Multiclassing", "Proficiencies"): the picks made
// at earlier levels stay where they are and do not move to it.
func TestTheAllowedSkillsAreTheSkillsTheLevelUpAccepts(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	for _, tc := range levelPickCases(t, c) {
		o, err := LevelUpOptions(tc.before, tc.class, c)
		if err != nil {
			t.Errorf("%s: %v", tc.name, err)
			continue
		}
		skills, expertise, allowed, expertiseAllowed, sub := askedOf(o)
		if skills != 1 && expertise == 0 {
			continue
		}
		base := satisfy(t, c, tc.before, tc.class, sub)
		for _, k := range slices.Sorted(maps.Keys(c.c.skills)) {
			if skills == 1 {
				ch := base
				ch.SkillProficiencies = []string{k}
				after := mustApply(t, c, tc.before, ch)
				accepted := CheckLevelUp(tc.before, after, c) == nil
				if accepted != slices.Contains(allowed, k) {
					t.Errorf("%s: new skill %s accepted = %v, in skills_allowed = %v", tc.name, k, accepted, slices.Contains(allowed, k))
				}
			}
			if expertise > 0 {
				ch := base
				ch.Expertise = slices.Concat([]string{k}, base.Expertise[1:])
				after := mustApply(t, c, tc.before, ch)
				if CheckLevelUp(tc.before, after, c) == nil && !slices.Contains(expertiseAllowed, k) {
					t.Errorf("%s: expertise %s accepted, not in expertise_allowed", tc.name, k)
				}
			}
		}
	}
}

// For the ranger and the rogue, the skills only the fighter's list holds are not offered to a Fighter 5 who adds the class:
// the pick comes from the new class's row.
func TestAClassAddedByMulticlassingNamesNoSkillOfAnEarlierSource(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	for class, absent := range map[string][]string{
		"class:ranger": {"skill:history", "skill:acrobatics", "skill:intimidation"},
		"class:rogue":  {"skill:history", "skill:animal-handling", "skill:survival"},
	} {
		o, err := LevelUpOptions(fighterForEveryClass(), class, c)
		if err != nil {
			t.Fatal(err)
		}
		for _, k := range absent {
			if slices.Contains(o.SkillsAllowed, k) {
				t.Errorf("%s: skills_allowed holds %s, which only the fighter's list gives", class, k)
			}
		}
	}
}
