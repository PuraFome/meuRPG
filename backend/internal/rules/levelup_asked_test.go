package rules

import (
	"slices"
	"testing"
)

// A sheet that holds more expertise than its sources allow does not get them asked again: the level asks only what its
// allowance adds beyond the expertise held.
func TestALevelAsksNoExpertiseTheSheetHoldsAlready(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	b := sweepBase(t, c, "class:rogue", "")
	for range 4 {
		sub := ""
		if o, _ := LevelUpOptions(b, "class:rogue", c); o.SubclassDue {
			sub = o.Subclasses[0].Key
		}
		b = mustApply(t, c, b, satisfy(t, c, b, "class:rogue", sub))
	}
	have, _ := proficientIn(c, b)
	b.Expertise = withoutExpertise(have, Build{})[:3] // one more than the allowance of rogue 5
	o, err := LevelUpOptions(b, "class:rogue", c)
	if err != nil || o.ToLevel != 6 {
		t.Fatalf("rogue 5 to 6: %+v, %v", o, err)
	}
	if o.ExpertiseChoices != 1 {
		t.Errorf("expertise asked = %d, want 1: rogue 6 allows 4 and the sheet holds 3", o.ExpertiseChoices)
	}
}

// A sheet that holds more skill picks than its sources allow (a third pick the master wrote) can always complete the level
// the app offers: the number of new skills the write demands is the number the offer asks, at a level of its own class and at
// the level that adds a class.
func TestASheetHoldingMoreSkillPicksThanItsSourcesAllowCompletesTheLevelOffered(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	f := fighterForEveryClass()
	f.SkillProficiencies = append(f.SkillProficiencies, "skill:history")
	for _, class := range []string{"class:bard", "class:fighter"} {
		o, err := LevelUpOptions(f, class, c)
		if err != nil {
			t.Fatalf("%s: %v", class, err)
		}
		skills, _, allowed, _, sub := askedOf(o)
		ch := satisfy(t, c, f, class, sub)
		if len(ch.SkillProficiencies) != skills {
			t.Errorf("%s: the offer asks %d skills, the request has %d", class, skills, len(ch.SkillProficiencies))
		}
		for _, k := range ch.SkillProficiencies {
			if !slices.Contains(allowed, k) {
				t.Errorf("%s: %s is not in skills_allowed %v", class, k, allowed)
			}
		}
		if err := CheckLevelUp(f, mustApply(t, c, f, ch), c); err != nil {
			t.Errorf("%s: the level the offer describes is refused: %v", class, err)
		}
	}
}
