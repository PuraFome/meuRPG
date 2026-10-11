package rules

import "testing"

// A raw ability check includes no proficiency, so a bard's Jack of All Trades adds half the
// proficiency bonus, rounded down, to it (SRD 5.1, Bard).
func TestJackOfAllTradesAddsToARawAbilityCheck(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	bonus := func(class string, level int) int {
		t.Helper()
		opts, err := SceneOptions(Derive(standard(class, level), c), []SceneAction{{Key: "ability:str"}})
		if err != nil {
			t.Fatalf("SceneOptions() error = %v", err)
		}
		return opts[0].Bonus
	}
	// Strength 15 and the human's +1 is 16 (+3), at proficiency +2: the bard adds +1; a
	// fighter adds nothing.
	if got := bonus("class:bard", 3); got != 4 {
		t.Errorf("a bard 3's Strength check = %+d, want +4 (+3 and half of the +2 proficiency)", got)
	}
	if got := bonus("class:fighter", 3); got != 3 {
		t.Errorf("a fighter 3's Strength check = %+d, want +3", got)
	}
	// Proficiency +5 at level 13: half rounds down to +2.
	if got := bonus("class:bard", 13); got != 5 {
		t.Errorf("a bard 13's Strength check = %+d, want +5 (+3 and half of +5, rounded down)", got)
	}
}
