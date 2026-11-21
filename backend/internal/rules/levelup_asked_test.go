package rules

import "testing"

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
